package grpc

import (
	"errors"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "gitverse.ru/basuev/susu-booking-coursework/gen/go/booking/v1"
	"gitverse.ru/basuev/susu-booking-coursework/internal/domain"
	"gitverse.ru/basuev/susu-booking-coursework/internal/domain/booking"
)

func TestDecimalToMinorUnits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{"0.01", 1, false},
		{"199.99", 19999, false},
		{"100", 10000, false},
		{"100.", 10000, false},
		{"100.5", 10050, false},
		{"100.555", 0, true},
		{"abc", 0, true},
		{"", 0, true},
		{"-10.50", -1050, false},
		{"0", 0, false},
		{"0.10", 10, false},
	}
	for _, tc := range tests {
		got, err := decimalToMinorUnits(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("decimalToMinorUnits(%q): want error, got %d", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("decimalToMinorUnits(%q): unexpected error %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("decimalToMinorUnits(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestMinorUnitsToDecimal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   int64
		want string
	}{
		{1, "0.01"},
		{19999, "199.99"},
		{10000, "100.00"},
		{10050, "100.50"},
		{-1050, "-10.50"},
		{0, "0.00"},
		{10, "0.10"},
	}
	for _, tc := range tests {
		got := minorUnitsToDecimal(tc.in)
		if got != tc.want {
			t.Errorf("minorUnitsToDecimal(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMapDomainError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want codes.Code
	}{
		{"nil", nil, codes.OK},
		{"not found", domain.ErrNotFound, codes.NotFound},
		{"invalid argument", domain.ErrInvalidArgument, codes.InvalidArgument},
		{"invalid transition", domain.ErrInvalidTransition, codes.FailedPrecondition},
		{"concurrent update", domain.ErrConcurrentUpdate, codes.Aborted},
		{"wrapped not found", fmt.Errorf("wrap: %w", domain.ErrNotFound), codes.NotFound},
		{"unknown", errors.New("boom"), codes.Internal},
	}
	for _, tc := range tests {
		got := mapDomainError(tc.err)
		if tc.err == nil {
			if got != nil {
				t.Errorf("%s: want nil, got %v", tc.name, got)
			}
			continue
		}
		st, ok := status.FromError(got)
		if !ok {
			t.Errorf("%s: not a gRPC status error: %v", tc.name, got)
			continue
		}
		if st.Code() != tc.want {
			t.Errorf("%s: got code %s, want %s", tc.name, st.Code(), tc.want)
		}
	}
}

func TestStatusToProto(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   booking.Status
		want pb.BookingStatus
	}{
		{booking.StatusPending, pb.BookingStatus_BOOKING_STATUS_PENDING},
		{booking.StatusConfirmed, pb.BookingStatus_BOOKING_STATUS_CONFIRMED},
		{booking.StatusApproved, pb.BookingStatus_BOOKING_STATUS_APPROVED},
		{booking.StatusRejected, pb.BookingStatus_BOOKING_STATUS_REJECTED},
		{booking.StatusCancelled, pb.BookingStatus_BOOKING_STATUS_CANCELLED},
		{booking.Status("WAT"), pb.BookingStatus_BOOKING_STATUS_UNSPECIFIED},
		{booking.Status(""), pb.BookingStatus_BOOKING_STATUS_UNSPECIFIED},
	}
	for _, tc := range tests {
		got := statusToProto(tc.in)
		if got != tc.want {
			t.Errorf("statusToProto(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestStatusFilterFromProto(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in     pb.BookingStatus
		want   booking.Status
		wantOk bool
	}{
		{pb.BookingStatus_BOOKING_STATUS_PENDING, booking.StatusPending, true},
		{pb.BookingStatus_BOOKING_STATUS_APPROVED, booking.StatusApproved, true},
		{pb.BookingStatus_BOOKING_STATUS_UNSPECIFIED, "", false},
	}
	for _, tc := range tests {
		got, ok := statusFilterFromProto(tc.in)
		if ok != tc.wantOk {
			t.Errorf("statusFilterFromProto(%v) ok=%v, want %v", tc.in, ok, tc.wantOk)
		}
		if ok && got != tc.want {
			t.Errorf("statusFilterFromProto(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMoneyToProto(t *testing.T) {
	t.Parallel()
	m, err := booking.NewMoney(19999, "RUB")
	if err != nil {
		t.Fatalf("money: %v", err)
	}
	got := moneyToProto(m)
	if got.GetAmount() != "199.99" || got.GetCurrency() != "RUB" {
		t.Fatalf("got %+v, want amount=199.99 currency=RUB", got)
	}
}

func TestMoneyFromProto(t *testing.T) {
	t.Parallel()

	got, err := moneyFromProto(&pb.Money{Amount: "199.99", Currency: "RUB"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Amount() != 19999 || got.Currency() != "RUB" {
		t.Fatalf("got amount=%d currency=%s", got.Amount(), got.Currency())
	}

	if _, err := moneyFromProto(nil); !errors.Is(err, domain.ErrInvalidArgument) {
		t.Fatalf("nil money: want ErrInvalidArgument, got %v", err)
	}

	if _, err := moneyFromProto(&pb.Money{Amount: "abc", Currency: "RUB"}); !errors.Is(err, domain.ErrInvalidArgument) {
		t.Fatalf("invalid amount: want ErrInvalidArgument, got %v", err)
	}
}
