package outbox

import (
	"encoding/json"
	"testing"
	"time"

	"gitverse.ru/basuev/susu-booking-coursework/internal/domain/booking"
)

func TestFromDomainEvent_BookingCreated(t *testing.T) {
	t.Parallel()

	total, _ := booking.NewMoney(30000, "RUB")
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	ev := booking.BookingCreated{
		BookingID: "b-1",
		GuestID:   "g-1",
		HotelID:   "h-1",
		CheckIn:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		CheckOut:  time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC),
		Total:     total,
		Timestamp: now,
	}

	msg, err := FromDomainEvent(ev)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Topic != "booking.created" {
		t.Fatalf("topic: want booking.created, got %s", msg.Topic)
	}
	if msg.Key != "b-1" {
		t.Fatalf("key: want b-1, got %s", msg.Key)
	}

	var payload map[string]any
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		t.Fatalf("payload not JSON: %v", err)
	}
	if payload["booking_id"] != "b-1" || payload["guest_id"] != "g-1" || payload["hotel_id"] != "h-1" {
		t.Fatalf("payload missing identifiers: %v", payload)
	}
	totalMap, ok := payload["total"].(map[string]any)
	if !ok {
		t.Fatalf("payload.total is not object: %T", payload["total"])
	}
	if totalMap["currency"] != "RUB" {
		t.Fatalf("currency: %v", totalMap["currency"])
	}
}

func TestFromDomainEvent_AllEventTypes(t *testing.T) {
	t.Parallel()

	now := time.Now()
	cases := []struct {
		name      string
		event     booking.Event
		wantTopic string
	}{
		{"cancelled", booking.BookingCancelled{BookingID: "b-1", Timestamp: now}, "booking.cancelled"},
		{"approved", booking.BookingApproved{BookingID: "b-1", Timestamp: now}, "booking.approved"},
		{"rejected", booking.BookingRejected{BookingID: "b-1", Reason: "x", Timestamp: now}, "booking.rejected"},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			msg, err := FromDomainEvent(c.event)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if msg.Topic != c.wantTopic {
				t.Fatalf("topic: want %s, got %s", c.wantTopic, msg.Topic)
			}
			if msg.Key != "b-1" {
				t.Fatalf("key: want b-1, got %s", msg.Key)
			}
			if len(msg.Payload) == 0 {
				t.Fatalf("empty payload")
			}
		})
	}
}

type unknownEvent struct{}

func (unknownEvent) EventName() string     { return "unknown" }
func (unknownEvent) OccurredAt() time.Time { return time.Now() }

func TestFromDomainEvent_UnknownTypeReturnsError(t *testing.T) {
	t.Parallel()
	if _, err := FromDomainEvent(unknownEvent{}); err == nil {
		t.Fatalf("expected error for unknown event type")
	}
}

func TestFromDomainEvent_RejectedReasonPreserved(t *testing.T) {
	t.Parallel()

	msg, err := FromDomainEvent(booking.BookingRejected{
		BookingID: "b-1",
		Reason:    "rooms unavailable",
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		t.Fatalf("payload not JSON: %v", err)
	}
	if payload["reason"] != "rooms unavailable" {
		t.Fatalf("reason: %v", payload["reason"])
	}
}
