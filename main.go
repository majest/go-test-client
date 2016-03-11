package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
	"github.com/majest/go-service-test/server"
	"github.com/majest/go-strings-client/client"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var logger log.Logger
var root context.Context

func main() {
	logger = log.NewLogfmtLogger(os.Stdout)
	logger = log.NewContext(logger).With("caller", log.DefaultCaller)
	logger = log.NewContext(logger).With("transport", "grpc")

	root = context.Background()
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/count/{data}", Count)
	logger.Log(http.ListenAndServe(":9010", router))
}

func trace(s string) (string, time.Time) {
	fmt.Println("START:", s)
	return s, time.Now()
}

func un(s string, startTime time.Time) {
	endTime := time.Now()
	fmt.Println("  END:", s, "ElapsedTime in seconds:", endTime.Sub(startTime))
}

func Count(w http.ResponseWriter, r *http.Request) {
	defer un(trace("count"))
	cc, err := grpc.Dial("localhost:9090", grpc.WithInsecure())
	if err != nil {
		logger.Log("err", err)
		os.Exit(1)
	}
	defer cc.Close()

	vars := mux.Vars(r)
	data := vars["data"]

	var svc server.StringService

	svc = client.New(root, cc, logger)
	v := svc.Count(data)
	fmt.Fprintln(w, "Data:", v)
}
