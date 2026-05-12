package outbox

import (
	"context"
	"log/slog"
	"time"
)

const (
	defaultBatchSize = 50
	defaultInterval  = 500 * time.Millisecond
)

type Publisher interface {
	Publish(ctx context.Context, subject string, payload []byte) error
}

type MessageStore interface {
	SelectPending(ctx context.Context, limit int) ([]PendingRow, error)
	MarkPublished(ctx context.Context, id string) error
}

type Worker struct {
	store     MessageStore
	publisher Publisher
	batchSize int
	interval  time.Duration
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

func NewWorker(store MessageStore, publisher Publisher, opts ...WorkerOption) *Worker {
	w := &Worker{
		store:     store,
		publisher: publisher,
		batchSize: defaultBatchSize,
		interval:  defaultInterval,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

func (w *Worker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := w.drain(ctx); err != nil {
				slog.Error("outbox worker drain failed", "error", err)
			}
		}
	}
}

func (w *Worker) drain(ctx context.Context) error {
	rows, err := w.store.SelectPending(ctx, w.batchSize)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := w.publisher.Publish(ctx, row.Topic, row.Payload); err != nil {
			slog.Error("outbox publish failed", "id", row.ID, "topic", row.Topic, "error", err)
			continue
		}
		if err := w.store.MarkPublished(ctx, row.ID); err != nil {
			slog.Error("outbox mark published failed", "id", row.ID, "error", err)
		}
	}
	return nil
}
