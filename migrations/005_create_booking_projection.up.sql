CREATE TABLE booking_projection (
    id          UUID PRIMARY KEY,
    guest_id    UUID        NOT NULL,
    hotel_id    UUID        NOT NULL,
    room_type   TEXT        NOT NULL,
    check_in    DATE        NOT NULL,
    check_out   DATE        NOT NULL,
    total_amount BIGINT     NOT NULL,
    currency    TEXT        NOT NULL,
    status      TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_projection_guest ON booking_projection(guest_id, updated_at DESC);
