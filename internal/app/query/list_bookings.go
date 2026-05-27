package query

import (
	"context"
	"time"

	"gitverse.ru/basuev/susu-booking-coursework/internal/domain/booking"
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
	reader ProjectionReader
}

func NewListBookingsHandler(reader ProjectionReader) *ListBookingsHandler {
	return &ListBookingsHandler{reader: reader}
}

func (h *ListBookingsHandler) Handle(ctx context.Context, q ListBookings) ([]ProjectionItem, error) {
	return h.reader.ListByGuest(ctx, ProjectionFilter{
		GuestID:     q.GuestID,
		Status:      q.Status,
		CheckInFrom: q.CheckInFrom,
		CheckInTo:   q.CheckInTo,
		Limit:       q.PageSize,
		Offset:      q.Offset,
	})
}
