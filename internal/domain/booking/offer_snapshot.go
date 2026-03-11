package booking

import (
	"fmt"

	"github.com/basuev/susu-booking-coursework/internal/domain"
)

type OfferSnapshot struct {
	offerID  string
	hotelID  string
	roomType string
	price    Money
}

func NewOfferSnapshot(offerID, hotelID, roomType string, price Money) (OfferSnapshot, error) {
	if offerID == "" {
		return OfferSnapshot{}, fmt.Errorf("%w: offer_id is required", domain.ErrInvalidArgument)
	}
	if hotelID == "" {
		return OfferSnapshot{}, fmt.Errorf("%w: hotel_id is required", domain.ErrInvalidArgument)
	}
	if roomType == "" {
		return OfferSnapshot{}, fmt.Errorf("%w: room_type is required", domain.ErrInvalidArgument)
	}
	return OfferSnapshot{
		offerID:  offerID,
		hotelID:  hotelID,
		roomType: roomType,
		price:    price,
	}, nil
}

func (o OfferSnapshot) OfferID() string  { return o.offerID }
func (o OfferSnapshot) HotelID() string  { return o.hotelID }
func (o OfferSnapshot) RoomType() string { return o.roomType }
func (o OfferSnapshot) Price() Money     { return o.price }
