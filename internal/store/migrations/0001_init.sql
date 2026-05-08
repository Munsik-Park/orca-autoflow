-- Migration 0001: initial schema for orca (PRD §9.1)
-- This file is embedded via embed.FS and applied by applyMigrations.
-- It is idempotent: every CREATE statement uses IF NOT EXISTS.
-- NOTE: PRAGMAs (journal_mode, foreign_keys) are applied by Open() before
-- migrations run; they cannot be set inside a transaction in SQLite.

-- schema_migrations tracks applied migration versions.
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER PRIMARY KEY,
    applied_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- pods groups related runs sharing a base branch and configuration.
CREATE TABLE IF NOT EXISTS pods (
    id           TEXT    PRIMARY KEY,
    name         TEXT    NOT NULL UNIQUE,
    base_branch  TEXT    NOT NULL,
    adapter      TEXT    NOT NULL,
    config_json  TEXT    NOT NULL DEFAULT '{}',
    created_at   TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at   TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- runs represents a single coding agent execution within a pod.
-- short_ref is NOT UNIQUE: two runs may share the same 8-char prefix.
-- Ambiguity is detected at query time by ResolveShortRef.
CREATE TABLE IF NOT EXISTS runs (
    id            TEXT    PRIMARY KEY,
    short_ref     TEXT    NOT NULL,
    pod_id        TEXT    NOT NULL REFERENCES pods(id) ON DELETE CASCADE,
    status        TEXT    NOT NULL DEFAULT 'queued',
    prompt        TEXT    NOT NULL,
    worktree_path TEXT    NOT NULL DEFAULT '',
    adapter       TEXT    NOT NULL DEFAULT '',
    branch        TEXT    NOT NULL DEFAULT '',
    started_at    TEXT,
    completed_at  TEXT,
    created_at    TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at    TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- run_dependencies models prerequisite relationships between runs (DAG edges).
CREATE TABLE IF NOT EXISTS run_dependencies (
    run_id  TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    dep_id  TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    PRIMARY KEY (run_id, dep_id)
);

-- context_files lists extra files included in a run's agent context.
CREATE TABLE IF NOT EXISTS context_files (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id   TEXT    NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    path     TEXT    NOT NULL,
    included INTEGER NOT NULL DEFAULT 1
);

-- constraints records named constraint definitions attached to a pod or run.
CREATE TABLE IF NOT EXISTS constraints (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    pod_id     TEXT    NOT NULL REFERENCES pods(id) ON DELETE CASCADE,
    name       TEXT    NOT NULL,
    command    TEXT    NOT NULL,
    timeout_s  INTEGER NOT NULL DEFAULT 60,
    enabled    INTEGER NOT NULL DEFAULT 1
);

-- logs captures structured stdout/stderr lines from agent runs.
CREATE TABLE IF NOT EXISTS logs (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id     TEXT    NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    level      TEXT    NOT NULL DEFAULT 'info',
    message    TEXT    NOT NULL,
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- events is an immutable audit trail of all state transitions and system events.
CREATE TABLE IF NOT EXISTS events (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id     TEXT    NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    kind       TEXT    NOT NULL,
    payload    TEXT    NOT NULL DEFAULT '{}',
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- FTS5 virtual tables for full-text search on logs and runs.
CREATE VIRTUAL TABLE IF NOT EXISTS logs_fts USING fts5(
    message,
    content='logs',
    content_rowid='id'
);

CREATE VIRTUAL TABLE IF NOT EXISTS runs_fts USING fts5(
    prompt,
    content='runs',
    content_rowid='rowid'
);

-- Trigger: keep logs_fts in sync with logs inserts.
CREATE TRIGGER IF NOT EXISTS logs_ai
AFTER INSERT ON logs BEGIN
    INSERT INTO logs_fts(rowid, message) VALUES (new.id, new.message);
END;

-- Trigger: keep runs_fts in sync with runs inserts.
CREATE TRIGGER IF NOT EXISTS runs_ai
AFTER INSERT ON runs BEGIN
    INSERT INTO runs_fts(rowid, prompt) VALUES (new.rowid, new.prompt);
END;

-- Indexes for common query patterns.
CREATE INDEX IF NOT EXISTS idx_runs_pod_id      ON runs(pod_id);
CREATE INDEX IF NOT EXISTS idx_runs_status      ON runs(status);
CREATE INDEX IF NOT EXISTS idx_runs_created_at  ON runs(created_at);
CREATE INDEX IF NOT EXISTS idx_logs_run_id      ON logs(run_id);
CREATE INDEX IF NOT EXISTS idx_events_run_id    ON events(run_id);
CREATE INDEX IF NOT EXISTS idx_context_run_id   ON context_files(run_id);
CREATE INDEX IF NOT EXISTS idx_constraints_pod  ON constraints(pod_id);

-- Record this migration as applied.
INSERT OR IGNORE INTO schema_migrations(version) VALUES (1);
