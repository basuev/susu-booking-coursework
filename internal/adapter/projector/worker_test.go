package projector

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"gitverse.ru/basuev/susu-booking-coursework/internal/adapter/postgres"
)

type fakeStore struct {
	mu       sync.Mutex
	inserts  []postgres.BookingProjection
	updates  []statusUpdate
	upsertEr error
}

type statusUpdate struct {
	ID        string
	Status    string
	UpdatedAt time.Time
}

func (f *fakeStore) Upsert(_ context.Context, p postgres.BookingProjection) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.upsertEr != nil {
		return f.upsertEr
	}
	f.inserts = append(f.inserts, p)
	return nil
}

func (f *fakeStore) UpdateStatus(_ context.Context, id, newStatus string, updatedAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updates = append(f.updates, statusUpdate{ID: id, Status: newStatus, UpdatedAt: updatedAt})
	return nil
}

type fakeMsg struct {
	subject string
	data    []byte
	acks    *int
	naks    *int
	terms   *int
	ackErr  error
	mu      *sync.Mutex
}

func (m *fakeMsg) Metadata() (*jetstream.MsgMetadata, error) { return &jetstream.MsgMetadata{}, nil }
func (m *fakeMsg) Data() []byte                              { return m.data }
func (m *fakeMsg) Headers() nats.Header                      { return nil }
func (m *fakeMsg) Subject() string                           { return m.subject }
func (m *fakeMsg) Reply() string                             { return "" }
func (m *fakeMsg) Ack() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	*m.acks++
	return m.ackErr
}
func (m *fakeMsg) DoubleAck(_ context.Context) error { return m.Ack() }
func (m *fakeMsg) Nak() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	*m.naks++
	return nil
}
func (m *fakeMsg) NakWithDelay(_ time.Duration) error { return m.Nak() }
func (m *fakeMsg) InProgress() error                  { return nil }
func (m *fakeMsg) Term() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	*m.terms++
	return nil
}
func (m *fakeMsg) TermWithReason(_ string) error { return m.Term() }

func newMsg(subject string, data []byte, counters *msgCounters) *fakeMsg {
	return &fakeMsg{
		subject: subject,
		data:    data,
		acks:    &counters.acks,
		naks:    &counters.naks,
		terms:   &counters.terms,
		mu:      &counters.mu,
	}
}

type msgCounters struct {
	mu    sync.Mutex
	acks  int
	naks  int
	terms int
}

func createdPayload(t *testing.T, id string) []byte {
	t.Helper()
	ts := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	raw, err := json.Marshal(map[string]any{
		"booking_id": id,
		"guest_id":   "g-1",
		"hotel_id":   "h-1",
		"room_type":  "deluxe",
		"check_in":   ts,
		"check_out":  ts.AddDate(0, 0, 1),
		"total":      map[string]any{"amount": 12345, "currency": "RUB"},
		"timestamp":  ts,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

func TestWorker_handle_BookingCreatedCallsUpsertAndAcks(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	w := NewWorker(store, nil)
	counters := &msgCounters{}
	msg := newMsg("booking.created", createdPayload(t, "b-1"), counters)

	w.handle(context.Background(), msg)

	if len(store.inserts) != 1 || store.inserts[0].ID != "b-1" {
		t.Fatalf("upsert not called or wrong id: %+v", store.inserts)
	}
	if counters.acks != 1 || counters.naks != 0 || counters.terms != 0 {
		t.Fatalf("ack counters mismatch: %+v", counters)
	}
}

func TestWorker_handle_DuplicateCreatedRemainsIdempotent(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	w := NewWorker(store, nil)
	counters := &msgCounters{}
	payload := createdPayload(t, "b-dup")

	w.handle(context.Background(), newMsg("booking.created", payload, counters))
	w.handle(context.Background(), newMsg("booking.created", payload, counters))

	if len(store.inserts) != 2 {
		t.Fatalf("each delivery should call Upsert (idempotency lives in store): got %d", len(store.inserts))
	}
	for _, p := range store.inserts {
		if p.ID != "b-dup" {
			t.Fatalf("all inserts must share id b-dup: %+v", store.inserts)
		}
	}
	if counters.acks != 2 {
		t.Fatalf("both messages must be acked: %+v", counters)
	}
}

func TestWorker_handle_StatusEventCallsUpdateAndAcks(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	w := NewWorker(store, nil)
	counters := &msgCounters{}
	ts := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	raw, _ := json.Marshal(map[string]any{
		"booking_id": "b-2",
		"old_status": "PENDING",
		"new_status": "APPROVED",
		"timestamp":  ts,
	})

	w.handle(context.Background(), newMsg("booking.approved", raw, counters))

	if len(store.updates) != 1 {
		t.Fatalf("update not recorded: %+v", store.updates)
	}
	u := store.updates[0]
	if u.ID != "b-2" || u.Status != "APPROVED" || !u.UpdatedAt.Equal(ts) {
		t.Fatalf("wrong update: %+v", u)
	}
	if counters.acks != 1 {
		t.Fatalf("ack expected, got %+v", counters)
	}
}

func TestWorker_handle_StoreErrorNaks(t *testing.T) {
	t.Parallel()

	store := &fakeStore{upsertEr: errors.New("db down")}
	w := NewWorker(store, nil)
	counters := &msgCounters{}

	w.handle(context.Background(), newMsg("booking.created", createdPayload(t, "b-3"), counters))

	if counters.naks != 1 || counters.acks != 0 {
		t.Fatalf("expected single nak and no ack: %+v", counters)
	}
}

func TestWorker_handle_UnknownSubjectTerminates(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	w := NewWorker(store, nil)
	counters := &msgCounters{}

	w.handle(context.Background(), newMsg("booking.weird", []byte("{}"), counters))

	if counters.terms != 1 {
		t.Fatalf("unknown subject must be Term-ed: %+v", counters)
	}
	if len(store.inserts) != 0 || len(store.updates) != 0 {
		t.Fatalf("store must not be touched: inserts=%v updates=%v", store.inserts, store.updates)
	}
}

type fakeSubscriber struct {
	consumed chan struct{}
	stopped  chan struct{}
	handler  func(jetstream.Msg)
}

func (f *fakeSubscriber) Consume(_ context.Context, _, _ string, h func(jetstream.Msg)) (jetstream.ConsumeContext, error) {
	f.handler = h
	close(f.consumed)
	return &fakeConsumeContext{stopped: f.stopped}, nil
}

type fakeConsumeContext struct {
	stopped chan struct{}
}

func (c *fakeConsumeContext) Stop()  { close(c.stopped) }
func (c *fakeConsumeContext) Drain() {}
func (c *fakeConsumeContext) Closed() <-chan struct{} {
	ch := make(chan struct{})
	return ch
}

func TestWorker_Run_StopsOnContextCancel(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	sub := &fakeSubscriber{
		consumed: make(chan struct{}),
		stopped:  make(chan struct{}),
	}
	w := NewWorker(store, sub)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()

	select {
	case <-sub.consumed:
	case <-time.After(time.Second):
		t.Fatalf("Consume was not called")
	}

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("worker did not stop")
	}

	select {
	case <-sub.stopped:
	case <-time.After(time.Second):
		t.Fatalf("consume context not stopped")
	}
}
