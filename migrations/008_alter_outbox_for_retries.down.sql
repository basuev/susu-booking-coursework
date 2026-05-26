DROP INDEX IF EXISTS idx_outbox_inflight;
DROP INDEX IF EXISTS idx_outbox_ready;
CREATE INDEX idx_outbox_status ON outbox_message(status, created_at);

ALTER TABLE outbox_message
    DROP COLUMN next_retry_at,
    DROP COLUMN locked_at,
    DROP COLUMN last_error,
    DROP COLUMN attempt_count;
