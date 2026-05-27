package projector

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDecode_BookingCreated(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	payload := map[string]any{
		"booking_id": "b-1",
		"guest_id":   "g-1",
		"hotel_id":   "h-1",
		"room_type":  "standard",
		"check_in":   ts,
		"check_out":  ts.AddDate(0, 0, 2),
		"total":      map[string]any{"amount": 19999, "currency": "RUB"},
		"timestamp":  ts,
	}
	raw, _ := json.Marshal(payload)

	op, err := Decode("booking.created", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if op.Kind != OpInsert {
		t.Fatalf("kind: want OpInsert, got %d", op.Kind)
	}
	if op.Insert.ID != "b-1" || op.Insert.GuestID != "g-1" || op.Insert.HotelID != "h-1" {
		t.Fatalf("identifiers mismatch: %+v", op.Insert)
	}
	if op.Insert.RoomType != "standard" {
		t.Fatalf("room_type: want standard, got %q", op.Insert.RoomType)
	}
	if op.Insert.TotalAmount != 19999 || op.Insert.Currency != "RUB" {
		t.Fatalf("money mismatch: %+v", op.Insert)
	}
	if op.Insert.Status != "PENDING" {
		t.Fatalf("status: want PENDING, got %q", op.Insert.Status)
	}
}

func TestDecode_StatusChanges(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		subject string
		new     string
	}{
		{"cancelled", "booking.cancelled", "CANCELLED"},
		{"approved", "booking.approved", "APPROVED"},
		{"rejected", "booking.rejected", "REJECTED"},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			payload, _ := json.Marshal(map[string]any{
				"booking_id": "b-1",
				"old_status": "PENDING",
				"new_status": c.new,
				"timestamp":  ts,
			})
			op, err := Decode(c.subject, payload)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if op.Kind != OpUpdateStatus {
				t.Fatalf("kind: want OpUpdateStatus, got %d", op.Kind)
			}
			if op.ID != "b-1" || op.Status != c.new {
				t.Fatalf("op mismatch: %+v", op)
			}
			if !op.UpdatedAt.Equal(ts) {
				t.Fatalf("timestamp: want %v, got %v", ts, op.UpdatedAt)
			}
		})
	}
}

func TestDecode_UnknownSubject(t *testing.T) {
	t.Parallel()
	if _, err := Decode("booking.unknown", []byte("{}")); err == nil {
		t.Fatalf("expected error for unknown subject")
	}
}

func TestDecode_InvalidJSON(t *testing.T) {
	t.Parallel()
	if _, err := Decode("booking.created", []byte("not json")); err == nil {
		t.Fatalf("expected error for invalid JSON")
	}
}
