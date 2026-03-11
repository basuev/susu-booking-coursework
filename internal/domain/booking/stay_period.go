package booking

import (
	"fmt"
	"time"

	"github.com/basuev/susu-booking-coursework/internal/domain"
)

type StayPeriod struct {
	checkIn  time.Time
	checkOut time.Time
}

func NewStayPeriod(checkIn, checkOut time.Time) (StayPeriod, error) {
	checkIn = truncateToDate(checkIn)
	checkOut = truncateToDate(checkOut)

	if !checkOut.After(checkIn) {
		return StayPeriod{}, fmt.Errorf("%w: check-out must be after check-in", domain.ErrInvalidArgument)
	}
	return StayPeriod{checkIn: checkIn, checkOut: checkOut}, nil
}

func (s StayPeriod) CheckIn() time.Time  { return s.checkIn }
func (s StayPeriod) CheckOut() time.Time { return s.checkOut }

func (s StayPeriod) Nights() int {
	return int(s.checkOut.Sub(s.checkIn).Hours() / 24)
}

func truncateToDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
