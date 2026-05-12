package booking

import (
	"errors"
	"testing"

	"github.com/basuev/susu-booking-coursework/internal/domain"
)

func TestStatus_CanTransitionTo(t *testing.T) {
	t.Parallel()

	cases := []struct {
		from   Status
		to     Status
		wantOk bool
	}{
		{StatusPending, StatusConfirmed, true},
		{StatusPending, StatusApproved, true},
		{StatusPending, StatusRejected, true},
		{StatusPending, StatusCancelled, true},
		{StatusPending, StatusPending, false},

		{StatusConfirmed, StatusApproved, true},
		{StatusConfirmed, StatusRejected, true},
		{StatusConfirmed, StatusCancelled, true},
		{StatusConfirmed, StatusPending, false},
		{StatusConfirmed, StatusConfirmed, false},

		{StatusApproved, StatusCancelled, false},
		{StatusApproved, StatusRejected, false},
		{StatusApproved, StatusPending, false},

		{StatusRejected, StatusApproved, false},
		{StatusRejected, StatusCancelled, false},

		{StatusCancelled, StatusApproved, false},
		{StatusCancelled, StatusPending, false},
	}

	for _, c := range cases {
		c := c
		t.Run(string(c.from)+"_to_"+string(c.to), func(t *testing.T) {
			t.Parallel()
			got := c.from.CanTransitionTo(c.to)
			if got != c.wantOk {
				t.Fatalf("%s -> %s: want %v, got %v", c.from, c.to, c.wantOk, got)
			}
		})
	}
}

func TestStatus_TransitionTo(t *testing.T) {
	t.Parallel()

	t.Run("valid transition returns target", func(t *testing.T) {
		t.Parallel()
		got, err := StatusPending.TransitionTo(StatusApproved)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != StatusApproved {
			t.Fatalf("want %s, got %s", StatusApproved, got)
		}
	})

	t.Run("invalid transition returns ErrInvalidTransition", func(t *testing.T) {
		t.Parallel()
		got, err := StatusApproved.TransitionTo(StatusCancelled)
		if !errors.Is(err, domain.ErrInvalidTransition) {
			t.Fatalf("want ErrInvalidTransition, got %v", err)
		}
		if got != StatusApproved {
			t.Fatalf("status must remain unchanged on invalid transition, got %s", got)
		}
	})
}

func TestStatus_String(t *testing.T) {
	t.Parallel()
	if StatusPending.String() != "PENDING" {
		t.Fatalf("unexpected String value: %s", StatusPending.String())
	}
}
