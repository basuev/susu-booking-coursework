package grpc

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "gitverse.ru/basuev/susu-booking-coursework/gen/go/booking/v1"
	"gitverse.ru/basuev/susu-booking-coursework/internal/domain"
	"gitverse.ru/basuev/susu-booking-coursework/internal/domain/booking"
)

func decimalToMinorUnits(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty amount")
	}

	parts := strings.SplitN(s, ".", 2)
	major, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount major part: %w", err)
	}

	if len(parts) == 1 {
		return major * 100, nil
	}

	minor := parts[1]
	switch {
	case len(minor) == 0:
		return major * 100, nil
	case len(minor) == 1:
		minor += "0"
	case len(minor) > 2:
		return 0, fmt.Errorf("amount has more than 2 decimal places")
	}

	minorVal, err := strconv.ParseInt(minor, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount minor part: %w", err)
	}

	if major < 0 {
		return major*100 - minorVal, nil
	}
	return major*100 + minorVal, nil
}

func minorUnitsToDecimal(v int64) string {
	negative := v < 0
	if negative {
		v = -v
	}
	major := v / 100
	minor := v % 100
	if negative {
		return fmt.Sprintf("-%d.%02d", major, minor)
	}
	return fmt.Sprintf("%d.%02d", major, minor)
}

func moneyToProto(m booking.Money) *pb.Money {
	return &pb.Money{
		Amount:   minorUnitsToDecimal(m.Amount()),
		Currency: m.Currency(),
	}
}

func moneyFromProto(m *pb.Money) (booking.Money, error) {
	if m == nil {
		return booking.Money{}, fmt.Errorf("%w: money is required", domain.ErrInvalidArgument)
	}
	amount, err := decimalToMinorUnits(m.GetAmount())
	if err != nil {
		return booking.Money{}, fmt.Errorf("%w: %s", domain.ErrInvalidArgument, err.Error())
	}
	return booking.NewMoney(amount, m.GetCurrency())
}

func offerToProto(o booking.OfferSnapshot) *pb.OfferSnapshot {
	return &pb.OfferSnapshot{
		OfferId:       o.OfferID(),
		HotelId:       o.HotelID(),
		RoomType:      o.RoomType(),
		PricePerNight: moneyToProto(o.Price()),
	}
}

func offerFromProto(o *pb.OfferSnapshot) (booking.OfferSnapshot, error) {
	if o == nil {
		return booking.OfferSnapshot{}, fmt.Errorf("%w: offer is required", domain.ErrInvalidArgument)
	}
	price, err := moneyFromProto(o.GetPricePerNight())
	if err != nil {
		return booking.OfferSnapshot{}, err
	}
	return booking.NewOfferSnapshot(o.GetOfferId(), o.GetHotelId(), o.GetRoomType(), price)
}

var statusToProtoMap = map[booking.Status]pb.BookingStatus{
	booking.StatusPending:   pb.BookingStatus_BOOKING_STATUS_PENDING,
	booking.StatusConfirmed: pb.BookingStatus_BOOKING_STATUS_CONFIRMED,
	booking.StatusApproved:  pb.BookingStatus_BOOKING_STATUS_APPROVED,
	booking.StatusRejected:  pb.BookingStatus_BOOKING_STATUS_REJECTED,
	booking.StatusCancelled: pb.BookingStatus_BOOKING_STATUS_CANCELLED,
}

var statusFromProtoMap = map[pb.BookingStatus]booking.Status{
	pb.BookingStatus_BOOKING_STATUS_PENDING:   booking.StatusPending,
	pb.BookingStatus_BOOKING_STATUS_CONFIRMED: booking.StatusConfirmed,
	pb.BookingStatus_BOOKING_STATUS_APPROVED:  booking.StatusApproved,
	pb.BookingStatus_BOOKING_STATUS_REJECTED:  booking.StatusRejected,
	pb.BookingStatus_BOOKING_STATUS_CANCELLED: booking.StatusCancelled,
}

func statusToProto(s booking.Status) pb.BookingStatus {
	if v, ok := statusToProtoMap[s]; ok {
		return v
	}
	return pb.BookingStatus_BOOKING_STATUS_UNSPECIFIED
}

func statusFilterFromProto(s pb.BookingStatus) (booking.Status, bool) {
	v, ok := statusFromProtoMap[s]
	return v, ok
}

func bookingToProto(b *booking.Booking) *pb.Booking {
	return &pb.Booking{
		Id:         b.ID(),
		GuestId:    b.GuestID(),
		Offer:      offerToProto(b.Offer()),
		CheckIn:    timestamppb.New(b.Stay().CheckIn()),
		CheckOut:   timestamppb.New(b.Stay().CheckOut()),
		TotalPrice: moneyToProto(b.Total()),
		Status:     statusToProto(b.Status()),
		CreatedAt:  timestamppb.New(b.CreatedAt()),
		UpdatedAt:  timestamppb.New(b.UpdatedAt()),
	}
}

func statusHistoryToProto(e booking.StatusHistoryEntry) *pb.StatusHistoryEntry {
	return &pb.StatusHistoryEntry{
		OldStatus: statusToProto(e.OldStatus),
		NewStatus: statusToProto(e.NewStatus),
		Reason:    e.Reason,
		ChangedAt: timestamppb.New(e.ChangedAt),
	}
}

func bookingToSummary(b *booking.Booking) *pb.BookingSummary {
	return &pb.BookingSummary{
		Id:         b.ID(),
		GuestId:    b.GuestID(),
		HotelId:    b.Offer().HotelID(),
		CheckIn:    timestamppb.New(b.Stay().CheckIn()),
		CheckOut:   timestamppb.New(b.Stay().CheckOut()),
		TotalPrice: moneyToProto(b.Total()),
		Status:     statusToProto(b.Status()),
		CreatedAt:  timestamppb.New(b.CreatedAt()),
	}
}

func mapDomainError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrInvalidArgument):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrInvalidTransition):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrConcurrentUpdate):
		return status.Error(codes.Aborted, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
