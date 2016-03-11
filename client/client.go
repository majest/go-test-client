package client

import (
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/go-kit/kit/log"
	"github.com/majest/go-service-test/pb"
	"github.com/majest/go-service-test/server"
)

// New returns an AddService that's backed by the provided ClientConn.
func New(ctx context.Context, cc *grpc.ClientConn, logger log.Logger) server.StringService {
	return Client{ctx, pb.NewStringsClient(cc), logger}
}

type Client struct {
	context.Context
	pb.StringsClient
	log.Logger
}

func (c Client) Count(v string) int {
	request := &pb.CountRequest{
		A: v,
	}
	reply, err := c.StringsClient.Count(c.Context, request)
	if err != nil {
		c.Logger.Log("err", err) // Without an error return parameter, we can't do anything else...
		return 0
	}
	return int(reply.V)
}
