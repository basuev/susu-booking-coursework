package outbox

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeStore struct {
	mu         sync.Mutex
	pending    []PendingRow
	published  []string
	failed     []failedRecord
	recovered  int64
	pendingErr error
}

type failedRecord struct {
	ID          string
	Err         string
	MaxAttempts int
}

func (f *fakeStore) ClaimPending(_ context.Context, limit int) ([]PendingRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pendingErr != nil {
		return nil, f.pendingErr
	}
	n := limit
	if n > len(f.pending) {
		n = len(f.pending)
	}
	out := make([]PendingRow, n)
	for i := 0; i < n; i++ {
		row := f.pending[i]
		row.AttemptCount++
		out[i] = row
	}
	f.pending = f.pending[n:]
	return out, nil
}

func (f *fakeStore) MarkPublished(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.published = append(f.published, id)
	return nil
}

func (f *fakeStore) MarkFailed(_ context.Context, id string, publishErr error, maxAttempts int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failed = append(f.failed, failedRecord{ID: id, Err: publishErr.Error(), MaxAttempts: maxAttempts})
	return nil
}

func (f *fakeStore) RecoverStuck(_ context.Context, _ time.Duration) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.recovered, nil
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
	if len(store.failed) != 0 {
		t.Fatalf("expected no failures, got %v", store.failed)
	}
}

func TestWorker_drain_FailureCallsMarkFailed(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		pending: []PendingRow{
			{ID: "1", Topic: "booking.created", Key: "b-1", Payload: []byte("{}")},
			{ID: "2", Topic: "booking.approved", Key: "b-2", Payload: []byte("{}")},
		},
	}
	publisher := &fakePublisher{failOn: map[string]bool{"booking.created": true}}

	w := NewWorker(store, publisher, WithMaxAttempts(7))
	if err := w.drain(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(store.published) != 1 || store.published[0] != "2" {
		t.Fatalf("only successful publish must be marked published, got %v", store.published)
	}
	if len(store.failed) != 1 || store.failed[0].ID != "1" {
		t.Fatalf("expected one MarkFailed call for id=1, got %v", store.failed)
	}
	if store.failed[0].MaxAttempts != 7 {
		t.Fatalf("MarkFailed must receive configured max attempts: want 7, got %d", store.failed[0].MaxAttempts)
	}
}

func TestNewRepo_Construct(t *testing.T) {
	t.Parallel()
	if NewRepo(nil) == nil {
		t.Fatalf("NewRepo returned nil")
	}
}

func TestWorker_Options(t *testing.T) {
	t.Parallel()
	w := NewWorker(&fakeStore{}, &fakePublisher{},
		WithBatchSize(7),
		WithInterval(50*time.Millisecond),
		WithMaxAttempts(9),
		WithStuckThreshold(2*time.Minute),
		WithRecoverInterval(3*time.Second),
		WithBatchSize(0),
		WithInterval(0),
		WithMaxAttempts(0),
		WithStuckThreshold(0),
		WithRecoverInterval(0),
	)
	if w.batchSize != 7 {
		t.Errorf("batchSize: want 7, got %d", w.batchSize)
	}
	if w.interval != 50*time.Millisecond {
		t.Errorf("interval not set")
	}
	if w.maxAttempts != 9 {
		t.Errorf("maxAttempts: want 9, got %d", w.maxAttempts)
	}
	if w.stuckThreshold != 2*time.Minute {
		t.Errorf("stuckThreshold not set")
	}
	if w.recoverInterval != 3*time.Second {
		t.Errorf("recoverInterval not set")
	}
}

func TestWorker_drain_ClaimErrorPropagates(t *testing.T) {
	t.Parallel()
	store := &fakeStore{pendingErr: errors.New("db boom")}
	w := NewWorker(store, &fakePublisher{})
	err := w.drain(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestWorker_Run_DrainsAndRecovers(t *testing.T) {
	t.Parallel()
	store := &fakeStore{
		pending: []PendingRow{
			{ID: "1", Topic: "booking.created", Key: "b-1", Payload: []byte("{}")},
		},
		recovered: 3,
	}
	publisher := &fakePublisher{}
	w := NewWorker(store, publisher,
		WithInterval(5*time.Millisecond),
		WithRecoverInterval(5*time.Millisecond),
		WithStuckThreshold(time.Minute),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := w.Run(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want DeadlineExceeded, got %v", err)
	}
	if len(publisher.published) == 0 {
		t.Fatalf("expected at least one publish")
	}
}

func TestWorker_Run_RespectsContextCancel(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	publisher := &fakePublisher{}
	w := NewWorker(store, publisher,
		WithInterval(10*time.Millisecond),
		WithRecoverInterval(10*time.Millisecond),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	err := w.Run(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want DeadlineExceeded, got %v", err)
	}
}
