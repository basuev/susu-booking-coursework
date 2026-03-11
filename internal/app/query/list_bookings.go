package query

import (
	"context"

	"github.com/basuev/susu-booking-coursework/internal/domain/booking"
)

type ListBookings struct {
	GuestID  string
	PageSize int
	Offset   int
}

type ListBookingsHandler struct {
	repo booking.Repository
}

func NewListBookingsHandler(repo booking.Repository) *ListBookingsHandler {
	return &ListBookingsHandler{repo: repo}
}

func (h *ListBookingsHandler) Handle(ctx context.Context, q ListBookings) ([]*booking.Booking, error) {
	pageSize := q.PageSize
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	return h.repo.ListByGuestID(ctx, q.GuestID, pageSize, q.Offset)
}
