package interceptors

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestLogging_CapturesMethodAndCode(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	ic := Logging()
	handler := func(_ context.Context, _ any) (any, error) {
		return "x", nil
	}
	ctx := WithRequestID(context.Background(), "rid-42")
	_, err := ic(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/svc/Echo"}, handler)
	if err != nil {
		t.Fatalf("interceptor err: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "/svc/Echo") {
		t.Fatalf("method missing in log: %s", out)
	}
	if !strings.Contains(out, "code=OK") {
		t.Fatalf("code missing or wrong: %s", out)
	}
	if !strings.Contains(out, "request_id=rid-42") {
		t.Fatalf("request_id missing: %s", out)
	}
}

func TestLogging_CapturesErrorAndCode(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	ic := Logging()
	handler := func(_ context.Context, _ any) (any, error) {
		return nil, status.Error(codes.NotFound, "missing")
	}
	_, err := ic(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/svc/M"}, handler)
	if status.Code(err) != codes.NotFound {
		t.Fatalf("want NotFound, got %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "code=NotFound") {
		t.Fatalf("code missing: %s", out)
	}
	if !strings.Contains(out, "error=") {
		t.Fatalf("error attr missing: %s", out)
	}
}

func TestLogging_WrapsPlainError(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	ic := Logging()
	handler := func(_ context.Context, _ any) (any, error) {
		return nil, errors.New("boom")
	}
	_, err := ic(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/svc/M"}, handler)
	if err == nil {
		t.Fatalf("expected err")
	}
	if !strings.Contains(buf.String(), "code=Unknown") {
		t.Fatalf("want Unknown code for plain error: %s", buf.String())
	}
}
