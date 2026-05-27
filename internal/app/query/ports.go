package query

import (
	"context"
	"time"

	"gitverse.ru/basuev/susu-booking-coursework/internal/domain/booking"
)

type ProjectionItem struct {
	ID         string
	GuestID    string
	HotelID    string
	RoomType   string
	CheckIn    time.Time
	CheckOut   time.Time
	TotalMinor int64
	Currency   string
	Status     booking.Status
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type ProjectionFilter struct {
	GuestID     string
	Status      *booking.Status
	CheckInFrom *time.Time
	CheckInTo   *time.Time
	Limit       int
	Offset      int
}

type ProjectionReader interface {
	ListByGuest(ctx context.Context, f ProjectionFilter) ([]ProjectionItem, error)
}
