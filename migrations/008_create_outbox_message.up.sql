CREATE TABLE outbox_message (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    topic       TEXT        NOT NULL,
    key         TEXT        NOT NULL,
    payload     JSONB       NOT NULL,
    status      TEXT        NOT NULL DEFAULT 'PENDING',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);

CREATE INDEX idx_outbox_status ON outbox_message(status, created_at);
