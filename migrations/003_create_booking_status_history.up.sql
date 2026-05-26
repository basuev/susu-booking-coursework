CREATE TABLE booking_status_history (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    booking_id  UUID        NOT NULL REFERENCES booking(id) ON DELETE CASCADE,
    old_status  TEXT        NOT NULL,
    new_status  TEXT        NOT NULL,
    reason      TEXT,
    changed_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_status_history_booking ON booking_status_history(booking_id, changed_at);
