package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type BookingProjection struct {
	ID          string
	GuestID     string
	HotelID     string
	RoomType    string
	CheckIn     time.Time
	CheckOut    time.Time
	TotalAmount int64
	Currency    string
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ProjectionRepo struct {
	db *sql.DB
}

func NewProjectionRepo(db *sql.DB) *ProjectionRepo {
	return &ProjectionRepo{db: db}
}

func (r *ProjectionRepo) Upsert(ctx context.Context, p BookingProjection) error {
	query := `
		INSERT INTO booking_projection (id, guest_id, hotel_id, room_type, check_in, check_out, total_amount, currency, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (id) DO UPDATE SET
			status     = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at`

	_, err := r.db.ExecContext(ctx, query,
		p.ID, p.GuestID, p.HotelID, p.RoomType,
		p.CheckIn, p.CheckOut, p.TotalAmount, p.Currency,
		p.Status, p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("projection_repo.Upsert: %w", err)
	}
	return nil
}

func (r *ProjectionRepo) FindByGuestID(ctx context.Context, guestID string, limit, offset int) ([]BookingProjection, error) {
	query := `
		SELECT id, guest_id, hotel_id, room_type, check_in, check_out,
		       total_amount, currency, status, created_at, updated_at
		FROM booking_projection
		WHERE guest_id = $1
		ORDER BY updated_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryContext(ctx, query, guestID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("projection_repo.FindByGuestID: %w", err)
	}
	defer rows.Close()

	var result []BookingProjection
	for rows.Next() {
		var p BookingProjection
		if err := rows.Scan(
			&p.ID, &p.GuestID, &p.HotelID, &p.RoomType,
			&p.CheckIn, &p.CheckOut, &p.TotalAmount, &p.Currency,
			&p.Status, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("projection_repo.scan: %w", err)
		}
		result = append(result, p)
	}
	return result, rows.Err()
}
