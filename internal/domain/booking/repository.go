package booking

import "context"

type Repository interface {
	Save(ctx context.Context, b *Booking) error
	FindByID(ctx context.Context, id string) (*Booking, error)
	GetStatusHistory(ctx context.Context, bookingID string) ([]StatusHistoryEntry, error)
}
