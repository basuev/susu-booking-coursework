package http

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/nats-io/nats.go"
)

type natsStatusProvider interface {
	Status() nats.Status
}

type Server struct {
	httpSrv *http.Server
	db      *sql.DB
	nc      natsStatusProvider
}

func NewServer(addr string, db *sql.DB, nc natsStatusProvider) *Server {
	s := &Server{db: db, nc: nc}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	s.httpSrv = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return s
}

func (s *Server) Start() error {
	slog.Info("http server listening", "addr", s.httpSrv.Addr)
	if err := s.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	slog.InfoContext(ctx, "http server shutting down")
	return s.httpSrv.Shutdown(ctx)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if s.db != nil {
		if err := s.db.PingContext(ctx); err != nil {
			slog.WarnContext(ctx, "healthz db ping failed", "error", err)
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
	}
	if s.nc != nil && s.nc.Status() != nats.CONNECTED {
		slog.WarnContext(ctx, "healthz nats not connected", "status", s.nc.Status())
		http.Error(w, "nats unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
