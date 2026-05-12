package booking

import "testing"

func TestPaymentState_Lifecycle(t *testing.T) {
	t.Parallel()

	amount, err := NewMoney(30000, "RUB")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	ps := NewPaymentState("payment-1", "booking-1", amount)
	if ps.Status() != PaymentPending {
		t.Fatalf("initial status: want PENDING, got %s", ps.Status())
	}
	if ps.Amount().Amount() != 30000 {
		t.Fatalf("amount preserved")
	}
	if ps.ID() != "payment-1" || ps.BookingID() != "booking-1" {
		t.Fatalf("identifiers preserved")
	}
	createdAt := ps.CreatedAt()

	ps.MarkSucceeded("tx-42")
	if ps.Status() != PaymentSucceeded {
		t.Fatalf("status after MarkSucceeded: %s", ps.Status())
	}
	if ps.TransactionID() != "tx-42" {
		t.Fatalf("transaction id preserved")
	}
	if !ps.UpdatedAt().After(createdAt) && !ps.UpdatedAt().Equal(createdAt) {
		t.Fatalf("UpdatedAt must advance or equal CreatedAt")
	}
}

func TestPaymentState_MarkFailed(t *testing.T) {
	t.Parallel()

	amount, _ := NewMoney(1000, "USD")
	ps := NewPaymentState("payment-2", "booking-2", amount)
	ps.MarkFailed()
	if ps.Status() != PaymentFailed {
		t.Fatalf("status after MarkFailed: %s", ps.Status())
	}
}
