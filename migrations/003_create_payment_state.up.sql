CREATE TABLE payment_state (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    booking_id      UUID        NOT NULL REFERENCES booking(id) ON DELETE CASCADE,
    status          TEXT        NOT NULL DEFAULT 'PENDING',
    amount          BIGINT      NOT NULL CHECK (amount > 0),
    currency        TEXT        NOT NULL,
    transaction_id  TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_payment_booking UNIQUE (booking_id)
);
