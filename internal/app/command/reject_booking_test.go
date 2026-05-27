package command

import (
	"context"
	"errors"
	"testing"

	"gitverse.ru/basuev/susu-booking-coursework/internal/domain"
	"gitverse.ru/basuev/susu-booking-coursework/internal/domain/booking"
)

func TestRejectBooking_NotFound(t *testing.T) {
	t.Parallel()
	repo := newFakeBookingRepo()
	h := NewRejectBookingHandler(repo)

	_, err := h.Handle(context.Background(), RejectBooking{BookingID: "x", Reason: "no rooms"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestRejectBooking_InvalidTransition(t *testing.T) {
	t.Parallel()
	repo := newFakeBookingRepo()
	seedCancelledBooking(t, repo, "id-1")
	h := NewRejectBookingHandler(repo)

	_, err := h.Handle(context.Background(), RejectBooking{BookingID: "id-1", Reason: "x"})
	if !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("want ErrInvalidTransition, got %v", err)
	}
	if repo.saves != 0 {
		t.Fatalf("repo.Save must not be called; got %d", repo.saves)
	}
}

func TestRejectBooking_Success(t *testing.T) {
	t.Parallel()
	repo := newFakeBookingRepo()
	seedPendingBooking(t, repo, "id-1")
	h := NewRejectBookingHandler(repo)

	b, err := h.Handle(context.Background(), RejectBooking{BookingID: "id-1", Reason: "overbooked"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.Status() != booking.StatusRejected {
		t.Fatalf("want REJECTED, got %s", b.Status())
	}
	if repo.saves != 1 {
		t.Fatalf("repo.Save calls: want 1, got %d", repo.saves)
	}
}
