package outbox

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/basuev/susu-booking-coursework/internal/domain/booking"
)

type Message struct {
	Topic   string
	Key     string
	Payload []byte
}

type moneyPayload struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

type bookingCreatedPayload struct {
	BookingID string       `json:"booking_id"`
	GuestID   string       `json:"guest_id"`
	HotelID   string       `json:"hotel_id"`
	CheckIn   time.Time    `json:"check_in"`
	CheckOut  time.Time    `json:"check_out"`
	Total     moneyPayload `json:"total"`
	Timestamp time.Time    `json:"timestamp"`
}

type bookingCancelledPayload struct {
	BookingID string    `json:"booking_id"`
	OldStatus string    `json:"old_status"`
	NewStatus string    `json:"new_status"`
	Timestamp time.Time `json:"timestamp"`
}

type bookingApprovedPayload struct {
	BookingID string    `json:"booking_id"`
	OldStatus string    `json:"old_status"`
	NewStatus string    `json:"new_status"`
	Timestamp time.Time `json:"timestamp"`
}

type bookingRejectedPayload struct {
	BookingID string    `json:"booking_id"`
	OldStatus string    `json:"old_status"`
	NewStatus string    `json:"new_status"`
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`
}

func FromDomainEvent(e booking.Event) (Message, error) {
	switch ev := e.(type) {
	case booking.BookingCreated:
		payload, err := json.Marshal(bookingCreatedPayload{
			BookingID: ev.BookingID,
			GuestID:   ev.GuestID,
			HotelID:   ev.HotelID,
			CheckIn:   ev.CheckIn,
			CheckOut:  ev.CheckOut,
			Total:     moneyPayload{Amount: ev.Total.Amount(), Currency: ev.Total.Currency()},
			Timestamp: ev.Timestamp,
		})
		if err != nil {
			return Message{}, fmt.Errorf("marshal BookingCreated: %w", err)
		}
		return Message{Topic: e.EventName(), Key: ev.BookingID, Payload: payload}, nil
	case booking.BookingCancelled:
		payload, err := json.Marshal(bookingCancelledPayload{
			BookingID: ev.BookingID,
			OldStatus: string(ev.OldStatus),
			NewStatus: string(ev.NewStatus),
			Timestamp: ev.Timestamp,
		})
		if err != nil {
			return Message{}, fmt.Errorf("marshal BookingCancelled: %w", err)
		}
		return Message{Topic: e.EventName(), Key: ev.BookingID, Payload: payload}, nil
	case booking.BookingApproved:
		payload, err := json.Marshal(bookingApprovedPayload{
			BookingID: ev.BookingID,
			OldStatus: string(ev.OldStatus),
			NewStatus: string(ev.NewStatus),
			Timestamp: ev.Timestamp,
		})
		if err != nil {
			return Message{}, fmt.Errorf("marshal BookingApproved: %w", err)
		}
		return Message{Topic: e.EventName(), Key: ev.BookingID, Payload: payload}, nil
	case booking.BookingRejected:
		payload, err := json.Marshal(bookingRejectedPayload{
			BookingID: ev.BookingID,
			OldStatus: string(ev.OldStatus),
			NewStatus: string(ev.NewStatus),
			Reason:    ev.Reason,
			Timestamp: ev.Timestamp,
		})
		if err != nil {
			return Message{}, fmt.Errorf("marshal BookingRejected: %w", err)
		}
		return Message{Topic: e.EventName(), Key: ev.BookingID, Payload: payload}, nil
	default:
		return Message{}, fmt.Errorf("unsupported event type: %T", e)
	}
}
