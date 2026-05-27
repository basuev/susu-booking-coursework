package interceptors

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

func Logging() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)

		method := ""
		if info != nil {
			method = info.FullMethod
		}
		code := status.Code(err)
		reqID, _ := RequestIDFromContext(ctx)

		attrs := []any{
			"method", method,
			"code", code.String(),
			"duration_ms", duration.Milliseconds(),
		}
		if reqID != "" {
			attrs = append(attrs, "request_id", reqID)
		}
		if err != nil {
			attrs = append(attrs, "error", err.Error())
		}
		slog.LogAttrs(ctx, slog.LevelInfo, "grpc call", toAttrs(attrs)...)
		return resp, err
	}
}

func toAttrs(kv []any) []slog.Attr {
	attrs := make([]slog.Attr, 0, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		key, ok := kv[i].(string)
		if !ok {
			continue
		}
		attrs = append(attrs, slog.Any(key, kv[i+1]))
	}
	return attrs
}
