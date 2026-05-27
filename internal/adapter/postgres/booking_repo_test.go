//go:build integration

package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"gitverse.ru/basuev/susu-booking-coursework/internal/domain"
	"gitverse.ru/basuev/susu-booking-coursework/internal/domain/booking"
	"gitverse.ru/basuev/susu-booking-coursework/pkg/id"
)

func makeBooking(t *testing.T) *booking.Booking {
	t.Helper()
	price, err := booking.NewMoney(10000, "RUB")
	if err != nil {
		t.Fatalf("price: %v", err)
	}
	offer, err := booking.NewOfferSnapshot(id.New(), id.New(), "STANDARD", price)
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
	b, err := booking.NewBooking(id.New(), offer, stay)
	if err != nil {
		t.Fatalf("booking: %v", err)
	}
	return b
}

func TestBookingRepo_SaveThenFindByID(t *testing.T) {
	db := newTestDB(t)
	repo := NewBookingRepo(db)
	ctx := context.Background()

	b := makeBooking(t)
	if err := repo.Save(ctx, b); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := repo.FindByID(ctx, b.ID())
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.ID() != b.ID() {
		t.Fatalf("id mismatch")
	}
	if got.Status() != booking.StatusPending {
		t.Fatalf("status mismatch: %s", got.Status())
	}
	if got.Version() != 1 {
		t.Fatalf("version: want 1, got %d", got.Version())
	}
}

func TestBookingRepo_UpdateIncrementsVersion(t *testing.T) {
	db := newTestDB(t)
	repo := NewBookingRepo(db)
	ctx := context.Background()

	b := makeBooking(t)
	if err := repo.Save(ctx, b); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := b.Cancel(); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if err := repo.Save(ctx, b); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.FindByID(ctx, b.ID())
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.Version() != 2 {
		t.Fatalf("version: want 2, got %d", got.Version())
	}
	if got.Status() != booking.StatusCancelled {
		t.Fatalf("status: want CANCELLED, got %s", got.Status())
	}
}

func TestBookingRepo_ConcurrentSave_OneGetsConcurrentUpdate(t *testing.T) {
	db := newTestDB(t)
	repo := NewBookingRepo(db)
	ctx := context.Background()

	b := makeBooking(t)
	if err := repo.Save(ctx, b); err != nil {
		t.Fatalf("save: %v", err)
	}

	fetchA, err := repo.FindByID(ctx, b.ID())
	if err != nil {
		t.Fatalf("fetch a: %v", err)
	}
	fetchB, err := repo.FindByID(ctx, b.ID())
	if err != nil {
		t.Fatalf("fetch b: %v", err)
	}
	if err := fetchA.Cancel(); err != nil {
		t.Fatalf("cancel a: %v", err)
	}
	if err := fetchB.Approve(); err != nil {
		t.Fatalf("approve b: %v", err)
	}

	var (
		wg      sync.WaitGroup
		errA    error
		errB    error
		mu      sync.Mutex
		results []error
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		err := repo.Save(ctx, fetchA)
		mu.Lock()
		errA = err
		results = append(results, err)
		mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		err := repo.Save(ctx, fetchB)
		mu.Lock()
		errB = err
		results = append(results, err)
		mu.Unlock()
	}()
	wg.Wait()

	var winner, loser error
	if errA == nil && errB != nil {
		winner, loser = errA, errB
	} else if errB == nil && errA != nil {
		winner, loser = errB, errA
	} else {
		t.Fatalf("expected exactly one winner; got errA=%v errB=%v", errA, errB)
	}
	if winner != nil {
		t.Fatalf("unreachable: winner should be nil")
	}
	if !errors.Is(loser, domain.ErrConcurrentUpdate) {
		t.Fatalf("loser must be ErrConcurrentUpdate; got %v", loser)
	}
}

func TestBookingRepo_FindByID_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := NewBookingRepo(db)
	ctx := context.Background()

	_, err := repo.FindByID(ctx, "11111111-1111-1111-1111-111111111111")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestBookingRepo_StatusHistory_AppendedOnTransition(t *testing.T) {
	db := newTestDB(t)
	repo := NewBookingRepo(db)
	ctx := context.Background()

	b := makeBooking(t)
	if err := repo.Save(ctx, b); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := b.Approve(); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if err := repo.Save(ctx, b); err != nil {
		t.Fatalf("save approve: %v", err)
	}

	hist, err := repo.GetStatusHistory(ctx, b.ID())
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(hist) != 1 {
		t.Fatalf("want 1 entry, got %d", len(hist))
	}
	if hist[0].OldStatus != booking.StatusPending || hist[0].NewStatus != booking.StatusApproved {
		t.Fatalf("transition wrong: %+v", hist[0])
	}
}

func TestBookingRepo_OutboxRowCreatedOnSave(t *testing.T) {
	db := newTestDB(t)
	repo := NewBookingRepo(db)
	ctx := context.Background()

	b := makeBooking(t)
	if err := repo.Save(ctx, b); err != nil {
		t.Fatalf("save: %v", err)
	}

	var n int
	if err := db.QueryRow(`SELECT count(*) FROM outbox_message WHERE key = $1`, b.ID()).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("want 1 outbox row for booking, got %d", n)
	}
}
