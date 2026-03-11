package id

import "github.com/google/uuid"

func New() string {
	return uuid.New().String()
}

func Validate(s string) error {
	_, err := uuid.Parse(s)
	return err
}
