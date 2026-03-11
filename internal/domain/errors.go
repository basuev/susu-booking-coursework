package domain

import "errors"

var (
	ErrNotFound          = errors.New("not found")
	ErrInvalidArgument   = errors.New("invalid argument")
	ErrInvalidTransition = errors.New("invalid status transition")
	ErrAlreadyCancelled  = errors.New("booking already cancelled")
)
