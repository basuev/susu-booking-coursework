package booking

import "context"

type Repository interface {
	Save(ctx context.Context, b *Booking) error
	FindByID(ctx context.Context, id string) (*Booking, error)
	ListByGuestID(ctx context.Context, guestID string, limit int, offset int) ([]*Booking, error)
}
