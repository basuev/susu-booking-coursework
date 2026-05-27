package interceptors

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestRequestID_GeneratesWhenAbsent(t *testing.T) {
	t.Parallel()

	var seenCtx context.Context
	handler := func(c context.Context, _ any) (any, error) {
		seenCtx = c
		return nil, nil
	}

	ic := RequestID()
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs())
	_, err := ic(ctx, nil, &grpc.UnaryServerInfo{}, handler)
	if err != nil {
		t.Fatalf("interceptor err: %v", err)
	}
	id, ok := RequestIDFromContext(seenCtx)
	if !ok || id == "" {
		t.Fatalf("expected generated request id")
	}
}

func TestRequestID_PassesIncomingValue(t *testing.T) {
	t.Parallel()

	var seenCtx context.Context
	handler := func(c context.Context, _ any) (any, error) {
		seenCtx = c
		return nil, nil
	}

	ic := RequestID()
	md := metadata.Pairs(RequestIDMetadataKey, "abc-123")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	_, err := ic(ctx, nil, &grpc.UnaryServerInfo{}, handler)
	if err != nil {
		t.Fatalf("interceptor err: %v", err)
	}
	id, ok := RequestIDFromContext(seenCtx)
	if !ok || id != "abc-123" {
		t.Fatalf("got id=%q ok=%v", id, ok)
	}
}

func TestRequestID_EmptyIncomingValue(t *testing.T) {
	t.Parallel()

	var seenCtx context.Context
	handler := func(c context.Context, _ any) (any, error) {
		seenCtx = c
		return nil, nil
	}

	ic := RequestID()
	md := metadata.Pairs(RequestIDMetadataKey, "")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	_, err := ic(ctx, nil, &grpc.UnaryServerInfo{}, handler)
	if err != nil {
		t.Fatalf("interceptor err: %v", err)
	}
	id, ok := RequestIDFromContext(seenCtx)
	if !ok || id == "" {
		t.Fatalf("expected generated id when incoming was empty")
	}
}

func TestRequestIDContextHelpers(t *testing.T) {
	t.Parallel()

	if _, ok := RequestIDFromContext(context.Background()); ok {
		t.Fatalf("expected no id in empty ctx")
	}
	ctx := WithRequestID(context.Background(), "rid")
	if id, ok := RequestIDFromContext(ctx); !ok || id != "rid" {
		t.Fatalf("got id=%q ok=%v", id, ok)
	}
}
