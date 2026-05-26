ALTER TABLE outbox_message
    ADD COLUMN attempt_count INT          NOT NULL DEFAULT 0,
    ADD COLUMN last_error    TEXT,
    ADD COLUMN locked_at     TIMESTAMPTZ,
    ADD COLUMN next_retry_at TIMESTAMPTZ  NOT NULL DEFAULT now();

DROP INDEX IF EXISTS idx_outbox_status;
CREATE INDEX idx_outbox_ready ON outbox_message(next_retry_at)
    WHERE status = 'PENDING';
CREATE INDEX idx_outbox_inflight ON outbox_message(locked_at)
    WHERE status = 'IN_FLIGHT';
