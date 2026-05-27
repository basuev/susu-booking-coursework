package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"gitverse.ru/basuev/susu-booking-coursework/internal/domain"
	"gitverse.ru/basuev/susu-booking-coursework/internal/domain/booking"
)

func seedPendingBooking(t *testing.T, repo *fakeBookingRepo, id string) *booking.Booking {
	t.Helper()
	total, err := booking.NewMoney(30000, "RUB")
	if err != nil {
		t.Fatalf("total: %v", err)
	}
	now := time.Now()
	b := booking.Reconstruct(
		id, "guest-x",
		makeTestOffer(t), makeTestStay(t), total,
		booking.StatusPending, 1, now, now,
	)
	repo.stored[id] = b
	return b
}

func seedApprovedBooking(t *testing.T, repo *fakeBookingRepo, id string) *booking.Booking {
	t.Helper()
	total, err := booking.NewMoney(30000, "RUB")
	if err != nil {
		t.Fatalf("total: %v", err)
	}
	now := time.Now()
	b := booking.Reconstruct(
		id, "guest-x",
		makeTestOffer(t), makeTestStay(t), total,
		booking.StatusApproved, 2, now, now,
	)
	repo.stored[id] = b
	return b
}

func TestCancelBooking_NotFound(t *testing.T) {
	t.Parallel()
	repo := newFakeBookingRepo()
	h := NewCancelBookingHandler(repo)

	_, err := h.Handle(context.Background(), CancelBooking{BookingID: "missing"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestCancelBooking_InvalidTransition(t *testing.T) {
	t.Parallel()
	repo := newFakeBookingRepo()
	seedApprovedBooking(t, repo, "id-1")
	h := NewCancelBookingHandler(repo)

	_, err := h.Handle(context.Background(), CancelBooking{BookingID: "id-1"})
	if !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("want ErrInvalidTransition, got %v", err)
	}
	if repo.saves != 0 {
		t.Fatalf("repo.Save must not be called on invalid transition; got %d", repo.saves)
	}
}

func TestCancelBooking_Success(t *testing.T) {
	t.Parallel()
	repo := newFakeBookingRepo()
	seedPendingBooking(t, repo, "id-1")
	h := NewCancelBookingHandler(repo)

	b, err := h.Handle(context.Background(), CancelBooking{BookingID: "id-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.Status() != booking.StatusCancelled {
		t.Fatalf("want CANCELLED, got %s", b.Status())
	}
	if repo.saves != 1 {
		t.Fatalf("repo.Save calls: want 1, got %d", repo.saves)
	}
}
