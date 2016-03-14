package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
	"github.com/majest/go-test-service/pb"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var client pb.StringsClient
var ctx context.Context

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
	fmt.Fprintln(w, "Data:", res)
}
