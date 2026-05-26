package booking

import (
	"context"
	"time"
)

type ListFilter struct {
	GuestID     string
	Status      *Status
	CheckInFrom *time.Time
	CheckInTo   *time.Time
	Limit       int
	Offset      int
}

type Repository interface {
	Save(ctx context.Context, b *Booking) error
	FindByID(ctx context.Context, id string) (*Booking, error)
	List(ctx context.Context, filter ListFilter) ([]*Booking, error)
	GetStatusHistory(ctx context.Context, bookingID string) ([]StatusHistoryEntry, error)
}
