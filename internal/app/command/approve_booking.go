package command

import (
	"context"

	"github.com/basuev/susu-booking-coursework/internal/domain/booking"
)

type ApproveBooking struct {
	BookingID string
}

type ApproveBookingHandler struct {
	repo booking.Repository
}

func NewApproveBookingHandler(repo booking.Repository) *ApproveBookingHandler {
	return &ApproveBookingHandler{repo: repo}
}

func (h *ApproveBookingHandler) Handle(ctx context.Context, cmd ApproveBooking) (*booking.Booking, error) {
	b, err := h.repo.FindByID(ctx, cmd.BookingID)
	if err != nil {
		return nil, err
	}

	if err := b.Approve(); err != nil {
		return nil, err
	}

	if err := h.repo.Save(ctx, b); err != nil {
		return nil, err
	}

	return b, nil
}
