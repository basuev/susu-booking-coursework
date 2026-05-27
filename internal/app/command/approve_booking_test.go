package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"gitverse.ru/basuev/susu-booking-coursework/internal/domain"
	"gitverse.ru/basuev/susu-booking-coursework/internal/domain/booking"
)

func seedCancelledBooking(t *testing.T, repo *fakeBookingRepo, id string) *booking.Booking {
	t.Helper()
	total, err := booking.NewMoney(30000, "RUB")
	if err != nil {
		t.Fatalf("total: %v", err)
	}
	now := time.Now()
	b := booking.Reconstruct(
		id, "guest-x",
		makeTestOffer(t), makeTestStay(t), total,
		booking.StatusCancelled, 2, now, now,
	)
	repo.stored[id] = b
	return b
}

func TestApproveBooking_NotFound(t *testing.T) {
	t.Parallel()
	repo := newFakeBookingRepo()
	h := NewApproveBookingHandler(repo)

	_, err := h.Handle(context.Background(), ApproveBooking{BookingID: "x"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestApproveBooking_InvalidTransition(t *testing.T) {
	t.Parallel()
	repo := newFakeBookingRepo()
	seedCancelledBooking(t, repo, "id-1")
	h := NewApproveBookingHandler(repo)

	_, err := h.Handle(context.Background(), ApproveBooking{BookingID: "id-1"})
	if !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("want ErrInvalidTransition, got %v", err)
	}
	if repo.saves != 0 {
		t.Fatalf("repo.Save must not be called; got %d", repo.saves)
	}
}

func TestApproveBooking_Success(t *testing.T) {
	t.Parallel()
	repo := newFakeBookingRepo()
	seedPendingBooking(t, repo, "id-1")
	h := NewApproveBookingHandler(repo)

	b, err := h.Handle(context.Background(), ApproveBooking{BookingID: "id-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.Status() != booking.StatusApproved {
		t.Fatalf("want APPROVED, got %s", b.Status())
	}
	if repo.saves != 1 {
		t.Fatalf("repo.Save calls: want 1, got %d", repo.saves)
	}
}
