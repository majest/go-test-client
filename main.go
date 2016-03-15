package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/benschw/srv-lb/dns"
	"github.com/benschw/srv-lb/lb"
	"github.com/benschw/srv-lb/strategy/random"
	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
	"github.com/majest/go-test-service/pb"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var client pb.StringsClient
var ctx context.Context

var consulIP string
var consulPort int

func init() {
	flag.StringVar(&consulIP, "consulip", "192.168.99.101", "Consul node ip")
	flag.IntVar(&consulPort, "consulport", 8600, "Consul node port")
	flag.Parse()
}

func main() {
	go http.ListenAndServe(":36660", nil)
	logger := log.NewLogfmtLogger(os.Stdout)

	ctx = context.Background()

	conn, err := grpc.Dial("localhost:9090", grpc.WithInsecure())
	if err != nil {
		logger.Log(err.Error())
	}
	client = pb.NewStringsClient(conn)

	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/count/{data}", count)
	logger.Log(http.ListenAndServe(":9091", router))

}

func count(w http.ResponseWriter, r *http.Request) {
	res, _ := client.Count(ctx, &pb.CountRequest{A: mux.Vars(r)["data"]})
	instance := getInstance("string.service.consul")
	fmt.Printf("instance %s\n", instance)
	fmt.Fprintln(w, "Data:", res)
}

//
func getInstance(serviceName string) string {

	cfg := &lb.Config{
		Dns:      dns.NewLookupLib(fmt.Sprintf("%s:%v", consulIP, consulPort)),
		Strategy: random.RandomStrategy,
	}

	l := lb.New(cfg, serviceName)

	address, err := l.Next()
	if err != nil {
		fmt.Printf("%s", err.Error())
	}

	return address.String()
}
