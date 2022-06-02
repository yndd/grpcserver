package grpcserver

import (
	"context"

	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

// Check implements `service Health`.
func (s *GrpcServer) Check(ctx context.Context, in *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()
	err := s.acquireSem(ctx)
	if err != nil {
		return nil, err
	}
	defer s.sem.Release(1)

	if s.checkHandler != nil {
		return s.checkHandler(ctx, in)
	}

	return &healthpb.HealthCheckResponse{}, nil
}

// Watch implements `service Health`.
func (s *GrpcServer) Watch(in *healthpb.HealthCheckRequest, stream healthpb.Health_WatchServer) error {
	err := s.acquireSem(stream.Context())
	if err != nil {
		return err
	}
	defer s.sem.Release(1)

	if s.watchHandler != nil {
		return s.watchHandler(in, stream)
	}
	return status.Error(codes.Unimplemented, "")
}
