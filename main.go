package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/go-kit/kit/circuitbreaker"
	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/loadbalancer"
	"github.com/go-kit/kit/loadbalancer/consul"
	"github.com/go-kit/kit/log"
	kitratelimit "github.com/go-kit/kit/ratelimit"
	grpctransport "github.com/go-kit/kit/transport/grpc"
	"github.com/gorilla/mux"
	"github.com/hashicorp/consul/api"
	jujuratelimit "github.com/juju/ratelimit"
	"github.com/majest/go-test-service/pb"
	"github.com/sony/gobreaker"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var sc serviceClient

var consulIP string
var consulPort int

var grpcr *pb.CountReply

// very simple struct to hold context and endpoint so that it's available at http level
type serviceClient struct {
	context.Context
	Endpoint endpoint.Endpoint
}

func init() {
	flag.StringVar(&consulIP, "consulip", "192.168.99.101", "Consul node ip")
	flag.IntVar(&consulPort, "consulport", 8500, "Consul node port")
	flag.Parse()
}

type proxymw struct {
	context.Context
	Endpoint endpoint.Endpoint
}

var p proxymw
var logger log.Logger

func main() {
	go http.ListenAndServe(":36660", nil)
	ctx := context.Background()
	logger = log.NewLogfmtLogger(os.Stdout)

	config := api.DefaultConfig()

	// consul address and port
	config.Address = fmt.Sprintf("%s:%v", consulIP, consulPort)

	c, errc := api.NewClient(config)

	if errc != nil {
		panic(errc.Error())
	}

	// consul client
	client := consul.NewClient(c)

	var (
		qps            = 100 // max to each instance
		publisher, err = consul.NewPublisher(client, factory(ctx, qps), logger, "com.service.string")
		lb             = loadbalancer.NewRoundRobin(publisher)
		maxAttempts    = 3
		maxTime        = 100 * time.Millisecond
		endpoint       = loadbalancer.Retry(maxAttempts, maxTime, lb)
	)

	if err != nil {
		panic(err.Error())
	}

	p = proxymw{ctx, endpoint}

	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/test/{data}", GetCount)
	logger.Log(http.ListenAndServe(":8090", router))
}

func GetCount(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	data := vars["data"]
	response, err := p.Call(data)
	fmt.Fprintln(w, "Data:", response)
	logger.Log("error", err)

}

func (mw proxymw) Call(name string) (int, error) {
	r, err := mw.Endpoint(mw.Context, pb.CountRequest{A: name})

	if err != nil {
		return -1, err
	}
	a := r.(*pb.CountReply)

	return int(a.V), nil
}

func factory(ctx context.Context, qps int) loadbalancer.Factory {
	return func(instance string) (endpoint.Endpoint, io.Closer, error) {
		var e endpoint.Endpoint
		e = makeProxy(ctx, instance)
		e = circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{}))(e)
		e = kitratelimit.NewTokenBucketLimiter(jujuratelimit.NewBucketWithRate(float64(qps), int64(qps)))(e)
		return e, nil, nil
	}
}

func makeProxy(ctx context.Context, instance string) endpoint.Endpoint {
	fmt.Println()
	conn, err := grpc.Dial(instance, grpc.WithInsecure())

	if err != nil {
		fmt.Println(err.Error())
	}

	return grpctransport.NewClient(
		conn,
		"Strings",
		"Count",
		EncodeCountRequest,
		DecodeCountResponse,
		&pb.CountReply{}, // why?
	).Endpoint()
}

func EncodeCountRequest(ctx context.Context, request interface{}) (interface{}, error) {
	req := request.(pb.CountRequest)
	return &req, nil
}

func DecodeCountResponse(ctx context.Context, response interface{}) (interface{}, error) {
	return response, nil
}
