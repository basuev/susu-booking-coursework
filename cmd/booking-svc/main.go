package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"gitverse.ru/basuev/susu-booking-coursework/internal/adapter/natsx"
	"gitverse.ru/basuev/susu-booking-coursework/internal/adapter/outbox"
	"gitverse.ru/basuev/susu-booking-coursework/internal/adapter/postgres"
	"gitverse.ru/basuev/susu-booking-coursework/internal/adapter/projector"
	"gitverse.ru/basuev/susu-booking-coursework/internal/app/command"
	"gitverse.ru/basuev/susu-booking-coursework/internal/app/query"
	"gitverse.ru/basuev/susu-booking-coursework/internal/config"
	grpcport "gitverse.ru/basuev/susu-booking-coursework/internal/port/grpc"
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
	defer func() { _ = db.Close() }()

	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()
	if err := db.PingContext(pingCtx); err != nil {
		slog.Error("failed to ping database", "error", err)
		os.Exit(1)
	}

	natsClient, err := natsx.Connect(ctx, cfg.NATSUrl)
	if err != nil {
		slog.Error("failed to connect to NATS", "error", err)
		os.Exit(1)
	}
	defer natsClient.Close()

	repo := postgres.NewBookingRepo(db)
	idempotencyRepo := postgres.NewIdempotencyRepo(db)
	txManager := postgres.NewTxManager(db)
	projectionRepo := postgres.NewProjectionRepo(db)
	outboxRepo := outbox.NewRepo(db)
	outboxWorker := outbox.NewWorker(outboxRepo, natsClient)
	projectorWorker := projector.NewWorker(projectionRepo, natsClient)

	createHandler := command.NewCreateBookingHandler(repo, idempotencyRepo, txManager)
	cancelHandler := command.NewCancelBookingHandler(repo)
	approveHandler := command.NewApproveBookingHandler(repo)
	rejectHandler := command.NewRejectBookingHandler(repo)
	getHandler := query.NewGetBookingHandler(repo)
	listHandler := query.NewListBookingsHandler(projectionRepo)

	handler := grpcport.NewBookingHandler(
		createHandler, cancelHandler, approveHandler, rejectHandler,
		getHandler, listHandler,
	)

	srv, err := grpcport.NewServer(":"+cfg.GRPCPort, handler)
	if err != nil {
		slog.Error("failed to create gRPC server", "error", err)
		os.Exit(1)
	}

	var wg sync.WaitGroup
	grpcErr := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		grpcErr <- srv.Start()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := outboxWorker.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("outbox worker stopped", "error", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := projectorWorker.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("projector worker stopped", "error", err)
		}
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	case err := <-grpcErr:
		slog.Error("gRPC server error", "error", err)
		cancel()
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	srv.Shutdown(shutdownCtx)
	wg.Wait()
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
