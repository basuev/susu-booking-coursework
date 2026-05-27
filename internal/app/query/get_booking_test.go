package query

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"gitverse.ru/basuev/susu-booking-coursework/internal/domain"
	"gitverse.ru/basuev/susu-booking-coursework/internal/domain/booking"
)

type fakeBookingRepo struct {
	mu          sync.Mutex
	stored      map[string]*booking.Booking
	history     map[string][]booking.StatusHistoryEntry
	historyErr  error
	findErr     error
	saveErrOnce error
}

func newFakeBookingRepo() *fakeBookingRepo {
	return &fakeBookingRepo{
		stored:  map[string]*booking.Booking{},
		history: map[string][]booking.StatusHistoryEntry{},
	}
}

func (r *fakeBookingRepo) Save(_ context.Context, b *booking.Booking) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.saveErrOnce != nil {
		err := r.saveErrOnce
		r.saveErrOnce = nil
		return err
	}
	r.stored[b.ID()] = b
	return nil
}

func (r *fakeBookingRepo) FindByID(_ context.Context, id string) (*booking.Booking, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.findErr != nil {
		return nil, r.findErr
	}
	b, ok := r.stored[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return b, nil
}

func (r *fakeBookingRepo) GetStatusHistory(_ context.Context, bookingID string) ([]booking.StatusHistoryEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.historyErr != nil {
		return nil, r.historyErr
	}
	return r.history[bookingID], nil
}

func reconstructPending(t *testing.T, id string) *booking.Booking {
	t.Helper()
	price, err := booking.NewMoney(10000, "RUB")
	if err != nil {
		t.Fatalf("price: %v", err)
	}
	offer, err := booking.NewOfferSnapshot("offer-1", "hotel-1", "STANDARD", price)
	if err != nil {
		t.Fatalf("offer: %v", err)
	}
	stay, err := booking.NewStayPeriod(
		time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("stay: %v", err)
	}
	total, err := booking.NewMoney(30000, "RUB")
	if err != nil {
		t.Fatalf("total: %v", err)
	}
	now := time.Now()
	return booking.Reconstruct(id, "guest-1", offer, stay, total, booking.StatusPending, 1, now, now)
}

func TestGetBooking_NotFound(t *testing.T) {
	t.Parallel()
	repo := newFakeBookingRepo()
	h := NewGetBookingHandler(repo)

	_, err := h.Handle(context.Background(), GetBooking{BookingID: "missing"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestGetBooking_Success(t *testing.T) {
	t.Parallel()
	repo := newFakeBookingRepo()
	repo.stored["id-1"] = reconstructPending(t, "id-1")
	repo.history["id-1"] = []booking.StatusHistoryEntry{
		{OldStatus: booking.StatusPending, NewStatus: booking.StatusApproved, ChangedAt: time.Now()},
	}
	h := NewGetBookingHandler(repo)

	res, err := h.Handle(context.Background(), GetBooking{BookingID: "id-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Booking == nil || res.Booking.ID() != "id-1" {
		t.Fatalf("want booking id-1, got %#v", res.Booking)
	}
	if len(res.History) != 1 {
		t.Fatalf("want 1 history entry, got %d", len(res.History))
	}
}

func TestGetBooking_HistoryErrorPropagates(t *testing.T) {
	t.Parallel()
	repo := newFakeBookingRepo()
	repo.stored["id-1"] = reconstructPending(t, "id-1")
	boom := errors.New("history boom")
	repo.historyErr = boom
	h := NewGetBookingHandler(repo)

	_, err := h.Handle(context.Background(), GetBooking{BookingID: "id-1"})
	if !errors.Is(err, boom) {
		t.Fatalf("want %v, got %v", boom, err)
	}
}
