CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE booking (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    guest_id      UUID        NOT NULL,
    status        TEXT        NOT NULL DEFAULT 'PENDING',
    total_amount  BIGINT      NOT NULL CHECK (total_amount > 0),
    currency      TEXT        NOT NULL,
    check_in      DATE        NOT NULL,
    check_out     DATE        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT check_dates CHECK (check_out > check_in)
);

CREATE INDEX idx_booking_guest ON booking(guest_id, created_at DESC);
CREATE INDEX idx_booking_status ON booking(status, created_at DESC);
