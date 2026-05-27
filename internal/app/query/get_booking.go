package query

import (
	"context"

	"gitverse.ru/basuev/susu-booking-coursework/internal/domain/booking"
)

type GetBooking struct {
	BookingID string
}

type GetBookingResult struct {
	Booking *booking.Booking
	History []booking.StatusHistoryEntry
}

type GetBookingHandler struct {
	repo booking.Repository
}

func NewGetBookingHandler(repo booking.Repository) *GetBookingHandler {
	return &GetBookingHandler{repo: repo}
}

func (h *GetBookingHandler) Handle(ctx context.Context, q GetBooking) (GetBookingResult, error) {
	b, err := h.repo.FindByID(ctx, q.BookingID)
	if err != nil {
		return GetBookingResult{}, err
	}
	history, err := h.repo.GetStatusHistory(ctx, q.BookingID)
	if err != nil {
		return GetBookingResult{}, err
	}
	return GetBookingResult{Booking: b, History: history}, nil
}
