package grpcserver

import (
	"context"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *GrpcServer) Get(ctx context.Context, req *gnmi.GetRequest) (*gnmi.GetResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()
	err := s.acquireSem(ctx)
	if err != nil {
		return nil, err
	}
	defer s.sem.Release(1)
	paths := req.GetPath()
	if len(paths) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "missing path")
	}
	getRsp := &gnmi.GetResponse{
		Notification: make([]*gnmi.Notification, 0, len(paths)),
	}
	for _, p := range paths {
		origin := getOrigin(req.GetPrefix(), p)
		s.m.RLock()
		h, ok := s.getHandlers[origin]
		s.m.RUnlock()
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "unknown origin %q", origin)
		}
		rsp, err := h(ctx, req)
		if err != nil {
			return nil, err
		}
		getRsp.Notification = append(getRsp.Notification, rsp.GetNotification()...)
	}
	return getRsp, nil
}

func (s *GrpcServer) Set(ctx context.Context, req *gnmi.SetRequest) (*gnmi.SetResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()
	err := s.acquireSem(ctx)
	if err != nil {
		return nil, err
	}
	defer s.sem.Release(1)

	numUpdates := len(req.GetUpdate())
	numReplaces := len(req.GetReplace())
	numDeletes := len(req.GetDelete())
	if numDeletes+numReplaces+numUpdates == 0 {
		return nil, status.Error(codes.InvalidArgument, "missing operations")
	}

	setRps := &gnmi.SetResponse{
		Prefix:   req.GetPrefix(),
		Response: make([]*gnmi.UpdateResult, 0, numDeletes+numReplaces+numUpdates),
	}
	// deletes
	for _, del := range req.GetDelete() {
		origin := getOrigin(req.GetPrefix(), del)
		s.m.RLock()
		h, ok := s.setDeleteHandlers[origin]
		s.m.RUnlock()
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "unknown origin %q", origin)
		}
		rsp, err := h(ctx, req.GetPrefix(), del)
		if err != nil {
			return nil, err
		}
		setRps.Response = append(setRps.Response, rsp.GetResponse()...)
	}
	// replaces
	for _, upd := range req.GetReplace() {
		origin := getOrigin(req.GetPrefix(), upd.GetPath())
		s.m.RLock()
		h, ok := s.setReplaceHandlers[origin]
		s.m.RUnlock()
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "unknown origin %q", origin)
		}
		rsp, err := h(ctx, req.GetPrefix(), upd)
		if err != nil {
			return nil, err
		}
		setRps.Response = append(setRps.Response, rsp.GetResponse()...)
	}
	// updates
	for _, upd := range req.GetUpdate() {
		origin := getOrigin(req.GetPrefix(), upd.GetPath())
		s.m.RLock()
		h, ok := s.setUpdateHandlers[origin]
		s.m.RUnlock()
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "unknown origin %q", origin)
		}
		rsp, err := h(ctx, req.GetPrefix(), upd)
		if err != nil {
			return nil, err
		}
		setRps.Response = append(setRps.Response, rsp.GetResponse()...)
	}
	setRps.Timestamp = time.Now().UnixNano()
	return setRps, nil
}

func getOrigin(pf, p *gnmi.Path) string {
	if p.GetOrigin() != "" {
		return pf.GetOrigin()
	}
	return p.GetOrigin()
}
