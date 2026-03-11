package booking

import "time"

type Event interface {
	EventName() string
	OccurredAt() time.Time
}

type BookingCreated struct {
	BookingID string
	GuestID   string
	HotelID   string
	CheckIn   time.Time
	CheckOut  time.Time
	Total     Money
	Timestamp time.Time
}

func (e BookingCreated) EventName() string    { return "booking.created" }
func (e BookingCreated) OccurredAt() time.Time { return e.Timestamp }

type BookingCancelled struct {
	BookingID string
	Timestamp time.Time
}

func (e BookingCancelled) EventName() string    { return "booking.cancelled" }
func (e BookingCancelled) OccurredAt() time.Time { return e.Timestamp }

type BookingApproved struct {
	BookingID string
	Timestamp time.Time
}

func (e BookingApproved) EventName() string    { return "booking.approved" }
func (e BookingApproved) OccurredAt() time.Time { return e.Timestamp }

type BookingRejected struct {
	BookingID string
	Reason    string
	Timestamp time.Time
}

func (e BookingRejected) EventName() string    { return "booking.rejected" }
func (e BookingRejected) OccurredAt() time.Time { return e.Timestamp }
