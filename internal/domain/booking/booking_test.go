package booking

import (
	"errors"
	"testing"
	"time"

	"gitverse.ru/basuev/susu-booking-coursework/internal/domain"
)

func newTestOffer(t *testing.T) OfferSnapshot {
	t.Helper()
	price, err := NewMoney(10000, "RUB")
	if err != nil {
		t.Fatalf("setup money: %v", err)
	}
	offer, err := NewOfferSnapshot("offer-1", "hotel-1", "STANDARD", price)
	if err != nil {
		t.Fatalf("setup offer: %v", err)
	}
	return offer
}

func newTestStay(t *testing.T) StayPeriod {
	t.Helper()
	in := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	out := time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC)
	stay, err := NewStayPeriod(in, out)
	if err != nil {
		t.Fatalf("setup stay: %v", err)
	}
	return stay
}

func TestNewBooking(t *testing.T) {
	t.Parallel()

	offer := newTestOffer(t)
	stay := newTestStay(t)

	b, err := NewBooking("guest-1", offer, stay)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.ID() == "" {
		t.Fatalf("ID must be generated")
	}
	if b.Status() != StatusPending {
		t.Fatalf("new booking must be PENDING, got %s", b.Status())
	}
	if b.Total().Amount() != 30000 {
		t.Fatalf("total must equal 3 nights * 10000, got %d", b.Total().Amount())
	}
	if got := len(b.Events()); got != 1 {
		t.Fatalf("expected one BookingCreated event, got %d", got)
	}
	if _, ok := b.Events()[0].(BookingCreated); !ok {
		t.Fatalf("expected BookingCreated, got %T", b.Events()[0])
	}
}

func TestNewBooking_RejectsEmptyGuest(t *testing.T) {
	t.Parallel()
	_, err := NewBooking("", newTestOffer(t), newTestStay(t))
	if !errors.Is(err, domain.ErrInvalidArgument) {
		t.Fatalf("want ErrInvalidArgument, got %v", err)
	}
}

func TestBooking_Lifecycle(t *testing.T) {
	t.Parallel()

	t.Run("approve from pending records event with transition", func(t *testing.T) {
		t.Parallel()
		b, _ := NewBooking("guest-1", newTestOffer(t), newTestStay(t))
		b.ClearEvents()
		if err := b.Approve(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if b.Status() != StatusApproved {
			t.Fatalf("status: want APPROVED, got %s", b.Status())
		}
		if got := len(b.Events()); got != 1 {
			t.Fatalf("expected one BookingApproved event, got %d", got)
		}
		evt := b.Events()[0].(BookingApproved)
		if evt.OldStatus != StatusPending || evt.NewStatus != StatusApproved {
			t.Fatalf("transition: want PENDING->APPROVED, got %s->%s", evt.OldStatus, evt.NewStatus)
		}
	})

	t.Run("cancel from pending records event", func(t *testing.T) {
		t.Parallel()
		b, _ := NewBooking("guest-1", newTestOffer(t), newTestStay(t))
		b.ClearEvents()
		if err := b.Cancel(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if b.Status() != StatusCancelled {
			t.Fatalf("status: want CANCELLED, got %s", b.Status())
		}
	})

	t.Run("reject from pending stores reason", func(t *testing.T) {
		t.Parallel()
		b, _ := NewBooking("guest-1", newTestOffer(t), newTestStay(t))
		b.ClearEvents()
		if err := b.Reject("rooms unavailable"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if b.Status() != StatusRejected {
			t.Fatalf("status: want REJECTED, got %s", b.Status())
		}
		evt, ok := b.Events()[0].(BookingRejected)
		if !ok {
			t.Fatalf("expected BookingRejected, got %T", b.Events()[0])
		}
		if evt.Reason != "rooms unavailable" {
			t.Fatalf("reason: want %q, got %q", "rooms unavailable", evt.Reason)
		}
		if evt.OldStatus != StatusPending || evt.NewStatus != StatusRejected {
			t.Fatalf("transition: want PENDING->REJECTED, got %s->%s", evt.OldStatus, evt.NewStatus)
		}
	})

	t.Run("approve from approved fails", func(t *testing.T) {
		t.Parallel()
		b, _ := NewBooking("guest-1", newTestOffer(t), newTestStay(t))
		if err := b.Approve(); err != nil {
			t.Fatalf("first approve: %v", err)
		}
		if err := b.Approve(); !errors.Is(err, domain.ErrInvalidTransition) {
			t.Fatalf("second approve: want ErrInvalidTransition, got %v", err)
		}
	})

	t.Run("approve from pending", func(t *testing.T) {
		t.Parallel()
		b, _ := NewBooking("guest-1", newTestOffer(t), newTestStay(t))
		if err := b.Approve(); err != nil {
			t.Fatalf("approve: %v", err)
		}
		if b.Status() != StatusApproved {
			t.Fatalf("status after Approve: %s", b.Status())
		}
	})
}

func TestBooking_Reconstruct(t *testing.T) {
	t.Parallel()

	offer := newTestOffer(t)
	stay := newTestStay(t)
	total, _ := NewMoney(30000, "RUB")
	now := time.Now()

	b := Reconstruct("id-1", "guest-1", offer, stay, total, StatusApproved, 4, now, now)
	if b.Status() != StatusApproved {
		t.Fatalf("status: %s", b.Status())
	}
	if b.Version() != 4 {
		t.Fatalf("version: want 4, got %d", b.Version())
	}
	if got := len(b.Events()); got != 0 {
		t.Fatalf("Reconstruct must not emit events, got %d", got)
	}
}

func TestBooking_VersionIncrementsOnTransition(t *testing.T) {
	t.Parallel()

	b, err := NewBooking("guest-1", newTestOffer(t), newTestStay(t))
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if b.Version() != 1 {
		t.Fatalf("initial version: want 1, got %d", b.Version())
	}

	if err := b.Approve(); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if b.Version() != 2 {
		t.Fatalf("after approve: want 2, got %d", b.Version())
	}
}

func TestBooking_VersionStaysOnFailedTransition(t *testing.T) {
	t.Parallel()

	b, _ := NewBooking("guest-1", newTestOffer(t), newTestStay(t))
	_ = b.Approve()
	versionAfterApprove := b.Version()

	if err := b.Cancel(); err == nil {
		t.Fatalf("cancel after approve must fail")
	}
	if b.Version() != versionAfterApprove {
		t.Fatalf("version must not advance on failed transition: want %d, got %d",
			versionAfterApprove, b.Version())
	}
}
