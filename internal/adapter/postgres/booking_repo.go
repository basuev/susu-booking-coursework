package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/basuev/susu-booking-coursework/internal/domain"
	"github.com/basuev/susu-booking-coursework/internal/domain/booking"
)

type BookingRepo struct {
	db *sql.DB
}

func NewBookingRepo(db *sql.DB) *BookingRepo {
	return &BookingRepo{db: db}
}

func (r *BookingRepo) Save(ctx context.Context, b *booking.Booking) error {
	query := `
		INSERT INTO booking (id, guest_id, status, total_amount, currency, check_in, check_out, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			status     = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at`

	_, err := r.db.ExecContext(ctx, query,
		b.ID(),
		b.GuestID(),
		string(b.Status()),
		b.Total().Amount(),
		b.Total().Currency(),
		b.Stay().CheckIn(),
		b.Stay().CheckOut(),
		b.CreatedAt(),
		b.UpdatedAt(),
	)
	if err != nil {
		return fmt.Errorf("booking_repo.Save: %w", err)
	}

	return r.saveOfferSnapshot(ctx, b)
}

func (r *BookingRepo) saveOfferSnapshot(ctx context.Context, b *booking.Booking) error {
	query := `
		INSERT INTO booking_offer_snapshot (booking_id, offer_id, hotel_id, room_type, price_per_night, currency)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (booking_id) DO NOTHING`

	_, err := r.db.ExecContext(ctx, query,
		b.ID(),
		b.Offer().OfferID(),
		b.Offer().HotelID(),
		b.Offer().RoomType(),
		b.Offer().Price().Amount(),
		b.Offer().Price().Currency(),
	)
	if err != nil {
		return fmt.Errorf("booking_repo.saveOfferSnapshot: %w", err)
	}
	return nil
}

func (r *BookingRepo) FindByID(ctx context.Context, id string) (*booking.Booking, error) {
	query := `
		SELECT b.id, b.guest_id, b.status, b.total_amount, b.currency,
		       b.check_in, b.check_out, b.created_at, b.updated_at,
		       o.offer_id, o.hotel_id, o.room_type, o.price_per_night, o.currency
		FROM booking b
		JOIN booking_offer_snapshot o ON o.booking_id = b.id
		WHERE b.id = $1`

	return r.scanBooking(r.db.QueryRowContext(ctx, query, id))
}

func (r *BookingRepo) ListByGuestID(ctx context.Context, guestID string, limit, offset int) ([]*booking.Booking, error) {
	query := `
		SELECT b.id, b.guest_id, b.status, b.total_amount, b.currency,
		       b.check_in, b.check_out, b.created_at, b.updated_at,
		       o.offer_id, o.hotel_id, o.room_type, o.price_per_night, o.currency
		FROM booking b
		JOIN booking_offer_snapshot o ON o.booking_id = b.id
		WHERE b.guest_id = $1
		ORDER BY b.created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryContext(ctx, query, guestID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("booking_repo.ListByGuestID: %w", err)
	}
	defer rows.Close()

	var result []*booking.Booking
	for rows.Next() {
		b, err := r.scanBookingFromRows(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, b)
	}
	return result, rows.Err()
}

func (r *BookingRepo) scanBooking(row *sql.Row) (*booking.Booking, error) {
	var (
		id, guestID, status, currency         string
		totalAmount                            int64
		checkIn, checkOut, createdAt, updatedAt time.Time
		offerID, hotelID, roomType, offerCur   string
		pricePerNight                          int64
	)

	err := row.Scan(
		&id, &guestID, &status, &totalAmount, &currency,
		&checkIn, &checkOut, &createdAt, &updatedAt,
		&offerID, &hotelID, &roomType, &pricePerNight, &offerCur,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("%w: booking %s", domain.ErrNotFound, id)
		}
		return nil, fmt.Errorf("booking_repo.scanBooking: %w", err)
	}

	return reconstructBooking(
		id, guestID, status, totalAmount, currency,
		checkIn, checkOut, createdAt, updatedAt,
		offerID, hotelID, roomType, pricePerNight, offerCur,
	)
}

func (r *BookingRepo) scanBookingFromRows(rows *sql.Rows) (*booking.Booking, error) {
	var (
		id, guestID, status, currency         string
		totalAmount                            int64
		checkIn, checkOut, createdAt, updatedAt time.Time
		offerID, hotelID, roomType, offerCur   string
		pricePerNight                          int64
	)

	err := rows.Scan(
		&id, &guestID, &status, &totalAmount, &currency,
		&checkIn, &checkOut, &createdAt, &updatedAt,
		&offerID, &hotelID, &roomType, &pricePerNight, &offerCur,
	)
	if err != nil {
		return nil, fmt.Errorf("booking_repo.scanBookingFromRows: %w", err)
	}

	return reconstructBooking(
		id, guestID, status, totalAmount, currency,
		checkIn, checkOut, createdAt, updatedAt,
		offerID, hotelID, roomType, pricePerNight, offerCur,
	)
}

func reconstructBooking(
	id, guestID, status string, totalAmount int64, currency string,
	checkIn, checkOut, createdAt, updatedAt time.Time,
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
		booking.Status(status), createdAt, updatedAt,
	), nil
}
