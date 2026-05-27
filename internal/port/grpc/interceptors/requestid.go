package interceptors

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const RequestIDMetadataKey = "x-request-id"

type ctxKey struct{}

var requestIDKey = ctxKey{}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func RequestIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(requestIDKey).(string)
	if !ok || v == "" {
		return "", false
	}
	return v, true
}

func RequestID() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		id := extractOrGenerate(ctx)
		ctx = WithRequestID(ctx, id)
		if err := grpc.SetHeader(ctx, metadata.Pairs(RequestIDMetadataKey, id)); err != nil {
			_ = err
		}
		return handler(ctx, req)
	}
}

func extractOrGenerate(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		if vs := md.Get(RequestIDMetadataKey); len(vs) > 0 && vs[0] != "" {
			return vs[0]
		}
	}
	return uuid.NewString()
}
