package booking

import (
	"fmt"

	"github.com/tbasuev/susu-booking-coursework/internal/domain"
)

type Money struct {
	amount   int64
	currency string
}

func NewMoney(amount int64, currency string) (Money, error) {
	if amount <= 0 {
		return Money{}, fmt.Errorf("%w: amount must be positive", domain.ErrInvalidArgument)
	}
	if currency == "" {
		return Money{}, fmt.Errorf("%w: currency is required", domain.ErrInvalidArgument)
	}
	return Money{amount: amount, currency: currency}, nil
}

func (m Money) Amount() int64    { return m.amount }
func (m Money) Currency() string { return m.currency }

func (m Money) Multiply(n int) Money {
	return Money{amount: m.amount * int64(n), currency: m.currency}
}

func (m Money) Equal(other Money) bool {
	return m.amount == other.amount && m.currency == other.currency
}
