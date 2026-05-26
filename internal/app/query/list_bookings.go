package query

import (
	"context"
	"time"

	"github.com/basuev/susu-booking-coursework/internal/domain/booking"
)

type ListBookings struct {
	GuestID     string
	Status      *booking.Status
	CheckInFrom *time.Time
	CheckInTo   *time.Time
	PageSize    int
	Offset      int
}

type ListBookingsHandler struct {
	repo booking.Repository
}

func NewListBookingsHandler(repo booking.Repository) *ListBookingsHandler {
	return &ListBookingsHandler{repo: repo}
}

func (h *ListBookingsHandler) Handle(ctx context.Context, q ListBookings) ([]*booking.Booking, error) {
	return h.repo.List(ctx, booking.ListFilter{
		GuestID:     q.GuestID,
		Status:      q.Status,
		CheckInFrom: q.CheckInFrom,
		CheckInTo:   q.CheckInTo,
		Limit:       q.PageSize,
		Offset:      q.Offset,
	})
}
