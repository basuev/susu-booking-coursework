//go:build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"gitverse.ru/basuev/susu-booking-coursework/internal/app/query"
	"gitverse.ru/basuev/susu-booking-coursework/internal/domain/booking"
	"gitverse.ru/basuev/susu-booking-coursework/pkg/id"
)

func makeProjection(guest string) BookingProjection {
	now := time.Now().UTC()
	return BookingProjection{
		ID:          id.New(),
		GuestID:     guest,
		HotelID:     id.New(),
		RoomType:    "STANDARD",
		CheckIn:     time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		CheckOut:    time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC),
		TotalAmount: 30000,
		Currency:    "RUB",
		Status:      string(booking.StatusPending),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func TestProjectionRepo_UpsertInsert(t *testing.T) {
	db := newTestDB(t)
	repo := NewProjectionRepo(db)
	ctx := context.Background()
	guest := id.New()

	p := makeProjection(guest)
	if err := repo.Upsert(ctx, p); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	items, err := repo.ListByGuest(ctx, query.ProjectionFilter{GuestID: guest, Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 1 || items[0].ID != p.ID {
		t.Fatalf("expected one item, got %d", len(items))
	}
}

func TestProjectionRepo_UpsertUpdate(t *testing.T) {
	db := newTestDB(t)
	repo := NewProjectionRepo(db)
	ctx := context.Background()
	guest := id.New()

	p := makeProjection(guest)
	if err := repo.Upsert(ctx, p); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	p.Status = string(booking.StatusApproved)
	p.UpdatedAt = p.UpdatedAt.Add(time.Hour)
	if err := repo.Upsert(ctx, p); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	items, err := repo.ListByGuest(ctx, query.ProjectionFilter{GuestID: guest, Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one item, got %d", len(items))
	}
	if items[0].Status != booking.StatusApproved {
		t.Fatalf("status: want APPROVED, got %s", items[0].Status)
	}
}

func TestProjectionRepo_UpdateStatus(t *testing.T) {
	db := newTestDB(t)
	repo := NewProjectionRepo(db)
	ctx := context.Background()
	guest := id.New()

	p := makeProjection(guest)
	if err := repo.Upsert(ctx, p); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	newTime := p.UpdatedAt.Add(time.Hour)
	if err := repo.UpdateStatus(ctx, p.ID, string(booking.StatusCancelled), newTime); err != nil {
		t.Fatalf("update status: %v", err)
	}
	items, _ := repo.ListByGuest(ctx, query.ProjectionFilter{GuestID: guest, Limit: 10})
	if items[0].Status != booking.StatusCancelled {
		t.Fatalf("want CANCELLED, got %s", items[0].Status)
	}
}

func TestProjectionRepo_ListByGuest_Filters(t *testing.T) {
	db := newTestDB(t)
	repo := NewProjectionRepo(db)
	ctx := context.Background()
	guest := id.New()

	approved := makeProjection(guest)
	approved.Status = string(booking.StatusApproved)
	approved.CheckIn = time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)
	if err := repo.Upsert(ctx, approved); err != nil {
		t.Fatalf("upsert approved: %v", err)
	}

	pending := makeProjection(guest)
	pending.Status = string(booking.StatusPending)
	pending.CheckIn = time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	if err := repo.Upsert(ctx, pending); err != nil {
		t.Fatalf("upsert pending: %v", err)
	}

	other := makeProjection(id.New())
	if err := repo.Upsert(ctx, other); err != nil {
		t.Fatalf("upsert other guest: %v", err)
	}

	st := booking.StatusApproved
	items, err := repo.ListByGuest(ctx, query.ProjectionFilter{
		GuestID: guest, Status: &st, Limit: 10,
	})
	if err != nil {
		t.Fatalf("list by status: %v", err)
	}
	if len(items) != 1 || items[0].Status != booking.StatusApproved {
		t.Fatalf("status filter broken: %#v", items)
	}

	from := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	items2, err := repo.ListByGuest(ctx, query.ProjectionFilter{
		GuestID: guest, CheckInFrom: &from, Limit: 10,
	})
	if err != nil {
		t.Fatalf("list by from: %v", err)
	}
	if len(items2) != 1 || items2[0].ID != pending.ID {
		t.Fatalf("date filter broken: %#v", items2)
	}

	to := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	items3, err := repo.ListByGuest(ctx, query.ProjectionFilter{
		GuestID: guest, CheckInTo: &to, Limit: 10,
	})
	if err != nil {
		t.Fatalf("list by to: %v", err)
	}
	if len(items3) != 1 || items3[0].ID != approved.ID {
		t.Fatalf("upper bound filter broken: %#v", items3)
	}
}

func TestProjectionRepo_ListByGuest_SortByUpdatedAtDesc(t *testing.T) {
	db := newTestDB(t)
	repo := NewProjectionRepo(db)
	ctx := context.Background()
	guest := id.New()

	older := makeProjection(guest)
	older.UpdatedAt = time.Now().UTC().Add(-2 * time.Hour)
	older.CreatedAt = older.UpdatedAt
	if err := repo.Upsert(ctx, older); err != nil {
		t.Fatalf("upsert older: %v", err)
	}

	newer := makeProjection(guest)
	newer.UpdatedAt = time.Now().UTC()
	newer.CreatedAt = newer.UpdatedAt
	if err := repo.Upsert(ctx, newer); err != nil {
		t.Fatalf("upsert newer: %v", err)
	}

	items, err := repo.ListByGuest(ctx, query.ProjectionFilter{GuestID: guest, Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 items, got %d", len(items))
	}
	if items[0].ID != newer.ID {
		t.Fatalf("sort wrong: first must be newer; got %s", items[0].ID)
	}
}
