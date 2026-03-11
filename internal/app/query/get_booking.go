package query

import (
	"context"

	"github.com/basuev/susu-booking-coursework/internal/domain/booking"
)

type GetBooking struct {
	BookingID string
}

type GetBookingHandler struct {
	repo booking.Repository
}

func NewGetBookingHandler(repo booking.Repository) *GetBookingHandler {
	return &GetBookingHandler{repo: repo}
}

func (h *GetBookingHandler) Handle(ctx context.Context, q GetBooking) (*booking.Booking, error) {
	return h.repo.FindByID(ctx, q.BookingID)
}
