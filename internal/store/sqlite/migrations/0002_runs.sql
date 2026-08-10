CREATE TABLE runs (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    source_id       INTEGER NOT NULL REFERENCES sources (id),
    trigger         TEXT    NOT NULL,
    status          TEXT    NOT NULL,
    started_at      TEXT    NOT NULL,
    heartbeat_at    TEXT    NOT NULL,
    finished_at     TEXT,
    imported_count  INTEGER NOT NULL DEFAULT 0,
    duplicate_count INTEGER NOT NULL DEFAULT 0,
    failed_count    INTEGER NOT NULL DEFAULT 0,
    attempt         INTEGER NOT NULL DEFAULT 1,
    last_error      TEXT
) STRICT;

CREATE INDEX runs_source_started ON runs (source_id, started_at);

-- Recovery scans for non-terminal runs whose heartbeat has gone stale.
CREATE INDEX runs_status_heartbeat ON runs (status, heartbeat_at);

-- Events explain a run; they are append-only and never the source of truth for
-- run state. Purging them by age must stay cheap, hence the occurred_at index.
CREATE TABLE run_events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id      INTEGER NOT NULL REFERENCES runs (id),
    level       TEXT    NOT NULL,
    message     TEXT    NOT NULL,
    detail_json TEXT    NOT NULL DEFAULT '{}',
    occurred_at TEXT    NOT NULL
) STRICT;

CREATE INDEX run_events_run ON run_events (run_id, id);
CREATE INDEX run_events_occurred ON run_events (occurred_at);

-- Existing captures predate runs and keep a NULL run_id.
ALTER TABLE captures ADD COLUMN run_id INTEGER REFERENCES runs (id);

CREATE INDEX captures_run ON captures (run_id);
