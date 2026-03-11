package booking

import (
	"fmt"

	"github.com/tbasuev/susu-booking-coursework/internal/domain"
)

type Status string

const (
	StatusPending   Status = "PENDING"
	StatusConfirmed Status = "CONFIRMED"
	StatusApproved  Status = "APPROVED"
	StatusRejected  Status = "REJECTED"
	StatusCancelled Status = "CANCELLED"
)

var validTransitions = map[Status][]Status{
	StatusPending:   {StatusConfirmed, StatusCancelled},
	StatusConfirmed: {StatusApproved, StatusRejected, StatusCancelled},
}

func (s Status) CanTransitionTo(target Status) bool {
	targets, ok := validTransitions[s]
	if !ok {
		return false
	}
	for _, t := range targets {
		if t == target {
			return true
		}
	}
	return false
}

func (s Status) TransitionTo(target Status) (Status, error) {
	if !s.CanTransitionTo(target) {
		return s, fmt.Errorf("%w: cannot transition from %s to %s", domain.ErrInvalidTransition, s, target)
	}
	return target, nil
}

func (s Status) String() string {
	return string(s)
}
