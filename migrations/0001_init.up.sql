CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE companies (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT        NOT NULL,
    board_kind  TEXT        NOT NULL,
    board_slug  TEXT        NOT NULL,
    priority    SMALLINT    NOT NULL DEFAULT 2,
    tags        TEXT[]      NOT NULL DEFAULT '{}',
    notes       TEXT,
    active      BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (board_kind, board_slug)
);

CREATE TABLE listings (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id   UUID        NOT NULL REFERENCES companies(id),
    external_id  TEXT        NOT NULL,
    title        TEXT        NOT NULL,
    location     TEXT,
    url          TEXT        NOT NULL,
    description  TEXT        NOT NULL,
    raw_payload  JSONB       NOT NULL,
    posted_at    TIMESTAMPTZ,
    fetched_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status       TEXT        NOT NULL DEFAULT 'new',
    UNIQUE (company_id, external_id)
);

CREATE INDEX listings_status_idx ON listings(status);
