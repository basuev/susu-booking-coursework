package interceptors

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRecovery(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		handler grpc.UnaryHandler
		wantErr codes.Code
		wantOK  bool
	}{
		{
			name: "no panic passes through",
			handler: func(_ context.Context, _ any) (any, error) {
				return "ok", nil
			},
			wantOK: true,
		},
		{
			name: "panic with string becomes Internal",
			handler: func(_ context.Context, _ any) (any, error) {
				panic("boom")
			},
			wantErr: codes.Internal,
		},
		{
			name: "panic with error becomes Internal",
			handler: func(_ context.Context, _ any) (any, error) {
				panic(errors.New("oops"))
			},
			wantErr: codes.Internal,
		},
		{
			name: "handler returning error passes through",
			handler: func(_ context.Context, _ any) (any, error) {
				return nil, status.Error(codes.NotFound, "missing")
			},
			wantErr: codes.NotFound,
		},
	}

	rec := Recovery()
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			info := &grpc.UnaryServerInfo{FullMethod: "/svc/Method"}
			resp, err := rec(context.Background(), nil, info, tc.handler)
			if tc.wantOK {
				if err != nil {
					t.Fatalf("want no error, got %v", err)
				}
				if resp != "ok" {
					t.Fatalf("resp = %v", resp)
				}
				return
			}
			if status.Code(err) != tc.wantErr {
				t.Fatalf("want %s, got %s (err=%v)", tc.wantErr, status.Code(err), err)
			}
		})
	}
}
