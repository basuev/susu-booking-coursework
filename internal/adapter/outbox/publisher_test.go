package outbox

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeStore struct {
	mu          sync.Mutex
	pending     []PendingRow
	published   []string
	pendingErr  error
	publishCall int
}

func (f *fakeStore) SelectPending(_ context.Context, limit int) ([]PendingRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pendingErr != nil {
		return nil, f.pendingErr
	}
	if len(f.pending) <= limit {
		out := f.pending
		f.pending = nil
		return out, nil
	}
	out := f.pending[:limit]
	f.pending = f.pending[limit:]
	return out, nil
}

func (f *fakeStore) MarkPublished(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.published = append(f.published, id)
	return nil
}

type fakePublisher struct {
	mu        sync.Mutex
	failOn    map[string]bool
	published []PendingRow
}

func (p *fakePublisher) Publish(_ context.Context, subject string, payload []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failOn[subject] {
		return errors.New("simulated publish failure")
	}
	p.published = append(p.published, PendingRow{Topic: subject, Payload: payload})
	return nil
}

func TestWorker_drain_PublishesAllAndMarks(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		pending: []PendingRow{
			{ID: "1", Topic: "booking.created", Key: "b-1", Payload: []byte("{}")},
			{ID: "2", Topic: "booking.approved", Key: "b-2", Payload: []byte("{}")},
		},
	}
	publisher := &fakePublisher{}

	w := NewWorker(store, publisher, WithBatchSize(10))
	if err := w.drain(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(publisher.published) != 2 {
		t.Fatalf("published count: want 2, got %d", len(publisher.published))
	}
	if len(store.published) != 2 || store.published[0] != "1" || store.published[1] != "2" {
		t.Fatalf("marked ids: want [1 2], got %v", store.published)
	}
}

func TestWorker_drain_FailureDoesNotMark(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		pending: []PendingRow{
			{ID: "1", Topic: "booking.created", Key: "b-1", Payload: []byte("{}")},
			{ID: "2", Topic: "booking.approved", Key: "b-2", Payload: []byte("{}")},
		},
	}
	publisher := &fakePublisher{failOn: map[string]bool{"booking.created": true}}

	w := NewWorker(store, publisher)
	if err := w.drain(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(store.published) != 1 || store.published[0] != "2" {
		t.Fatalf("only successful publish should be marked, got %v", store.published)
	}
}

func TestWorker_Run_RespectsContextCancel(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	publisher := &fakePublisher{}
	w := NewWorker(store, publisher, WithInterval(10*time.Millisecond))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	err := w.Run(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want DeadlineExceeded, got %v", err)
	}
}
