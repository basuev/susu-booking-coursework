package booking

import "time"

type PaymentStatus string

const (
	PaymentPending   PaymentStatus = "PENDING"
	PaymentSucceeded PaymentStatus = "SUCCEEDED"
	PaymentFailed    PaymentStatus = "FAILED"
)

type PaymentState struct {
	id            string
	bookingID     string
	status        PaymentStatus
	amount        Money
	transactionID string
	createdAt     time.Time
	updatedAt     time.Time
}

func NewPaymentState(id, bookingID string, amount Money) PaymentState {
	now := time.Now()
	return PaymentState{
		id:        id,
		bookingID: bookingID,
		status:    PaymentPending,
		amount:    amount,
		createdAt: now,
		updatedAt: now,
	}
}

func (p *PaymentState) MarkSucceeded(transactionID string) {
	p.status = PaymentSucceeded
	p.transactionID = transactionID
	p.updatedAt = time.Now()
}

func (p *PaymentState) MarkFailed() {
	p.status = PaymentFailed
	p.updatedAt = time.Now()
}

func (p PaymentState) ID() string              { return p.id }
func (p PaymentState) BookingID() string       { return p.bookingID }
func (p PaymentState) Status() PaymentStatus   { return p.status }
func (p PaymentState) Amount() Money           { return p.amount }
func (p PaymentState) TransactionID() string   { return p.transactionID }
func (p PaymentState) CreatedAt() time.Time    { return p.createdAt }
func (p PaymentState) UpdatedAt() time.Time    { return p.updatedAt }
