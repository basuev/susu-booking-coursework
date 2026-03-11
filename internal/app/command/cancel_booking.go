package command

import (
	"context"

	"github.com/basuev/susu-booking-coursework/internal/domain/booking"
)

type CancelBooking struct {
	BookingID string
}

type CancelBookingHandler struct {
	repo booking.Repository
}

func NewCancelBookingHandler(repo booking.Repository) *CancelBookingHandler {
	return &CancelBookingHandler{repo: repo}
}

func (h *CancelBookingHandler) Handle(ctx context.Context, cmd CancelBooking) (*booking.Booking, error) {
	b, err := h.repo.FindByID(ctx, cmd.BookingID)
	if err != nil {
		return nil, err
	}

	if err := b.Cancel(); err != nil {
		return nil, err
	}

	if err := h.repo.Save(ctx, b); err != nil {
		return nil, err
	}

	return b, nil
}
