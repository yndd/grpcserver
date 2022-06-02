package grpcserver

import (
	"context"
	"crypto/tls"
	"net"
	"sync"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/pkg/errors"
	"github.com/yndd/ndd-runtime/pkg/logging"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type GrpcServer struct {
	config Config
	gnmi.UnimplementedGNMIServer

	sem *semaphore.Weighted

	m *sync.RWMutex
	// gnmi handlers
	getHandlers        map[string]GetHandler
	setUpdateHandlers  map[string]SetUpdateHandler
	setReplaceHandlers map[string]SetReplaceHandler
	setDeleteHandlers  map[string]SetDeleteHandler

	// k8s client,
	// used to retrieve certificates stored in secrets
	client client.Client
	// logger
	logger logging.Logger
	//
	//health handlers
	checkHandler CheckHandler
	watchHandler WatchHandler
	//
	// cached certificate
	cm   *sync.Mutex
	cert *tls.Certificate
	// certificate last n read time
	lastRead time.Time
}

// gNMI Handlers
type GetHandler func(ctx context.Context, req *gnmi.GetRequest) (*gnmi.GetResponse, error)

type SetUpdateHandler func(ctx context.Context, p *gnmi.Path, upd *gnmi.Update) (*gnmi.SetResponse, error)

type SetReplaceHandler SetUpdateHandler

type SetDeleteHandler func(ctx context.Context, p *gnmi.Path, del *gnmi.Path) (*gnmi.SetResponse, error)

// Health Handlers
type CheckHandler func(context.Context, *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error)

type WatchHandler func(*healthpb.HealthCheckRequest, healthpb.Health_WatchServer) error

type Option func(*GrpcServer)

func New(c Config, opts ...Option) *GrpcServer {
	c.setDefaults()
	s := &GrpcServer{
		config:             c,
		sem:                semaphore.NewWeighted(c.MaxRPC),
		m:                  &sync.RWMutex{},
		getHandlers:        map[string]GetHandler{},
		setUpdateHandlers:  map[string]SetUpdateHandler{},
		setReplaceHandlers: map[string]SetReplaceHandler{},
		setDeleteHandlers:  map[string]SetDeleteHandler{},
	}

	for _, o := range opts {
		o(s)
	}

	return s
}

func (s *GrpcServer) Start(ctx context.Context) error {
	l, err := net.Listen("tcp", s.config.Address)
	if err != nil {
		return errors.Wrap(err, "cannot listen")
	}
	opts, err := s.serverOpts(ctx)
	if err != nil {
		return err
	}
	// create a gRPC server object
	grpcServer := grpc.NewServer(opts...)

	// register gnmi service to the grpc server
	if s.config.GNMI {
		gnmi.RegisterGNMIServer(grpcServer, s)
		s.logger.Debug("grpc server with gnmi...")
	}
	// register health service to the grpc server
	if s.config.Health {
		healthpb.RegisterHealthServer(grpcServer, s)
		s.logger.Debug("grpc server with health...")
	}
	s.logger.Debug("starting grpc server...")
	if err := grpcServer.Serve(l); err != nil {
		s.logger.Debug("Errors", "error", err)
		return errors.Wrap(err, "cannot serve grpc server")
	}
	return nil
}

func WithClient(c client.Client) func(*GrpcServer) {
	return func(s *GrpcServer) {
		s.client = c
	}
}

func WithGetHandler(origin string, h GetHandler) func(*GrpcServer) {
	return func(s *GrpcServer) {
		s.m.Lock()
		defer s.m.Unlock()
		if s.getHandlers == nil {
			s.getHandlers = make(map[string]GetHandler)
		}
		s.getHandlers[origin] = h
	}
}

func WithSetUpdateHandler(origin string, h SetUpdateHandler) func(*GrpcServer) {
	return func(s *GrpcServer) {
		s.m.Lock()
		defer s.m.Unlock()
		if s.setUpdateHandlers == nil {
			s.setUpdateHandlers = make(map[string]SetUpdateHandler)
		}
		s.setUpdateHandlers[origin] = h
	}
}

func WithSetReplaceHandler(origin string, h SetReplaceHandler) func(*GrpcServer) {
	return func(s *GrpcServer) {
		s.m.Lock()
		defer s.m.Unlock()
		if s.setReplaceHandlers == nil {
			s.setReplaceHandlers = make(map[string]SetReplaceHandler)
		}
		s.setReplaceHandlers[origin] = h
	}
}

func WithSetDeleteHandler(origin string, h SetDeleteHandler) func(*GrpcServer) {
	return func(s *GrpcServer) {
		s.m.Lock()
		defer s.m.Unlock()
		if s.setDeleteHandlers == nil {
			s.setDeleteHandlers = make(map[string]SetDeleteHandler)
		}
		s.setDeleteHandlers[origin] = h
	}
}

func WithCheckHandler(h CheckHandler) func(*GrpcServer) {
	return func(s *GrpcServer) {
		s.checkHandler = h
	}
}

func WithWatchHandler(h WatchHandler) func(*GrpcServer) {
	return func(s *GrpcServer) {
		s.watchHandler = h
	}
}

func (s *GrpcServer) acquireSem(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return s.sem.Acquire(ctx, 1)
	}
}
