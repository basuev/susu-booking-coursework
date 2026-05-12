package booking

import (
	"errors"
	"testing"
	"time"

	"github.com/basuev/susu-booking-coursework/internal/domain"
)

func TestNewStayPeriod(t *testing.T) {
	t.Parallel()

	day := func(y, m, d int) time.Time {
		return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	}

	cases := []struct {
		name     string
		in, out  time.Time
		wantErr  error
		wantStay int
	}{
		{"single night", day(2026, 6, 1), day(2026, 6, 2), nil, 1},
		{"three nights", day(2026, 6, 1), day(2026, 6, 4), nil, 3},
		{"same day", day(2026, 6, 1), day(2026, 6, 1), domain.ErrInvalidArgument, 0},
		{"check-out before check-in", day(2026, 6, 5), day(2026, 6, 1), domain.ErrInvalidArgument, 0},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			p, err := NewStayPeriod(c.in, c.out)
			if c.wantErr != nil {
				if !errors.Is(err, c.wantErr) {
					t.Fatalf("want %v, got %v", c.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.Nights() != c.wantStay {
				t.Fatalf("Nights(): want %d, got %d", c.wantStay, p.Nights())
			}
		})
	}
}

func TestStayPeriod_TruncatesTime(t *testing.T) {
	t.Parallel()

	in := time.Date(2026, 6, 1, 14, 30, 0, 0, time.UTC)
	out := time.Date(2026, 6, 3, 11, 0, 0, 0, time.UTC)

	p, err := NewStayPeriod(in, out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if h, m, s := p.CheckIn().Clock(); h != 0 || m != 0 || s != 0 {
		t.Fatalf("CheckIn must be truncated to date, got %v", p.CheckIn())
	}
	if h, m, s := p.CheckOut().Clock(); h != 0 || m != 0 || s != 0 {
		t.Fatalf("CheckOut must be truncated to date, got %v", p.CheckOut())
	}
	if p.Nights() != 2 {
		t.Fatalf("Nights: want 2, got %d", p.Nights())
	}
}
