package booking

import (
	"errors"
	"testing"

	"gitverse.ru/basuev/susu-booking-coursework/internal/domain"
)

func TestNewMoney(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		amount   int64
		currency string
		wantErr  error
	}{
		{"positive amount", 19999, "RUB", nil},
		{"zero amount", 0, "RUB", domain.ErrInvalidArgument},
		{"negative amount", -1, "RUB", domain.ErrInvalidArgument},
		{"missing currency", 100, "", domain.ErrInvalidArgument},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m, err := NewMoney(c.amount, c.currency)
			if c.wantErr != nil {
				if !errors.Is(err, c.wantErr) {
					t.Fatalf("want %v, got %v", c.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if m.Amount() != c.amount || m.Currency() != c.currency {
				t.Fatalf("unexpected value: amount=%d currency=%q", m.Amount(), m.Currency())
			}
		})
	}
}

func TestMoney_Multiply(t *testing.T) {
	t.Parallel()
	m, err := NewMoney(19999, "RUB")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	got := m.Multiply(3)
	if got.Amount() != 59997 || got.Currency() != "RUB" {
		t.Fatalf("unexpected multiplied money: amount=%d currency=%q", got.Amount(), got.Currency())
	}
	if m.Amount() != 19999 {
		t.Fatalf("Multiply must not mutate the receiver, got amount=%d", m.Amount())
	}
}

func TestMoney_Equal(t *testing.T) {
	t.Parallel()
	a, _ := NewMoney(100, "RUB")
	b, _ := NewMoney(100, "RUB")
	c, _ := NewMoney(100, "USD")
	d, _ := NewMoney(200, "RUB")

	if !a.Equal(b) {
		t.Fatalf("identical money values must be equal")
	}
	if a.Equal(c) {
		t.Fatalf("different currencies must not be equal")
	}
	if a.Equal(d) {
		t.Fatalf("different amounts must not be equal")
	}
}
