package grpc

import (
	"context"
	"fmt"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "gitverse.ru/basuev/susu-booking-coursework/gen/go/booking/v1"
	"gitverse.ru/basuev/susu-booking-coursework/internal/app/command"
	"gitverse.ru/basuev/susu-booking-coursework/internal/app/query"
	"gitverse.ru/basuev/susu-booking-coursework/internal/domain"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

type BookingHandler struct {
	pb.UnimplementedBookingServiceServer

	createBooking  *command.CreateBookingHandler
	cancelBooking  *command.CancelBookingHandler
	approveBooking *command.ApproveBookingHandler
	rejectBooking  *command.RejectBookingHandler
	getBooking     *query.GetBookingHandler
	listBookings   *query.ListBookingsHandler
}

func NewBookingHandler(
	create *command.CreateBookingHandler,
	cancel *command.CancelBookingHandler,
	approve *command.ApproveBookingHandler,
	reject *command.RejectBookingHandler,
	get *query.GetBookingHandler,
	list *query.ListBookingsHandler,
) *BookingHandler {
	return &BookingHandler{
		createBooking:  create,
		cancelBooking:  cancel,
		approveBooking: approve,
		rejectBooking:  reject,
		getBooking:     get,
		listBookings:   list,
	}
}

func (h *BookingHandler) Register(gs *grpc.Server) {
	pb.RegisterBookingServiceServer(gs, h)
}

func (h *BookingHandler) CreateBooking(ctx context.Context, req *pb.CreateBookingRequest) (*pb.CreateBookingResponse, error) {
	if req.GetIdempotencyKey() == "" {
		return nil, status.Error(codes.InvalidArgument, "idempotency_key is required")
	}

	offer, err := offerFromProto(req.GetOffer())
	if err != nil {
		return nil, mapDomainError(err)
	}

	if req.GetCheckIn() == nil || req.GetCheckOut() == nil {
		return nil, status.Errorf(codes.InvalidArgument, "%s: check_in and check_out are required", domain.ErrInvalidArgument)
	}

	cmd := command.CreateBooking{
		IdempotencyKey: req.GetIdempotencyKey(),
		GuestID:        req.GetGuestId(),
		OfferID:        offer.OfferID(),
		HotelID:        offer.HotelID(),
		RoomType:       offer.RoomType(),
		PricePerNight:  offer.Price().Amount(),
		Currency:       offer.Price().Currency(),
		CheckIn:        req.GetCheckIn().AsTime(),
		CheckOut:       req.GetCheckOut().AsTime(),
	}

	b, err := h.createBooking.Handle(ctx, cmd)
	if err != nil {
		return nil, mapDomainError(err)
	}

	return &pb.CreateBookingResponse{Booking: bookingToProto(b)}, nil
}

func (h *BookingHandler) GetBooking(ctx context.Context, req *pb.GetBookingRequest) (*pb.GetBookingResponse, error) {
	if req.GetBookingId() == "" {
		return nil, status.Error(codes.InvalidArgument, "booking_id is required")
	}

	result, err := h.getBooking.Handle(ctx, query.GetBooking{BookingID: req.GetBookingId()})
	if err != nil {
		return nil, mapDomainError(err)
	}

	history := make([]*pb.StatusHistoryEntry, 0, len(result.History))
	for _, e := range result.History {
		history = append(history, statusHistoryToProto(e))
	}

	return &pb.GetBookingResponse{
		Booking: bookingToProto(result.Booking),
		History: history,
	}, nil
}

func (h *BookingHandler) ListBookings(ctx context.Context, req *pb.ListBookingsRequest) (*pb.ListBookingsResponse, error) {
	if req.GetGuestId() == "" {
		return nil, status.Error(codes.InvalidArgument, "guest_id is required")
	}

	pageSize := int(req.GetPageSize())
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	offset, err := parsePageToken(req.GetPageToken())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid page_token")
	}

	q := query.ListBookings{
		GuestID:  req.GetGuestId(),
		PageSize: pageSize,
		Offset:   offset,
	}
	if s, ok := statusFilterFromProto(req.GetStatusFilter()); ok {
		q.Status = &s
	}
	if req.GetCheckInFrom() != nil {
		t := req.GetCheckInFrom().AsTime()
		q.CheckInFrom = &t
	}
	if req.GetCheckInTo() != nil {
		t := req.GetCheckInTo().AsTime()
		q.CheckInTo = &t
	}

	items, err := h.listBookings.Handle(ctx, q)
	if err != nil {
		return nil, mapDomainError(err)
	}

	summaries := make([]*pb.BookingSummary, 0, len(items))
	for _, p := range items {
		summaries = append(summaries, projectionItemToSummary(p))
	}

	resp := &pb.ListBookingsResponse{Bookings: summaries}
	if len(items) == pageSize {
		resp.NextPageToken = strconv.Itoa(offset + pageSize)
	}
	return resp, nil
}

func (h *BookingHandler) CancelBooking(ctx context.Context, req *pb.CancelBookingRequest) (*pb.CancelBookingResponse, error) {
	if req.GetBookingId() == "" {
		return nil, status.Error(codes.InvalidArgument, "booking_id is required")
	}

	b, err := h.cancelBooking.Handle(ctx, command.CancelBooking{BookingID: req.GetBookingId()})
	if err != nil {
		return nil, mapDomainError(err)
	}

	return &pb.CancelBookingResponse{Booking: bookingToProto(b)}, nil
}

func (h *BookingHandler) ApproveBooking(ctx context.Context, req *pb.ApproveBookingRequest) (*pb.ApproveBookingResponse, error) {
	if req.GetBookingId() == "" {
		return nil, status.Error(codes.InvalidArgument, "booking_id is required")
	}

	b, err := h.approveBooking.Handle(ctx, command.ApproveBooking{BookingID: req.GetBookingId()})
	if err != nil {
		return nil, mapDomainError(err)
	}

	return &pb.ApproveBookingResponse{Booking: bookingToProto(b)}, nil
}

func (h *BookingHandler) RejectBooking(ctx context.Context, req *pb.RejectBookingRequest) (*pb.RejectBookingResponse, error) {
	if req.GetBookingId() == "" {
		return nil, status.Error(codes.InvalidArgument, "booking_id is required")
	}

	b, err := h.rejectBooking.Handle(ctx, command.RejectBooking{
		BookingID: req.GetBookingId(),
		Reason:    req.GetReason(),
	})
	if err != nil {
		return nil, mapDomainError(err)
	}

	return &pb.RejectBookingResponse{Booking: bookingToProto(b)}, nil
}

func parsePageToken(token string) (int, error) {
	if token == "" {
		return 0, nil
	}
	offset, err := strconv.Atoi(token)
	if err != nil || offset < 0 {
		return 0, fmt.Errorf("invalid page token")
	}
	return offset, nil
}
