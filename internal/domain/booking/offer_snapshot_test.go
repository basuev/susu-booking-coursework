package booking

import (
	"errors"
	"testing"

	"github.com/basuev/susu-booking-coursework/internal/domain"
)

func TestNewOfferSnapshot(t *testing.T) {
	t.Parallel()

	price, err := NewMoney(10000, "RUB")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	cases := []struct {
		name     string
		offerID  string
		hotelID  string
		roomType string
		wantErr  error
	}{
		{"valid offer", "offer-1", "hotel-1", "STANDARD", nil},
		{"missing offer_id", "", "hotel-1", "STANDARD", domain.ErrInvalidArgument},
		{"missing hotel_id", "offer-1", "", "STANDARD", domain.ErrInvalidArgument},
		{"missing room_type", "offer-1", "hotel-1", "", domain.ErrInvalidArgument},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			o, err := NewOfferSnapshot(c.offerID, c.hotelID, c.roomType, price)
			if c.wantErr != nil {
				if !errors.Is(err, c.wantErr) {
					t.Fatalf("want %v, got %v", c.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if o.OfferID() != c.offerID || o.HotelID() != c.hotelID || o.RoomType() != c.roomType {
				t.Fatalf("unexpected fields: %+v", o)
			}
			if !o.Price().Equal(price) {
				t.Fatalf("price preserved")
			}
		})
	}
}
