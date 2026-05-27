package command

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"gitverse.ru/basuev/susu-booking-coursework/internal/domain"
	"gitverse.ru/basuev/susu-booking-coursework/internal/domain/booking"
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
	repo        booking.Repository
	idempotency IdempotencyStore
	txManager   TxManager
}

func NewCreateBookingHandler(
	repo booking.Repository,
	idempotency IdempotencyStore,
	txManager TxManager,
) *CreateBookingHandler {
	return &CreateBookingHandler{
		repo:        repo,
		idempotency: idempotency,
		txManager:   txManager,
	}
}

func (h *CreateBookingHandler) Handle(ctx context.Context, cmd CreateBooking) (*booking.Booking, error) {
	if cmd.IdempotencyKey == "" {
		return nil, fmt.Errorf("%w: idempotency_key is required", domain.ErrInvalidArgument)
	}

	hash := hashIdempotencyKey(cmd.IdempotencyKey)

	if existingID, found, err := h.idempotency.Lookup(ctx, hash); err != nil {
		return nil, err
	} else if found {
		return h.repo.FindByID(ctx, existingID)
	}

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

	txErr := h.txManager.WithinTx(ctx, func(ctx context.Context) error {
		if err := h.repo.Save(ctx, b); err != nil {
			return err
		}
		return h.idempotency.Save(ctx, hash, b.ID())
	})

	if errors.Is(txErr, ErrIdempotencyConflict) {
		existingID, found, lookupErr := h.idempotency.Lookup(ctx, hash)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if !found {
			return nil, txErr
		}
		return h.repo.FindByID(ctx, existingID)
	}
	if txErr != nil {
		return nil, txErr
	}
	return b, nil
}

func hashIdempotencyKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}
