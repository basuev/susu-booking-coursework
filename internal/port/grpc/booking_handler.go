package grpc

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/basuev/susu-booking-coursework/internal/app/command"
	"github.com/basuev/susu-booking-coursework/internal/app/query"
)

type BookingHandler struct {
	createBooking *command.CreateBookingHandler
	cancelBooking *command.CancelBookingHandler
	approveBooking *command.ApproveBookingHandler
	rejectBooking  *command.RejectBookingHandler
	getBooking    *query.GetBookingHandler
	listBookings  *query.ListBookingsHandler
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
	// Registration will be implemented when generated protobuf code is available.
	_ = gs
}

func (h *BookingHandler) CreateBooking(_ context.Context, _ any) (any, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (h *BookingHandler) GetBooking(_ context.Context, _ any) (any, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (h *BookingHandler) ListBookings(_ context.Context, _ any) (any, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (h *BookingHandler) CancelBooking(_ context.Context, _ any) (any, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (h *BookingHandler) ApproveBooking(_ context.Context, _ any) (any, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (h *BookingHandler) RejectBooking(_ context.Context, _ any) (any, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}
