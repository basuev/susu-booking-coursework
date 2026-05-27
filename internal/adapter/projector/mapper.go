package projector

import (
	"encoding/json"
	"fmt"
	"time"

	"gitverse.ru/basuev/susu-booking-coursework/internal/adapter/postgres"
	"gitverse.ru/basuev/susu-booking-coursework/internal/domain/booking"
)

const (
	subjectCreated   = "booking.created"
	subjectCancelled = "booking.cancelled"
	subjectApproved  = "booking.approved"
	subjectRejected  = "booking.rejected"
)

type Op struct {
	Kind      OpKind
	Insert    postgres.BookingProjection
	ID        string
	Status    string
	UpdatedAt time.Time
}

type OpKind int

const (
	OpUnknown OpKind = iota
	OpInsert
	OpUpdateStatus
)

type bookingCreatedPayload struct {
	BookingID string    `json:"booking_id"`
	GuestID   string    `json:"guest_id"`
	HotelID   string    `json:"hotel_id"`
	RoomType  string    `json:"room_type"`
	CheckIn   time.Time `json:"check_in"`
	CheckOut  time.Time `json:"check_out"`
	Total     struct {
		Amount   int64  `json:"amount"`
		Currency string `json:"currency"`
	} `json:"total"`
	Timestamp time.Time `json:"timestamp"`
}

type statusChangePayload struct {
	BookingID string    `json:"booking_id"`
	OldStatus string    `json:"old_status"`
	NewStatus string    `json:"new_status"`
	Timestamp time.Time `json:"timestamp"`
}

func Decode(subject string, payload []byte) (Op, error) {
	switch subject {
	case subjectCreated:
		var p bookingCreatedPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			return Op{}, fmt.Errorf("decode %s: %w", subject, err)
		}
		return Op{
			Kind: OpInsert,
			Insert: postgres.BookingProjection{
				ID:          p.BookingID,
				GuestID:     p.GuestID,
				HotelID:     p.HotelID,
				RoomType:    p.RoomType,
				CheckIn:     p.CheckIn,
				CheckOut:    p.CheckOut,
				TotalAmount: p.Total.Amount,
				Currency:    p.Total.Currency,
				Status:      string(booking.StatusPending),
				CreatedAt:   p.Timestamp,
				UpdatedAt:   p.Timestamp,
			},
		}, nil
	case subjectCancelled, subjectApproved, subjectRejected:
		var p statusChangePayload
		if err := json.Unmarshal(payload, &p); err != nil {
			return Op{}, fmt.Errorf("decode %s: %w", subject, err)
		}
		return Op{
			Kind:      OpUpdateStatus,
			ID:        p.BookingID,
			Status:    p.NewStatus,
			UpdatedAt: p.Timestamp,
		}, nil
	default:
		return Op{}, fmt.Errorf("unknown subject %q", subject)
	}
}
