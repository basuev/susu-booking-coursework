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
	"google.golang.org/grpc"

	"gitverse.ru/basuev/susu-booking-coursework/internal/adapter/natsx"
	"gitverse.ru/basuev/susu-booking-coursework/internal/adapter/outbox"
	"gitverse.ru/basuev/susu-booking-coursework/internal/adapter/postgres"
	"gitverse.ru/basuev/susu-booking-coursework/internal/adapter/projector"
	"gitverse.ru/basuev/susu-booking-coursework/internal/app/command"
	"gitverse.ru/basuev/susu-booking-coursework/internal/app/query"
	"gitverse.ru/basuev/susu-booking-coursework/internal/config"
	grpcport "gitverse.ru/basuev/susu-booking-coursework/internal/port/grpc"
	"gitverse.ru/basuev/susu-booking-coursework/internal/port/grpc/interceptors"
	httpport "gitverse.ru/basuev/susu-booking-coursework/internal/port/http"
	"gitverse.ru/basuev/susu-booking-coursework/migrations"
)

func main() {
	cfg := config.Load()
	setupLogger(cfg.LogLevel)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		slog.ErrorContext(ctx, "failed to open database", "error", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	db.SetMaxOpenConns(cfg.DBMaxOpenConns)
	db.SetMaxIdleConns(cfg.DBMaxIdleConns)
	db.SetConnMaxLifetime(cfg.DBConnMaxLifetime)

	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()
	if err := db.PingContext(pingCtx); err != nil {
		slog.ErrorContext(ctx, "failed to ping database", "error", err)
		os.Exit(1)
	}

	if cfg.RunMigrations {
		if err := postgres.RunMigrations(db, migrations.FS); err != nil {
			slog.ErrorContext(ctx, "auto-migration failed", "error", err)
			os.Exit(1)
		}
		slog.InfoContext(ctx, "migrations applied")
	}

	natsClient, err := natsx.Connect(ctx, cfg.NATSUrl)
	if err != nil {
		slog.ErrorContext(ctx, "failed to connect to nats", "error", err)
		os.Exit(1)
	}
	defer natsClient.Close()

	repo := postgres.NewBookingRepo(db)
	idempotencyRepo := postgres.NewIdempotencyRepo(db)
	txManager := postgres.NewTxManager(db)
	projectionRepo := postgres.NewProjectionRepo(db)
	outboxRepo := outbox.NewRepo(db)
	outboxWorker := outbox.NewWorker(
		outboxRepo,
		natsClient,
		outbox.WithBatchSize(cfg.OutboxBatchSize),
		outbox.WithInterval(cfg.OutboxInterval),
		outbox.WithMaxAttempts(cfg.OutboxMaxAttempts),
	)
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

	srv, err := grpcport.NewServer(":"+cfg.GRPCPort, handler,
		grpc.ChainUnaryInterceptor(
			interceptors.Recovery(),
			interceptors.RequestID(),
			interceptors.Timeout(cfg.RPCTimeout),
			interceptors.Logging(),
		),
	)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create grpc server", "error", err)
		os.Exit(1)
	}

	healthSrv := httpport.NewServer(cfg.HTTPAddr, db, natsClient)

	var wg sync.WaitGroup
	grpcErr := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		grpcErr <- srv.Start()
	}()

	httpErr := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		httpErr <- healthSrv.Start()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := outboxWorker.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			slog.ErrorContext(ctx, "outbox worker stopped", "error", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := projectorWorker.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			slog.ErrorContext(ctx, "projector worker stopped", "error", err)
		}
	}()

	select {
	case <-ctx.Done():
		slog.InfoContext(ctx, "shutdown signal received")
	case err := <-grpcErr:
		slog.ErrorContext(ctx, "grpc server error", "error", err)
		cancel()
	case err := <-httpErr:
		slog.ErrorContext(ctx, "http server error", "error", err)
		cancel()
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()
	srv.Shutdown(shutdownCtx)
	if err := healthSrv.Shutdown(shutdownCtx); err != nil {
		slog.ErrorContext(shutdownCtx, "http server shutdown error", "error", err)
	}
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
