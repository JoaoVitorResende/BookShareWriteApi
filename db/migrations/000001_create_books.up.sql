CREATE TABLE IF NOT EXISTS books (
    id              UUID PRIMARY KEY,
    owner_id        UUID         NOT NULL,
    name            VARCHAR(200) NOT NULL,
    normalized_name VARCHAR(200) NOT NULL,
    author          VARCHAR(150) NOT NULL,
    description     TEXT         NOT NULL,
    pages           INT          NOT NULL CHECK (pages >= 50),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT uq_books_name_author UNIQUE (normalized_name, author)
);

CREATE INDEX IF NOT EXISTS idx_books_owner_id ON books (owner_id);
