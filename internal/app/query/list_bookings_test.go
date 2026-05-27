package query

import (
	"context"
	"errors"
	"testing"
	"time"

	"gitverse.ru/basuev/susu-booking-coursework/internal/domain/booking"
)

type fakeProjectionReader struct {
	items     []ProjectionItem
	err       error
	lastInput ProjectionFilter
	called    int
}

func (r *fakeProjectionReader) ListByGuest(_ context.Context, f ProjectionFilter) ([]ProjectionItem, error) {
	r.called++
	r.lastInput = f
	if r.err != nil {
		return nil, r.err
	}
	return r.items, nil
}

func TestListBookings_Success_PropagatesFilters(t *testing.T) {
	t.Parallel()
	st := booking.StatusApproved
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)

	reader := &fakeProjectionReader{
		items: []ProjectionItem{
			{ID: "b1", GuestID: "g1", Status: booking.StatusApproved},
			{ID: "b2", GuestID: "g1", Status: booking.StatusApproved},
		},
	}
	h := NewListBookingsHandler(reader)

	got, err := h.Handle(context.Background(), ListBookings{
		GuestID:     "g1",
		Status:      &st,
		CheckInFrom: &from,
		CheckInTo:   &to,
		PageSize:    50,
		Offset:      10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 items, got %d", len(got))
	}
	if reader.lastInput.GuestID != "g1" {
		t.Fatalf("want guest g1, got %s", reader.lastInput.GuestID)
	}
	if reader.lastInput.Status == nil || *reader.lastInput.Status != booking.StatusApproved {
		t.Fatalf("status filter not propagated: %#v", reader.lastInput.Status)
	}
	if reader.lastInput.CheckInFrom == nil || !reader.lastInput.CheckInFrom.Equal(from) {
		t.Fatalf("check_in_from not propagated: %#v", reader.lastInput.CheckInFrom)
	}
	if reader.lastInput.CheckInTo == nil || !reader.lastInput.CheckInTo.Equal(to) {
		t.Fatalf("check_in_to not propagated: %#v", reader.lastInput.CheckInTo)
	}
	if reader.lastInput.Limit != 50 || reader.lastInput.Offset != 10 {
		t.Fatalf("pagination not propagated: limit=%d offset=%d", reader.lastInput.Limit, reader.lastInput.Offset)
	}
}

func TestListBookings_ReaderErrorPropagates(t *testing.T) {
	t.Parallel()
	boom := errors.New("reader boom")
	h := NewListBookingsHandler(&fakeProjectionReader{err: boom})

	_, err := h.Handle(context.Background(), ListBookings{GuestID: "g1", PageSize: 20})
	if !errors.Is(err, boom) {
		t.Fatalf("want %v, got %v", boom, err)
	}
}

func TestListBookings_OptionalFiltersNil(t *testing.T) {
	t.Parallel()
	reader := &fakeProjectionReader{}
	h := NewListBookingsHandler(reader)

	if _, err := h.Handle(context.Background(), ListBookings{GuestID: "g1", PageSize: 20}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reader.lastInput.Status != nil || reader.lastInput.CheckInFrom != nil || reader.lastInput.CheckInTo != nil {
		t.Fatalf("optional filters should be nil, got %#v", reader.lastInput)
	}
}
