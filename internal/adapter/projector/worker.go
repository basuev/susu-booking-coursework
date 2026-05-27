package projector

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"gitverse.ru/basuev/susu-booking-coursework/internal/adapter/postgres"
)

const (
	defaultDurable       = "booking-projector"
	defaultFilterSubject = "booking.>"
)

type Store interface {
	Upsert(ctx context.Context, p postgres.BookingProjection) error
	UpdateStatus(ctx context.Context, id, newStatus string, updatedAt time.Time) error
}

type Subscriber interface {
	Consume(ctx context.Context, durable, filterSubject string, handler func(jetstream.Msg)) (jetstream.ConsumeContext, error)
}

type Worker struct {
	store         Store
	sub           Subscriber
	durable       string
	filterSubject string
}

type Option func(*Worker)

func WithDurable(name string) Option {
	return func(w *Worker) {
		if name != "" {
			w.durable = name
		}
	}
}

func WithFilterSubject(s string) Option {
	return func(w *Worker) {
		if s != "" {
			w.filterSubject = s
		}
	}
}

func NewWorker(store Store, sub Subscriber, opts ...Option) *Worker {
	w := &Worker{
		store:         store,
		sub:           sub,
		durable:       defaultDurable,
		filterSubject: defaultFilterSubject,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

func (w *Worker) Run(ctx context.Context) error {
	cc, err := w.sub.Consume(ctx, w.durable, w.filterSubject, func(msg jetstream.Msg) {
		w.handle(ctx, msg)
	})
	if err != nil {
		return fmt.Errorf("projector subscribe: %w", err)
	}
	defer cc.Stop()
	<-ctx.Done()
	return ctx.Err()
}

func (w *Worker) handle(ctx context.Context, msg jetstream.Msg) {
	op, err := Decode(msg.Subject(), msg.Data())
	if err != nil {
		slog.ErrorContext(ctx, "projector decode failed", "subject", msg.Subject(), "error", err)
		_ = msg.Term()
		return
	}
	if err := w.apply(ctx, op); err != nil {
		slog.ErrorContext(ctx, "projector apply failed", "subject", msg.Subject(), "error", err)
		_ = msg.Nak()
		return
	}
	if err := msg.Ack(); err != nil {
		slog.ErrorContext(ctx, "projector ack failed", "subject", msg.Subject(), "error", err)
		return
	}
	slog.InfoContext(ctx, "projection applied", "subject", msg.Subject())
}

func (w *Worker) apply(ctx context.Context, op Op) error {
	switch op.Kind {
	case OpInsert:
		return w.store.Upsert(ctx, op.Insert)
	case OpUpdateStatus:
		return w.store.UpdateStatus(ctx, op.ID, op.Status, op.UpdatedAt)
	default:
		return fmt.Errorf("unknown op kind %d", op.Kind)
	}
}
