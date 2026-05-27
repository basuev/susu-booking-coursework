package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"gitverse.ru/basuev/susu-booking-coursework/internal/adapter/outbox"
	"gitverse.ru/basuev/susu-booking-coursework/internal/domain"
	"gitverse.ru/basuev/susu-booking-coursework/internal/domain/booking"
)

type BookingRepo struct {
	db *sql.DB
}

func NewBookingRepo(db *sql.DB) *BookingRepo {
	return &BookingRepo{db: db}
}

func (r *BookingRepo) Save(ctx context.Context, b *booking.Booking) error {
	if tx, ok := txFromContext(ctx); ok {
		return saveInTx(ctx, tx, b)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("booking_repo.Save begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := saveInTx(ctx, tx, b); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("booking_repo.Save commit: %w", err)
	}
	b.ClearEvents()
	return nil
}

func saveInTx(ctx context.Context, tx *sql.Tx, b *booking.Booking) error {
	if err := upsertBooking(ctx, tx, b); err != nil {
		return err
	}
	if err := upsertOfferSnapshot(ctx, tx, b); err != nil {
		return err
	}
	if err := appendStatusHistory(ctx, tx, b); err != nil {
		return err
	}
	if err := appendOutbox(ctx, tx, b); err != nil {
		return err
	}
	return nil
}

func appendStatusHistory(ctx context.Context, tx *sql.Tx, b *booking.Booking) error {
	for _, ev := range b.Events() {
		oldStatus, newStatus, reason, ok := statusTransitionFromEvent(ev)
		if !ok {
			continue
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO booking_status_history (booking_id, old_status, new_status, reason, changed_at)
			 VALUES ($1, $2, $3, $4, $5)`,
			b.ID(), string(oldStatus), string(newStatus), nullableString(reason), ev.OccurredAt(),
		)
		if err != nil {
			return fmt.Errorf("booking_repo.appendStatusHistory: %w", err)
		}
	}
	return nil
}

func statusTransitionFromEvent(e booking.Event) (booking.Status, booking.Status, string, bool) {
	switch ev := e.(type) {
	case booking.BookingCancelled:
		return ev.OldStatus, ev.NewStatus, "", true
	case booking.BookingApproved:
		return ev.OldStatus, ev.NewStatus, "", true
	case booking.BookingRejected:
		return ev.OldStatus, ev.NewStatus, ev.Reason, true
	default:
		return "", "", "", false
	}
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func upsertBooking(ctx context.Context, tx *sql.Tx, b *booking.Booking) error {
	res, err := tx.ExecContext(ctx,
		`INSERT INTO booking (id, guest_id, status, total_amount, currency, check_in, check_out, version, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 ON CONFLICT (id) DO UPDATE SET
			status     = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at,
			version    = EXCLUDED.version
		 WHERE booking.version = EXCLUDED.version - 1`,
		b.ID(), b.GuestID(), string(b.Status()),
		b.Total().Amount(), b.Total().Currency(),
		b.Stay().CheckIn(), b.Stay().CheckOut(),
		b.Version(), b.CreatedAt(), b.UpdatedAt(),
	)
	if err != nil {
		return fmt.Errorf("booking_repo.upsertBooking: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("booking_repo.upsertBooking rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("%w: booking %s", domain.ErrConcurrentUpdate, b.ID())
	}
	return nil
}

func upsertOfferSnapshot(ctx context.Context, tx *sql.Tx, b *booking.Booking) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO booking_offer_snapshot (booking_id, offer_id, hotel_id, room_type, price_per_night, currency)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (booking_id) DO NOTHING`,
		b.ID(), b.Offer().OfferID(), b.Offer().HotelID(), b.Offer().RoomType(),
		b.Offer().Price().Amount(), b.Offer().Price().Currency(),
	)
	if err != nil {
		return fmt.Errorf("booking_repo.upsertOfferSnapshot: %w", err)
	}
	return nil
}

func appendOutbox(ctx context.Context, tx *sql.Tx, b *booking.Booking) error {
	for _, ev := range b.Events() {
		msg, err := outbox.FromDomainEvent(ev)
		if err != nil {
			return fmt.Errorf("booking_repo.appendOutbox map: %w", err)
		}
		if err := outbox.AppendMessage(ctx, tx, msg); err != nil {
			return fmt.Errorf("booking_repo.appendOutbox insert: %w", err)
		}
	}
	return nil
}

func (r *BookingRepo) FindByID(ctx context.Context, id string) (*booking.Booking, error) {
	return r.scanBooking(r.db.QueryRowContext(ctx,
		`SELECT b.id, b.guest_id, b.status, b.total_amount, b.currency,
		        b.check_in, b.check_out, b.version, b.created_at, b.updated_at,
		        o.offer_id, o.hotel_id, o.room_type, o.price_per_night, o.currency
		 FROM booking b
		 JOIN booking_offer_snapshot o ON o.booking_id = b.id
		 WHERE b.id = $1`,
		id,
	))
}

func (r *BookingRepo) GetStatusHistory(ctx context.Context, bookingID string) ([]booking.StatusHistoryEntry, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT old_status, new_status, reason, changed_at
		 FROM booking_status_history
		 WHERE booking_id = $1
		 ORDER BY changed_at ASC, id ASC`,
		bookingID,
	)
	if err != nil {
		return nil, fmt.Errorf("booking_repo.GetStatusHistory: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []booking.StatusHistoryEntry
	for rows.Next() {
		var (
			oldStatus, newStatus string
			reason               sql.NullString
			changedAt            time.Time
		)
		if err := rows.Scan(&oldStatus, &newStatus, &reason, &changedAt); err != nil {
			return nil, fmt.Errorf("booking_repo.GetStatusHistory scan: %w", err)
		}
		entry := booking.StatusHistoryEntry{
			OldStatus: booking.Status(oldStatus),
			NewStatus: booking.Status(newStatus),
			ChangedAt: changedAt,
		}
		if reason.Valid {
			entry.Reason = reason.String
		}
		result = append(result, entry)
	}
	return result, rows.Err()
}

func (r *BookingRepo) scanBooking(row *sql.Row) (*booking.Booking, error) {
	var (
		id, guestID, status, currency           string
		totalAmount                             int64
		version                                 int
		checkIn, checkOut, createdAt, updatedAt time.Time
		offerID, hotelID, roomType, offerCur    string
		pricePerNight                           int64
	)

	err := row.Scan(
		&id, &guestID, &status, &totalAmount, &currency,
		&checkIn, &checkOut, &version, &createdAt, &updatedAt,
		&offerID, &hotelID, &roomType, &pricePerNight, &offerCur,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: booking %s", domain.ErrNotFound, id)
		}
		return nil, fmt.Errorf("booking_repo.scanBooking: %w", err)
	}

	return reconstructBooking(
		id, guestID, status, totalAmount, currency,
		checkIn, checkOut, version, createdAt, updatedAt,
		offerID, hotelID, roomType, pricePerNight, offerCur,
	)
}

func reconstructBooking(
	id, guestID, status string, totalAmount int64, currency string,
	checkIn, checkOut time.Time, version int, createdAt, updatedAt time.Time,
	offerID, hotelID, roomType string, pricePerNight int64, offerCur string,
) (*booking.Booking, error) {
	price, err := booking.NewMoney(pricePerNight, offerCur)
	if err != nil {
		return nil, err
	}
	offer, err := booking.NewOfferSnapshot(offerID, hotelID, roomType, price)
	if err != nil {
		return nil, err
	}
	stay, err := booking.NewStayPeriod(checkIn, checkOut)
	if err != nil {
		return nil, err
	}
	total, err := booking.NewMoney(totalAmount, currency)
	if err != nil {
		return nil, err
	}

	return booking.Reconstruct(
		id, guestID, offer, stay, total,
		booking.Status(status), version, createdAt, updatedAt,
	), nil
}
