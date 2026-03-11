CREATE TABLE idempotency_key (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    request_hash  TEXT        NOT NULL,
    booking_id    UUID        NOT NULL REFERENCES booking(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_idempotency_hash ON idempotency_key(request_hash);
