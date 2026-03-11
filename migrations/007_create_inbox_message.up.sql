CREATE TABLE inbox_message (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    message_id  TEXT        NOT NULL,
    handler     TEXT        NOT NULL,
    payload     JSONB       NOT NULL,
    status      TEXT        NOT NULL DEFAULT 'NEW',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_inbox_message_id ON inbox_message(message_id);
