package booking

import (
	"fmt"
	"time"

	"github.com/tbasuev/susu-booking-coursework/internal/domain"
	"github.com/tbasuev/susu-booking-coursework/pkg/id"
)

type Booking struct {
	id        string
	guestID   string
	offer     OfferSnapshot
	stay      StayPeriod
	total     Money
	status    Status
	createdAt time.Time
	updatedAt time.Time
	events    []Event
}

func NewBooking(guestID string, offer OfferSnapshot, stay StayPeriod) (*Booking, error) {
	if guestID == "" {
		return nil, fmt.Errorf("%w: guest_id is required", domain.ErrInvalidArgument)
	}

	total := offer.Price().Multiply(stay.Nights())
	now := time.Now()

	b := &Booking{
		id:        id.New(),
		guestID:   guestID,
		offer:     offer,
		stay:      stay,
		total:     total,
		status:    StatusPending,
		createdAt: now,
		updatedAt: now,
	}

	b.record(BookingCreated{
		BookingID: b.id,
		GuestID:   b.guestID,
		HotelID:   offer.HotelID(),
		CheckIn:   stay.CheckIn(),
		CheckOut:  stay.CheckOut(),
		Total:     total,
		Timestamp: now,
	})

	return b, nil
}

// Reconstruct creates a Booking from persisted data without generating events.
func Reconstruct(
	bookingID, guestID string,
	offer OfferSnapshot,
	stay StayPeriod,
	total Money,
	status Status,
	createdAt, updatedAt time.Time,
) *Booking {
	return &Booking{
		id:        bookingID,
		guestID:   guestID,
		offer:     offer,
		stay:      stay,
		total:     total,
		status:    status,
		createdAt: createdAt,
		updatedAt: updatedAt,
	}
}

func (b *Booking) Cancel() error {
	newStatus, err := b.status.TransitionTo(StatusCancelled)
	if err != nil {
		return err
	}
	b.status = newStatus
	b.updatedAt = time.Now()
	b.record(BookingCancelled{BookingID: b.id, Timestamp: b.updatedAt})
	return nil
}

func (b *Booking) Approve() error {
	newStatus, err := b.status.TransitionTo(StatusApproved)
	if err != nil {
		return err
	}
	b.status = newStatus
	b.updatedAt = time.Now()
	b.record(BookingApproved{BookingID: b.id, Timestamp: b.updatedAt})
	return nil
}

func (b *Booking) Reject(reason string) error {
	newStatus, err := b.status.TransitionTo(StatusRejected)
	if err != nil {
		return err
	}
	b.status = newStatus
	b.updatedAt = time.Now()
	b.record(BookingRejected{BookingID: b.id, Reason: reason, Timestamp: b.updatedAt})
	return nil
}

func (b *Booking) Confirm() error {
	newStatus, err := b.status.TransitionTo(StatusConfirmed)
	if err != nil {
		return err
	}
	b.status = newStatus
	b.updatedAt = time.Now()
	return nil
}

func (b *Booking) record(e Event) {
	b.events = append(b.events, e)
}

func (b *Booking) Events() []Event {
	return b.events
}

func (b *Booking) ClearEvents() {
	b.events = nil
}

func (b *Booking) ID() string             { return b.id }
func (b *Booking) GuestID() string        { return b.guestID }
func (b *Booking) Offer() OfferSnapshot   { return b.offer }
func (b *Booking) Stay() StayPeriod       { return b.stay }
func (b *Booking) Total() Money           { return b.total }
func (b *Booking) Status() Status         { return b.status }
func (b *Booking) CreatedAt() time.Time   { return b.createdAt }
func (b *Booking) UpdatedAt() time.Time   { return b.updatedAt }
