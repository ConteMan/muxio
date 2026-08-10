CREATE TABLE sources (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT    NOT NULL UNIQUE,
    connector_kind  TEXT    NOT NULL,
    config_json     TEXT    NOT NULL DEFAULT '{}',
    checkpoint_json TEXT    NOT NULL DEFAULT '{}',
    enabled         INTEGER NOT NULL DEFAULT 1,
    created_at      TEXT    NOT NULL,
    updated_at      TEXT    NOT NULL
) STRICT;

-- Captures are immutable. A changed source produces a new row, never an update,
-- so the unique key spans the identity and the content version together.
CREATE TABLE captures (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    source_id     INTEGER NOT NULL REFERENCES sources (id),
    external_id   TEXT    NOT NULL,
    content_hash  TEXT    NOT NULL,
    title         TEXT    NOT NULL DEFAULT '',
    body          TEXT    NOT NULL DEFAULT '',
    mime_type     TEXT    NOT NULL DEFAULT 'text/plain',
    canonical_url TEXT    NOT NULL DEFAULT '',
    occurred_at   TEXT,
    captured_at   TEXT    NOT NULL,
    metadata_json TEXT    NOT NULL DEFAULT '{}',
    UNIQUE (source_id, external_id, content_hash)
) STRICT;

CREATE INDEX captures_source_external ON captures (source_id, external_id);
