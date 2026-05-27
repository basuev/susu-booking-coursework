//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"

	"gitverse.ru/basuev/susu-booking-coursework/internal/app/command"
	"gitverse.ru/basuev/susu-booking-coursework/pkg/id"
)

func TestIdempotencyRepo_LookupMiss(t *testing.T) {
	db := newTestDB(t)
	repo := NewIdempotencyRepo(db)

	_, found, err := repo.Lookup(context.Background(), "deadbeef")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if found {
		t.Fatalf("expected miss")
	}
}

func TestIdempotencyRepo_SaveAndLookup(t *testing.T) {
	db := newTestDB(t)
	repo := NewIdempotencyRepo(db)
	bookingRepo := NewBookingRepo(db)
	ctx := context.Background()

	b := makeBooking(t)
	if err := bookingRepo.Save(ctx, b); err != nil {
		t.Fatalf("seed booking: %v", err)
	}

	hash := "hash-1"
	if err := repo.Save(ctx, hash, b.ID()); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, found, err := repo.Lookup(ctx, hash)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if !found || got != b.ID() {
		t.Fatalf("want %s, got %s found=%v", b.ID(), got, found)
	}
}

func TestIdempotencyRepo_UniqueConflict(t *testing.T) {
	db := newTestDB(t)
	repo := NewIdempotencyRepo(db)
	bookingRepo := NewBookingRepo(db)
	ctx := context.Background()

	b := makeBooking(t)
	if err := bookingRepo.Save(ctx, b); err != nil {
		t.Fatalf("seed booking: %v", err)
	}

	hash := "shared-hash"
	if err := repo.Save(ctx, hash, b.ID()); err != nil {
		t.Fatalf("first save: %v", err)
	}
	err := repo.Save(ctx, hash, id.New())
	if !errors.Is(err, command.ErrIdempotencyConflict) {
		t.Fatalf("want ErrIdempotencyConflict, got %v", err)
	}
}
