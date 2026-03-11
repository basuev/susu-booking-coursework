package main

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/basuev/susu-booking-coursework/internal/adapter/postgres"
	"github.com/basuev/susu-booking-coursework/internal/app/command"
	"github.com/basuev/susu-booking-coursework/internal/app/query"
	"github.com/basuev/susu-booking-coursework/internal/config"
	grpcport "github.com/basuev/susu-booking-coursework/internal/port/grpc"
)

func main() {
	cfg := config.Load()
	setupLogger(cfg.LogLevel)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		slog.Error("failed to ping database", "error", err)
		os.Exit(1)
	}

	repo := postgres.NewBookingRepo(db)

	createHandler := command.NewCreateBookingHandler(repo)
	cancelHandler := command.NewCancelBookingHandler(repo)
	approveHandler := command.NewApproveBookingHandler(repo)
	rejectHandler := command.NewRejectBookingHandler(repo)
	getHandler := query.NewGetBookingHandler(repo)
	listHandler := query.NewListBookingsHandler(repo)

	handler := grpcport.NewBookingHandler(
		createHandler, cancelHandler, approveHandler, rejectHandler,
		getHandler, listHandler,
	)

	srv, err := grpcport.NewServer(":"+cfg.GRPCPort, handler)
	if err != nil {
		slog.Error("failed to create gRPC server", "error", err)
		os.Exit(1)
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()

	select {
	case <-ctx.Done():
		srv.Shutdown(context.Background())
	case err := <-errCh:
		slog.Error("gRPC server error", "error", err)
		os.Exit(1)
	}
}

func setupLogger(level string) {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})))
}
