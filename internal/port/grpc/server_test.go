package grpc

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "gitverse.ru/basuev/susu-booking-coursework/gen/go/booking/v1"
	"gitverse.ru/basuev/susu-booking-coursework/internal/app/command"
	"gitverse.ru/basuev/susu-booking-coursework/internal/app/query"
	"gitverse.ru/basuev/susu-booking-coursework/internal/domain"
	"gitverse.ru/basuev/susu-booking-coursework/internal/domain/booking"
)

type fakeBookingRepo struct {
	mu      sync.Mutex
	stored  map[string]*booking.Booking
	history map[string][]booking.StatusHistoryEntry
}

func newFakeBookingRepo() *fakeBookingRepo {
	return &fakeBookingRepo{
		stored:  map[string]*booking.Booking{},
		history: map[string][]booking.StatusHistoryEntry{},
	}
}

func (r *fakeBookingRepo) Save(_ context.Context, b *booking.Booking) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stored[b.ID()] = b
	return nil
}

func (r *fakeBookingRepo) FindByID(_ context.Context, id string) (*booking.Booking, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.stored[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return b, nil
}

func (r *fakeBookingRepo) GetStatusHistory(_ context.Context, id string) ([]booking.StatusHistoryEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.history[id], nil
}

type fakeIdempotencyStore struct {
	mu      sync.Mutex
	entries map[string]string
}

func newFakeIdempotency() *fakeIdempotencyStore {
	return &fakeIdempotencyStore{entries: map[string]string{}}
}

func (s *fakeIdempotencyStore) Lookup(_ context.Context, hash string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.entries[hash]
	return id, ok, nil
}

func (s *fakeIdempotencyStore) Save(_ context.Context, hash, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.entries[hash]; exists {
		return command.ErrIdempotencyConflict
	}
	s.entries[hash] = id
	return nil
}

type inlineTxManager struct{}

func (inlineTxManager) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type fakeProjectionReader struct {
	items []query.ProjectionItem
	err   error
	last  query.ProjectionFilter
}

func (r *fakeProjectionReader) ListByGuest(_ context.Context, f query.ProjectionFilter) ([]query.ProjectionItem, error) {
	r.last = f
	if r.err != nil {
		return nil, r.err
	}
	return r.items, nil
}

type harness struct {
	conn      *grpc.ClientConn
	client    pb.BookingServiceClient
	server    *grpc.Server
	repo      *fakeBookingRepo
	idem      *fakeIdempotencyStore
	reader    *fakeProjectionReader
	cleanupFn func()
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	repo := newFakeBookingRepo()
	idem := newFakeIdempotency()
	reader := &fakeProjectionReader{}

	createH := command.NewCreateBookingHandler(repo, idem, inlineTxManager{})
	cancelH := command.NewCancelBookingHandler(repo)
	approveH := command.NewApproveBookingHandler(repo)
	rejectH := command.NewRejectBookingHandler(repo)
	getH := query.NewGetBookingHandler(repo)
	listH := query.NewListBookingsHandler(reader)

	handler := NewBookingHandler(createH, cancelH, approveH, rejectH, getH, listH)

	lis := bufconn.Listen(1 << 20)
	gs := grpc.NewServer()
	pb.RegisterBookingServiceServer(gs, handler)

	go func() {
		_ = gs.Serve(lis)
	}()

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}
	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	h := &harness{
		conn:   conn,
		client: pb.NewBookingServiceClient(conn),
		server: gs,
		repo:   repo,
		idem:   idem,
		reader: reader,
	}
	h.cleanupFn = func() {
		_ = conn.Close()
		gs.Stop()
		_ = lis.Close()
	}
	t.Cleanup(h.cleanupFn)
	return h
}

func validCreateReq(key string) *pb.CreateBookingRequest {
	return &pb.CreateBookingRequest{
		IdempotencyKey: key,
		GuestId:        "guest-1",
		Offer: &pb.OfferSnapshot{
			OfferId:       "offer-1",
			HotelId:       "hotel-1",
			RoomType:      "STANDARD",
			PricePerNight: &pb.Money{Amount: "100.00", Currency: "RUB"},
		},
		CheckIn:  timestamppb.New(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)),
		CheckOut: timestamppb.New(time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC)),
	}
}

func TestCreateBooking_Success(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := h.client.CreateBooking(ctx, validCreateReq("k-1"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if res.GetBooking().GetId() == "" {
		t.Fatalf("empty booking id")
	}
	if res.GetBooking().GetStatus() != pb.BookingStatus_BOOKING_STATUS_PENDING {
		t.Fatalf("want PENDING, got %s", res.GetBooking().GetStatus())
	}
}

func TestCreateBooking_MissingIdempotencyKey(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := validCreateReq("")
	_, err := h.client.CreateBooking(ctx, req)
	if err == nil {
		t.Fatalf("expected error")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument, got %s", status.Code(err))
	}
}

func TestCreateBooking_IdempotencyReturnsSameID(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	first, err := h.client.CreateBooking(ctx, validCreateReq("same-key"))
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := h.client.CreateBooking(ctx, validCreateReq("same-key"))
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if first.GetBooking().GetId() != second.GetBooking().GetId() {
		t.Fatalf("want same id, got %s vs %s", first.GetBooking().GetId(), second.GetBooking().GetId())
	}
}

func TestGetBooking_Success(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	created, err := h.client.CreateBooking(ctx, validCreateReq("k"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := h.client.GetBooking(ctx, &pb.GetBookingRequest{BookingId: created.GetBooking().GetId()})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.GetBooking().GetId() != created.GetBooking().GetId() {
		t.Fatalf("id mismatch")
	}
}

func TestGetBooking_NotFound(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := h.client.GetBooking(ctx, &pb.GetBookingRequest{BookingId: "11111111-1111-1111-1111-111111111111"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if status.Code(err) != codes.NotFound {
		t.Fatalf("want NotFound, got %s", status.Code(err))
	}
}

func TestGetBooking_MissingID(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := h.client.GetBooking(ctx, &pb.GetBookingRequest{BookingId: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument, got %s", status.Code(err))
	}
}

func TestListBookings_Success(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	h.reader.items = []query.ProjectionItem{
		{ID: "b1", GuestID: "g1", HotelID: "h1", Status: booking.StatusPending, TotalMinor: 30000, Currency: "RUB",
			CheckIn: time.Now(), CheckOut: time.Now().Add(24 * time.Hour), CreatedAt: time.Now()},
		{ID: "b2", GuestID: "g1", HotelID: "h1", Status: booking.StatusApproved, TotalMinor: 50000, Currency: "RUB",
			CheckIn: time.Now(), CheckOut: time.Now().Add(24 * time.Hour), CreatedAt: time.Now()},
	}

	res, err := h.client.ListBookings(ctx, &pb.ListBookingsRequest{
		GuestId:       "g1",
		StatusFilter:  pb.BookingStatus_BOOKING_STATUS_APPROVED,
		PageSize:      10,
		CheckInFrom:   timestamppb.New(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)),
		CheckInTo:     timestamppb.New(time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(res.GetBookings()) != 2 {
		t.Fatalf("want 2 bookings, got %d", len(res.GetBookings()))
	}
	if h.reader.last.Status == nil || *h.reader.last.Status != booking.StatusApproved {
		t.Fatalf("status filter not forwarded: %#v", h.reader.last.Status)
	}
	if h.reader.last.CheckInFrom == nil || h.reader.last.CheckInTo == nil {
		t.Fatalf("date filters not forwarded")
	}
}

func TestListBookings_MissingGuest(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := h.client.ListBookings(ctx, &pb.ListBookingsRequest{GuestId: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument, got %s", status.Code(err))
	}
}

func TestListBookings_InvalidPageToken(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := h.client.ListBookings(ctx, &pb.ListBookingsRequest{
		GuestId:   "g1",
		PageToken: "not-a-number",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument, got %s", status.Code(err))
	}
}

func TestCancelBooking_Success(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	created, err := h.client.CreateBooking(ctx, validCreateReq("k-cancel"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	res, err := h.client.CancelBooking(ctx, &pb.CancelBookingRequest{BookingId: created.GetBooking().GetId()})
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if res.GetBooking().GetStatus() != pb.BookingStatus_BOOKING_STATUS_CANCELLED {
		t.Fatalf("want CANCELLED, got %s", res.GetBooking().GetStatus())
	}
}

func TestCancelBooking_SecondCancelFails(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	created, err := h.client.CreateBooking(ctx, validCreateReq("k-cancel2"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := h.client.CancelBooking(ctx, &pb.CancelBookingRequest{BookingId: created.GetBooking().GetId()}); err != nil {
		t.Fatalf("first cancel: %v", err)
	}
	_, err = h.client.CancelBooking(ctx, &pb.CancelBookingRequest{BookingId: created.GetBooking().GetId()})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("want FailedPrecondition, got %s", status.Code(err))
	}
}

func TestApproveBooking_Success(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	created, err := h.client.CreateBooking(ctx, validCreateReq("k-app"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	res, err := h.client.ApproveBooking(ctx, &pb.ApproveBookingRequest{BookingId: created.GetBooking().GetId()})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if res.GetBooking().GetStatus() != pb.BookingStatus_BOOKING_STATUS_APPROVED {
		t.Fatalf("want APPROVED, got %s", res.GetBooking().GetStatus())
	}
}

func TestRejectBooking_Success(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	created, err := h.client.CreateBooking(ctx, validCreateReq("k-rej"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	res, err := h.client.RejectBooking(ctx, &pb.RejectBookingRequest{BookingId: created.GetBooking().GetId(), Reason: "no rooms"})
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if res.GetBooking().GetStatus() != pb.BookingStatus_BOOKING_STATUS_REJECTED {
		t.Fatalf("want REJECTED, got %s", res.GetBooking().GetStatus())
	}
}

func TestCreateBooking_MissingDates(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := validCreateReq("k")
	req.CheckIn = nil
	_, err := h.client.CreateBooking(ctx, req)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument, got %s", status.Code(err))
	}
}

func TestApproveBooking_MissingID(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := h.client.ApproveBooking(ctx, &pb.ApproveBookingRequest{BookingId: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument, got %s", status.Code(err))
	}
}

func TestRejectBooking_MissingID(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := h.client.RejectBooking(ctx, &pb.RejectBookingRequest{BookingId: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument, got %s", status.Code(err))
	}
}

func TestCancelBooking_MissingID(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := h.client.CancelBooking(ctx, &pb.CancelBookingRequest{BookingId: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument, got %s", status.Code(err))
	}
}

func TestGetBooking_WithHistory(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	created, err := h.client.CreateBooking(ctx, validCreateReq("k-hist"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	id := created.GetBooking().GetId()
	h.repo.mu.Lock()
	h.repo.history[id] = []booking.StatusHistoryEntry{
		{OldStatus: booking.StatusPending, NewStatus: booking.StatusApproved, ChangedAt: time.Now()},
	}
	h.repo.mu.Unlock()

	got, err := h.client.GetBooking(ctx, &pb.GetBookingRequest{BookingId: id})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got.GetHistory()) != 1 {
		t.Fatalf("want 1 history entry, got %d", len(got.GetHistory()))
	}
}

func TestServerLifecycle(t *testing.T) {
	t.Parallel()
	repo := newFakeBookingRepo()
	idem := newFakeIdempotency()
	reader := &fakeProjectionReader{}
	handler := NewBookingHandler(
		command.NewCreateBookingHandler(repo, idem, inlineTxManager{}),
		command.NewCancelBookingHandler(repo),
		command.NewApproveBookingHandler(repo),
		command.NewRejectBookingHandler(repo),
		query.NewGetBookingHandler(repo),
		query.NewListBookingsHandler(reader),
	)
	srv, err := NewServer("127.0.0.1:0", handler)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	go func() { _ = srv.Start() }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}

func TestListBookings_ReaderErrorMapsToInternal(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	h.reader.err = errors.New("boom")
	_, err := h.client.ListBookings(ctx, &pb.ListBookingsRequest{GuestId: "g1"})
	if status.Code(err) != codes.Internal {
		t.Fatalf("want Internal, got %s", status.Code(err))
	}
}
