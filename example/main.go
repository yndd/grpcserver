package main

import (
	"context"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	grpcserver "github.com/yndd/grpc-server"
)

func main() {
	c := grpcserver.Config{
		Insecure: true,
		GNMI:     true,
	}
	s := grpcserver.New(c,
		grpcserver.WithGetHandler("openconfig",
			func(ctx context.Context, req *gnmi.GetRequest) (*gnmi.GetResponse, error) {
				return &gnmi.GetResponse{
					Notification: []*gnmi.Notification{
						{
							Timestamp: time.Now().UnixNano(),
							Prefix:    req.GetPrefix(),
							Update: []*gnmi.Update{
								{Path: req.GetPath()[0]},
							},
						},
					},
				}, nil
			}),
		grpcserver.WithGetHandler("",
			func(ctx context.Context, req *gnmi.GetRequest) (*gnmi.GetResponse, error) {
				return &gnmi.GetResponse{
					Notification: []*gnmi.Notification{
						{
							Timestamp: time.Now().UnixNano(),
							Prefix:    req.GetPrefix(),
							Update: []*gnmi.Update{
								{Path: req.GetPath()[0]},
							},
						},
					},
				}, nil
			}),
	)

	err := s.Start(context.Background())
	if err != nil {
		panic(err)
	}
}
