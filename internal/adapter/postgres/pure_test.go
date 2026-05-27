package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"gitverse.ru/basuev/susu-booking-coursework/internal/domain/booking"
)

func TestStatusTransitionFromEvent(t *testing.T) {
	t.Parallel()
	now := time.Now()

	old, newSt, reason, ok := statusTransitionFromEvent(booking.BookingCancelled{
		OldStatus: booking.StatusPending, NewStatus: booking.StatusCancelled, Timestamp: now,
	})
	if !ok || old != booking.StatusPending || newSt != booking.StatusCancelled || reason != "" {
		t.Fatalf("cancelled: %v %v %q %v", old, newSt, reason, ok)
	}

	old, newSt, reason, ok = statusTransitionFromEvent(booking.BookingApproved{
		OldStatus: booking.StatusPending, NewStatus: booking.StatusApproved, Timestamp: now,
	})
	if !ok || newSt != booking.StatusApproved || reason != "" {
		t.Fatalf("approved: %v %v %q %v", old, newSt, reason, ok)
	}

	old, newSt, reason, ok = statusTransitionFromEvent(booking.BookingRejected{
		OldStatus: booking.StatusPending, NewStatus: booking.StatusRejected, Reason: "x", Timestamp: now,
	})
	if !ok || reason != "x" {
		t.Fatalf("rejected: %v %v %q %v", old, newSt, reason, ok)
	}

	_, _, _, ok = statusTransitionFromEvent(booking.BookingCreated{Timestamp: now})
	if ok {
		t.Fatalf("created must not yield a transition")
	}
}

func TestNullableString(t *testing.T) {
	t.Parallel()
	if got := nullableString(""); got != nil {
		t.Errorf("empty must be nil, got %v", got)
	}
	if got := nullableString("x"); got != "x" {
		t.Errorf("non-empty must round-trip, got %v", got)
	}
}

func TestIsUniqueViolation(t *testing.T) {
	t.Parallel()
	if IsUniqueViolation(errors.New("plain")) {
		t.Fatalf("plain error must not be unique violation")
	}
	if IsUniqueViolation(nil) {
		t.Fatalf("nil must not be unique violation")
	}
	pgErr := &pgconn.PgError{Code: pgUniqueViolation}
	if !IsUniqueViolation(pgErr) {
		t.Fatalf("23505 must be flagged unique violation")
	}
	other := &pgconn.PgError{Code: "42P01"}
	if IsUniqueViolation(other) {
		t.Fatalf("non-23505 must not be unique violation")
	}
}

func TestTxFromContext_Empty(t *testing.T) {
	t.Parallel()
	if _, ok := txFromContext(context.Background()); ok {
		t.Fatalf("must not find tx in fresh ctx")
	}
}

func TestConstructors(t *testing.T) {
	t.Parallel()
	if NewBookingRepo(nil) == nil {
		t.Fatalf("NewBookingRepo returned nil")
	}
	if NewIdempotencyRepo(nil) == nil {
		t.Fatalf("NewIdempotencyRepo returned nil")
	}
	if NewProjectionRepo(nil) == nil {
		t.Fatalf("NewProjectionRepo returned nil")
	}
	if NewTxManager(nil) == nil {
		t.Fatalf("NewTxManager returned nil")
	}
}
