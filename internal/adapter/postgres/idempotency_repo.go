package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/basuev/susu-booking-coursework/internal/app/command"
)

type IdempotencyRepo struct {
	db *sql.DB
}

func NewIdempotencyRepo(db *sql.DB) *IdempotencyRepo {
	return &IdempotencyRepo{db: db}
}

func (r *IdempotencyRepo) Lookup(ctx context.Context, hash string) (string, bool, error) {
	q := `SELECT booking_id FROM idempotency_key WHERE request_hash = $1`

	var bookingID string
	var err error
	if tx, ok := txFromContext(ctx); ok {
		err = tx.QueryRowContext(ctx, q, hash).Scan(&bookingID)
	} else {
		err = r.db.QueryRowContext(ctx, q, hash).Scan(&bookingID)
	}

	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("idempotency_repo.Lookup: %w", err)
	}
	return bookingID, true, nil
}

func (r *IdempotencyRepo) Save(ctx context.Context, hash, bookingID string) error {
	q := `INSERT INTO idempotency_key (request_hash, booking_id) VALUES ($1, $2)`

	var err error
	if tx, ok := txFromContext(ctx); ok {
		_, err = tx.ExecContext(ctx, q, hash, bookingID)
	} else {
		_, err = r.db.ExecContext(ctx, q, hash, bookingID)
	}

	if err != nil {
		if IsUniqueViolation(err) {
			return command.ErrIdempotencyConflict
		}
		return fmt.Errorf("idempotency_repo.Save: %w", err)
	}
	return nil
}
