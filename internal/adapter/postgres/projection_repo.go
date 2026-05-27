package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"gitverse.ru/basuev/susu-booking-coursework/internal/app/query"
	"gitverse.ru/basuev/susu-booking-coursework/internal/domain/booking"
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

func (r *ProjectionRepo) UpdateStatus(ctx context.Context, id, newStatus string, updatedAt time.Time) error {
	query := `
		UPDATE booking_projection
		   SET status     = $2,
		       updated_at = $3
		 WHERE id         = $1`
	_, err := r.db.ExecContext(ctx, query, id, newStatus, updatedAt)
	if err != nil {
		return fmt.Errorf("projection_repo.UpdateStatus: %w", err)
	}
	return nil
}

func (r *ProjectionRepo) ListByGuest(ctx context.Context, f query.ProjectionFilter) ([]query.ProjectionItem, error) {
	q := `SELECT id, guest_id, hotel_id, room_type, check_in, check_out,
	             total_amount, currency, status, created_at, updated_at
	      FROM booking_projection
	      WHERE guest_id = $1`
	args := []any{f.GuestID}
	idx := 2
	if f.Status != nil {
		q += fmt.Sprintf(" AND status = $%d", idx)
		args = append(args, string(*f.Status))
		idx++
	}
	if f.CheckInFrom != nil {
		q += fmt.Sprintf(" AND check_in >= $%d", idx)
		args = append(args, *f.CheckInFrom)
		idx++
	}
	if f.CheckInTo != nil {
		q += fmt.Sprintf(" AND check_in <= $%d", idx)
		args = append(args, *f.CheckInTo)
		idx++
	}
	q += fmt.Sprintf(" ORDER BY updated_at DESC LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, f.Limit, f.Offset)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("projection_repo.ListByGuest: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []query.ProjectionItem
	for rows.Next() {
		var p BookingProjection
		if err := rows.Scan(
			&p.ID, &p.GuestID, &p.HotelID, &p.RoomType,
			&p.CheckIn, &p.CheckOut, &p.TotalAmount, &p.Currency,
			&p.Status, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("projection_repo.scan: %w", err)
		}
		result = append(result, query.ProjectionItem{
			ID:         p.ID,
			GuestID:    p.GuestID,
			HotelID:    p.HotelID,
			RoomType:   p.RoomType,
			CheckIn:    p.CheckIn,
			CheckOut:   p.CheckOut,
			TotalMinor: p.TotalAmount,
			Currency:   p.Currency,
			Status:     booking.Status(p.Status),
			CreatedAt:  p.CreatedAt,
			UpdatedAt:  p.UpdatedAt,
		})
	}
	return result, rows.Err()
}
