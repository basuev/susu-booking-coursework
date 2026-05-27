package outbox

import (
	"context"
	"log/slog"
	"time"
)

const (
	defaultBatchSize       = 50
	defaultInterval        = 500 * time.Millisecond
	defaultMaxAttempts     = 5
	defaultStuckThreshold  = 5 * time.Minute
	defaultRecoverInterval = 30 * time.Second
)

type Publisher interface {
	Publish(ctx context.Context, subject string, payload []byte) error
}

type MessageStore interface {
	ClaimPending(ctx context.Context, limit int) ([]PendingRow, error)
	MarkPublished(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id string, publishErr error, maxAttempts int) error
	RecoverStuck(ctx context.Context, olderThan time.Duration) (int64, error)
}

type Worker struct {
	store           MessageStore
	publisher       Publisher
	batchSize       int
	interval        time.Duration
	maxAttempts     int
	stuckThreshold  time.Duration
	recoverInterval time.Duration
}

type WorkerOption func(*Worker)

func WithBatchSize(n int) WorkerOption {
	return func(w *Worker) {
		if n > 0 {
			w.batchSize = n
		}
	}
}

func WithInterval(d time.Duration) WorkerOption {
	return func(w *Worker) {
		if d > 0 {
			w.interval = d
		}
	}
}

func WithMaxAttempts(n int) WorkerOption {
	return func(w *Worker) {
		if n > 0 {
			w.maxAttempts = n
		}
	}
}

func WithStuckThreshold(d time.Duration) WorkerOption {
	return func(w *Worker) {
		if d > 0 {
			w.stuckThreshold = d
		}
	}
}

func WithRecoverInterval(d time.Duration) WorkerOption {
	return func(w *Worker) {
		if d > 0 {
			w.recoverInterval = d
		}
	}
}

func NewWorker(store MessageStore, publisher Publisher, opts ...WorkerOption) *Worker {
	w := &Worker{
		store:           store,
		publisher:       publisher,
		batchSize:       defaultBatchSize,
		interval:        defaultInterval,
		maxAttempts:     defaultMaxAttempts,
		stuckThreshold:  defaultStuckThreshold,
		recoverInterval: defaultRecoverInterval,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

func (w *Worker) Run(ctx context.Context) error {
	drainTicker := time.NewTicker(w.interval)
	defer drainTicker.Stop()
	recoverTicker := time.NewTicker(w.recoverInterval)
	defer recoverTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-drainTicker.C:
			if err := w.drain(ctx); err != nil {
				slog.ErrorContext(ctx, "outbox worker drain failed", "error", err)
			}
		case <-recoverTicker.C:
			recovered, err := w.store.RecoverStuck(ctx, w.stuckThreshold)
			if err != nil {
				slog.ErrorContext(ctx, "outbox worker recover failed", "error", err)
				continue
			}
			if recovered > 0 {
				slog.WarnContext(ctx, "outbox stuck rows recovered", "count", recovered)
			}
		}
	}
}

func (w *Worker) drain(ctx context.Context) error {
	rows, err := w.store.ClaimPending(ctx, w.batchSize)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if pubErr := w.publisher.Publish(ctx, row.Topic, row.Payload); pubErr != nil {
			slog.ErrorContext(ctx, "outbox publish failed", "id", row.ID, "topic", row.Topic,
				"attempt", row.AttemptCount, "error", pubErr)
			if err := w.store.MarkFailed(ctx, row.ID, pubErr, w.maxAttempts); err != nil {
				slog.ErrorContext(ctx, "outbox mark failed", "id", row.ID, "error", err)
			}
			continue
		}
		if err := w.store.MarkPublished(ctx, row.ID); err != nil {
			slog.ErrorContext(ctx, "outbox mark published failed", "id", row.ID, "error", err)
		} else {
			slog.InfoContext(ctx, "outbox message published", "id", row.ID, "topic", row.Topic)
		}
	}
	return nil
}
