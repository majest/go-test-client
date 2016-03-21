package string

// todo: fix paclage naming

import (
	"github.com/go-kit/kit/log"
	"github.com/majest/go-test-service/pb"
	"github.com/majest/go-test-service/server"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type client struct {
	context.Context
	pb.StringsClient
	log.Logger
}

func New(ctx context.Context, cc *grpc.ClientConn, logger log.Logger) server.StringsService {
	return client{ctx, pb.NewStringsClient(cc), logger}
}

func (c client) Count(a string) int {
	reply, err := c.StringsClient.Count(c.Context, &pb.CountRequest{a})
	if err != nil {
		c.Logger.Log("err", err) // Without an error return parameter, we can't do anything else...
		return 0
	}
	return int(reply.V)
}
