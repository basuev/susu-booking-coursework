package grpc

import (
	"context"
	"log/slog"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

type Server struct {
	gs       *grpc.Server
	listener net.Listener
	health   *health.Server
}

func NewServer(addr string, handler *BookingHandler, opts ...grpc.ServerOption) (*Server, error) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	gs := grpc.NewServer(opts...)
	handler.Register(gs)
	hs := health.NewServer()
	healthpb.RegisterHealthServer(gs, hs)
	reflection.Register(gs)

	return &Server{gs: gs, listener: lis, health: hs}, nil
}

func (s *Server) Start() error {
	slog.Info("grpc server listening", "addr", s.listener.Addr().String())
	return s.gs.Serve(s.listener)
}

func (s *Server) Health() *health.Server {
	return s.health
}

func (s *Server) Shutdown(ctx context.Context) {
	slog.InfoContext(ctx, "grpc server shutting down")
	if s.health != nil {
		s.health.Shutdown()
	}
	done := make(chan struct{})
	go func() {
		s.gs.GracefulStop()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		slog.WarnContext(ctx, "grpc graceful stop deadline exceeded, forcing stop")
		s.gs.Stop()
	}
}
