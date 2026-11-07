CREATE TABLE IF NOT EXISTS outbox (
    id           UUID PRIMARY KEY,
    event_type   VARCHAR(100) NOT NULL,
    payload      JSONB        NOT NULL,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);

-- O worker busca só os eventos pendentes; o índice parcial deixa isso rápido.
CREATE INDEX IF NOT EXISTS idx_outbox_pending ON outbox (created_at) WHERE published_at IS NULL;
