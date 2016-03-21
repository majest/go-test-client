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
	"github.com/gorilla/mux"
	jujuratelimit "github.com/juju/ratelimit"
	clb "github.com/majest/go-microservice/consul"
	stringssvc "github.com/majest/go-test-client/string"
	"github.com/majest/go-test-service/pb"
	"github.com/majest/go-test-service/server"
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

	discoveryClient := consul.NewClient(
		clb.New(
			&clb.Config{
				NodeIp:   consulIP,
				NodePort: consulPort,
			},
		).Client)

	var (
		qps            = 100 // max to each instance
		publisher, err = consul.NewPublisher(discoveryClient, factory(ctx, qps), logger, "string")
		lb             = loadbalancer.NewRoundRobin(publisher)
		maxAttempts    = 3
		maxTime        = 100 * time.Millisecond
		e              = loadbalancer.Retry(maxAttempts, maxTime, lb)
	)

	if err != nil {
		panic(err.Error())
	}

	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/test/{data}", makeHandler(ctx, e, logger))
	logger.Log(http.ListenAndServe(":8090", router))
}

func makeHandler(ctx context.Context, e endpoint.Endpoint, logger log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := mux.Vars(r)["data"]
		resp, _ := e(ctx, data)
		fmt.Fprintln(w, fmt.Sprintf("Data:%d", resp.(int)))
	}
}

func factory(ctx context.Context, qps int) loadbalancer.Factory {
	return func(instance string) (endpoint.Endpoint, io.Closer, error) {

		// loadbalancer factory should call grpc
		var e endpoint.Endpoint
		conn, err := grpc.Dial(instance, grpc.WithInsecure())
		if err != nil {
			return e, nil, err
		}

		// create a service
		svc := stringssvc.New(ctx, conn, logger)

		// make an endpoint out of serice, service should know about connextion
		e = makeEndpoint(svc)

		// circut breaker
		e = circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{}))(e)

		// rate limiter
		e = kitratelimit.NewTokenBucketLimiter(jujuratelimit.NewBucketWithRate(float64(qps), int64(qps)))(e)
		return e, nil, nil
	}
}

func makeEndpoint(svc server.StringsService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(string)
		resp := svc.Count(req)
		return resp, nil
	}
}
