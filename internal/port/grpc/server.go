package grpc

import (
	"context"
	"log/slog"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Server struct {
	gs       *grpc.Server
	listener net.Listener
}

func NewServer(addr string, handler *BookingHandler) (*Server, error) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	gs := grpc.NewServer()
	handler.Register(gs)
	reflection.Register(gs)

	return &Server{gs: gs, listener: lis}, nil
}

func (s *Server) Start() error {
	slog.Info("gRPC server listening", "addr", s.listener.Addr().String())
	return s.gs.Serve(s.listener)
}

func (s *Server) Shutdown(_ context.Context) {
	slog.Info("gRPC server shutting down")
	s.gs.GracefulStop()
}
