-- MerlinCI core tables. PostgreSQL is the source of truth (deck slide 12).

-- pgvector, for retrieval. Nothing uses it until the memory layer lands.
CREATE EXTENSION IF NOT EXISTS vector;

-- The latest checkpoint of every run. A run resumes from here.
CREATE TABLE run_states (
    task_id    text        PRIMARY KEY,
    role       text        NOT NULL,
    status     text        NOT NULL CHECK (status IN ('running', 'awaiting_approval', 'succeeded', 'failed')),
    state      jsonb       NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX run_states_status ON run_states (status);

-- One row per tool call, written before the call runs and completed after.
-- A call whose row is 'done' is never made again; a call whose row is still
-- 'started' was interrupted, and nobody knows whether it happened.
CREATE TABLE effects (
    key         text        PRIMARY KEY,
    task_id     text        NOT NULL,
    tool        text        NOT NULL,
    action      text        NOT NULL,
    status      text        NOT NULL CHECK (status IN ('started', 'done')),
    result      text,
    sources     jsonb,
    started_at  timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz
);
CREATE INDEX effects_task ON effects (task_id);

-- Artifact records. The payload lives in object storage; record holds its
-- address, hash and provenance, and matches schemas/artifact.schema.json.
CREATE TABLE artifacts (
    id         text        PRIMARY KEY,
    kind       text        NOT NULL,
    task_id    text        NOT NULL,
    role       text        NOT NULL,
    record     jsonb       NOT NULL,
    created_at timestamptz NOT NULL
);
CREATE INDEX artifacts_task ON artifacts (task_id);
CREATE INDEX artifacts_kind ON artifacts (kind, created_at DESC);

-- Settled spend. Append-only.
CREATE TABLE budget_entries (
    id            bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    category      text        NOT NULL,
    amount_micros bigint      NOT NULL CHECK (amount_micros >= 0),
    at            timestamptz NOT NULL
);
CREATE INDEX budget_entries_category_at ON budget_entries (category, at);

-- Money held for calls in flight. A hold is deleted when its call settles or
-- is released. A hold left behind by a crash keeps counting against the
-- budget of the period it was made in: spend is over-counted, never under.
CREATE TABLE budget_reservations (
    id            bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    category      text        NOT NULL,
    amount_micros bigint      NOT NULL CHECK (amount_micros >= 0),
    at            timestamptz NOT NULL
);
CREATE INDEX budget_reservations_category_at ON budget_reservations (category, at);

-- Alerts already raised, so each threshold alerts once per period.
CREATE TABLE budget_alerts (
    category     text        NOT NULL,
    period       text        NOT NULL,
    threshold    text        NOT NULL,
    period_start timestamptz NOT NULL,
    raised_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (category, period, threshold, period_start)
);
