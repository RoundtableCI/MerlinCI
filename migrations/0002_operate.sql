-- What a person needs to operate MerlinCI: the tasks that were asked for, a
-- permanent record of what each run did, and the controls to pause it.

-- A task as it was requested. Its progress lives in run_states.
CREATE TABLE tasks (
    id         text        PRIMARY KEY,
    role       text        NOT NULL,
    input      jsonb       NOT NULL,
    created_by text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX tasks_created_at ON tasks (created_at DESC);

-- Everything a run did and every decision made about it, in order.
-- Append-only: rows are never updated or deleted.
CREATE TABLE events (
    id      bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    task_id text        NOT NULL,
    role    text        NOT NULL,
    step    integer     NOT NULL,
    type    text        NOT NULL,
    data    jsonb       NOT NULL DEFAULT '{}',
    at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX events_task ON events (task_id, id);

-- Switches a person can flip while MerlinCI is running, such as "paused".
CREATE TABLE controls (
    key        text        PRIMARY KEY,
    value      text        NOT NULL,
    updated_by text        NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- The channel a tool call acted on, for "at most N per day on this channel".
ALTER TABLE effects ADD COLUMN channel text NOT NULL DEFAULT '';
CREATE INDEX effects_action_channel ON effects (action, channel, started_at);
