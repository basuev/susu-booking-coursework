package command

import (
	"context"
	"time"

	"github.com/basuev/susu-booking-coursework/internal/domain/booking"
)

type CreateBooking struct {
	IdempotencyKey string
	GuestID        string
	OfferID        string
	HotelID        string
	RoomType       string
	PricePerNight  int64
	Currency       string
	CheckIn        time.Time
	CheckOut       time.Time
}

type CreateBookingHandler struct {
	repo booking.Repository
}

func NewCreateBookingHandler(repo booking.Repository) *CreateBookingHandler {
	return &CreateBookingHandler{repo: repo}
}

func (h *CreateBookingHandler) Handle(ctx context.Context, cmd CreateBooking) (*booking.Booking, error) {
	price, err := booking.NewMoney(cmd.PricePerNight, cmd.Currency)
	if err != nil {
		return nil, err
	}

	offer, err := booking.NewOfferSnapshot(cmd.OfferID, cmd.HotelID, cmd.RoomType, price)
	if err != nil {
		return nil, err
	}

	stay, err := booking.NewStayPeriod(cmd.CheckIn, cmd.CheckOut)
	if err != nil {
		return nil, err
	}

	b, err := booking.NewBooking(cmd.GuestID, offer, stay)
	if err != nil {
		return nil, err
	}

	if err := h.repo.Save(ctx, b); err != nil {
		return nil, err
	}

	return b, nil
}
