package interceptors

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
)

func TestTimeout(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		ctx       func() (context.Context, context.CancelFunc)
		timeout   time.Duration
		wantHasDL bool
		within    time.Duration
	}{
		{
			name:      "no deadline gets one added",
			ctx:       func() (context.Context, context.CancelFunc) { return context.Background(), func() {} },
			timeout:   200 * time.Millisecond,
			wantHasDL: true,
			within:    200 * time.Millisecond,
		},
		{
			name: "existing deadline preserved",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithDeadline(context.Background(), time.Now().Add(50*time.Millisecond))
			},
			timeout:   10 * time.Second,
			wantHasDL: true,
			within:    100 * time.Millisecond,
		},
		{
			name:      "zero timeout is a no-op",
			ctx:       func() (context.Context, context.CancelFunc) { return context.Background(), func() {} },
			timeout:   0,
			wantHasDL: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := tc.ctx()
			defer cancel()
			var seenCtx context.Context
			handler := func(c context.Context, _ any) (any, error) {
				seenCtx = c
				return nil, nil
			}
			interceptor := Timeout(tc.timeout)
			_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)
			if err != nil {
				t.Fatalf("interceptor err: %v", err)
			}
			dl, hasDL := seenCtx.Deadline()
			if hasDL != tc.wantHasDL {
				t.Fatalf("hasDeadline = %v, want %v", hasDL, tc.wantHasDL)
			}
			if tc.wantHasDL && tc.within > 0 {
				if time.Until(dl) > tc.within+50*time.Millisecond {
					t.Fatalf("deadline too far: %s", time.Until(dl))
				}
			}
		})
	}
}
