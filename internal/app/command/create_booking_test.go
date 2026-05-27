package command

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
	mu       sync.Mutex
	stored   map[string]*booking.Booking
	saveErrs []error
	saves    int
}

func newFakeBookingRepo() *fakeBookingRepo {
	return &fakeBookingRepo{stored: map[string]*booking.Booking{}}
}

func (r *fakeBookingRepo) Save(_ context.Context, b *booking.Booking) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	idx := r.saves
	r.saves++
	if idx < len(r.saveErrs) && r.saveErrs[idx] != nil {
		return r.saveErrs[idx]
	}
	r.stored[b.ID()] = b
	return nil
}

func (r *fakeBookingRepo) FindByID(_ context.Context, id string) (*booking.Booking, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.stored[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return b, nil
}

func (r *fakeBookingRepo) List(_ context.Context, _ booking.ListFilter) ([]*booking.Booking, error) {
	return nil, nil
}

func (r *fakeBookingRepo) GetStatusHistory(_ context.Context, _ string) ([]booking.StatusHistoryEntry, error) {
	return nil, nil
}

type fakeIdempotencyStore struct {
	mu             sync.Mutex
	entries        map[string]string
	onSaveConflict map[string]string
	saveCalls      int
}

func newFakeIdempotency() *fakeIdempotencyStore {
	return &fakeIdempotencyStore{
		entries:        map[string]string{},
		onSaveConflict: map[string]string{},
	}
}

func (s *fakeIdempotencyStore) Lookup(_ context.Context, hash string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.entries[hash]
	return v, ok, nil
}

func (s *fakeIdempotencyStore) Save(_ context.Context, hash, bookingID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saveCalls++
	if winner, ok := s.onSaveConflict[hash]; ok {
		s.entries[hash] = winner
		delete(s.onSaveConflict, hash)
		return ErrIdempotencyConflict
	}
	if _, exists := s.entries[hash]; exists {
		return ErrIdempotencyConflict
	}
	s.entries[hash] = bookingID
	return nil
}

type inlineTxManager struct{}

func (inlineTxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func validCmd(key string) CreateBooking {
	return CreateBooking{
		IdempotencyKey: key,
		GuestID:        "guest-1",
		OfferID:        "offer-1",
		HotelID:        "hotel-1",
		RoomType:       "STANDARD",
		PricePerNight:  10000,
		Currency:       "RUB",
		CheckIn:        time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		CheckOut:       time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC),
	}
}

func TestCreateBooking_HappyPath(t *testing.T) {
	t.Parallel()

	repo := newFakeBookingRepo()
	idem := newFakeIdempotency()
	h := NewCreateBookingHandler(repo, idem, inlineTxManager{})

	b, err := h.Handle(context.Background(), validCmd("key-1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b == nil || b.ID() == "" {
		t.Fatalf("expected booking with id")
	}
	if repo.saves != 1 {
		t.Fatalf("repo.Save calls: want 1, got %d", repo.saves)
	}
	if len(idem.entries) != 1 {
		t.Fatalf("idempotency entries: want 1, got %d", len(idem.entries))
	}
}

func TestCreateBooking_RejectsEmptyKey(t *testing.T) {
	t.Parallel()

	h := NewCreateBookingHandler(newFakeBookingRepo(), newFakeIdempotency(), inlineTxManager{})

	_, err := h.Handle(context.Background(), validCmd(""))
	if !errors.Is(err, domain.ErrInvalidArgument) {
		t.Fatalf("want ErrInvalidArgument, got %v", err)
	}
}

func TestCreateBooking_RepeatedKeyReturnsSameBooking(t *testing.T) {
	t.Parallel()

	repo := newFakeBookingRepo()
	idem := newFakeIdempotency()
	h := NewCreateBookingHandler(repo, idem, inlineTxManager{})

	first, err := h.Handle(context.Background(), validCmd("dup-key"))
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	second, err := h.Handle(context.Background(), validCmd("dup-key"))
	if err != nil {
		t.Fatalf("second create: %v", err)
	}

	if first.ID() != second.ID() {
		t.Fatalf("expected same booking id; first=%s second=%s", first.ID(), second.ID())
	}
	if repo.saves != 1 {
		t.Fatalf("repo.Save must be called once; got %d", repo.saves)
	}
}

func TestCreateBooking_ReturnsExistingOnPriorLookupHit(t *testing.T) {
	t.Parallel()

	repo := newFakeBookingRepo()
	idem := newFakeIdempotency()
	winnerID := "11111111-1111-1111-1111-111111111111"
	winner := reconstructWinner(t, winnerID)
	repo.stored[winnerID] = winner
	idem.entries[hashIdempotencyKey("race-key")] = winnerID

	h := NewCreateBookingHandler(repo, idem, inlineTxManager{})

	got, err := h.Handle(context.Background(), validCmd("race-key"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID() != winnerID {
		t.Fatalf("expected preexisting booking %s; got %s", winnerID, got.ID())
	}
	if repo.saves != 0 {
		t.Fatalf("repo.Save must not be called on cache hit; got %d", repo.saves)
	}
}

func TestCreateBooking_ConflictDuringSaveResolvesToWinner(t *testing.T) {
	t.Parallel()

	repo := newFakeBookingRepo()
	idem := newFakeIdempotency()
	winnerID := "22222222-2222-2222-2222-222222222222"
	winner := reconstructWinner(t, winnerID)
	repo.stored[winnerID] = winner

	idem.onSaveConflict[hashIdempotencyKey("race-key")] = winnerID

	h := NewCreateBookingHandler(repo, idem, inlineTxManager{})

	got, err := h.Handle(context.Background(), validCmd("race-key"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID() != winnerID {
		t.Fatalf("expected winner %s after conflict; got %s", winnerID, got.ID())
	}
}

func TestCreateBooking_RejectsInvalidStay(t *testing.T) {
	t.Parallel()

	h := NewCreateBookingHandler(newFakeBookingRepo(), newFakeIdempotency(), inlineTxManager{})

	cmd := validCmd("k")
	cmd.CheckOut = cmd.CheckIn

	_, err := h.Handle(context.Background(), cmd)
	if !errors.Is(err, domain.ErrInvalidArgument) {
		t.Fatalf("want ErrInvalidArgument, got %v", err)
	}
}

func makeTestOffer(t *testing.T) booking.OfferSnapshot {
	t.Helper()
	price, err := booking.NewMoney(10000, "RUB")
	if err != nil {
		t.Fatalf("price: %v", err)
	}
	offer, err := booking.NewOfferSnapshot("offer-x", "hotel-x", "STANDARD", price)
	if err != nil {
		t.Fatalf("offer: %v", err)
	}
	return offer
}

func makeTestStay(t *testing.T) booking.StayPeriod {
	t.Helper()
	in := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	out := time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC)
	stay, err := booking.NewStayPeriod(in, out)
	if err != nil {
		t.Fatalf("stay: %v", err)
	}
	return stay
}

func reconstructWinner(t *testing.T, id string) *booking.Booking {
	t.Helper()
	total, err := booking.NewMoney(30000, "RUB")
	if err != nil {
		t.Fatalf("total: %v", err)
	}
	now := time.Now()
	return booking.Reconstruct(
		id, "guest-x",
		makeTestOffer(t), makeTestStay(t), total,
		booking.StatusPending, 1, now, now,
	)
}
