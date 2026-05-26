package command

import (
	"context"
	"errors"
)

var ErrIdempotencyConflict = errors.New("idempotency key already used")

type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type IdempotencyStore interface {
	Lookup(ctx context.Context, hash string) (bookingID string, found bool, err error)
	Save(ctx context.Context, hash, bookingID string) error
}
