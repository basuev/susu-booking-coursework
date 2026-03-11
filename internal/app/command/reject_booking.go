package command

import (
	"context"

	"github.com/basuev/susu-booking-coursework/internal/domain/booking"
)

type RejectBooking struct {
	BookingID string
	Reason    string
}

type RejectBookingHandler struct {
	repo booking.Repository
}

func NewRejectBookingHandler(repo booking.Repository) *RejectBookingHandler {
	return &RejectBookingHandler{repo: repo}
}

func (h *RejectBookingHandler) Handle(ctx context.Context, cmd RejectBooking) (*booking.Booking, error) {
	b, err := h.repo.FindByID(ctx, cmd.BookingID)
	if err != nil {
		return nil, err
	}

	if err := b.Reject(cmd.Reason); err != nil {
		return nil, err
	}

	if err := h.repo.Save(ctx, b); err != nil {
		return nil, err
	}

	return b, nil
}
