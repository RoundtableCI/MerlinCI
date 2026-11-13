-- A campaign: one run of a workflow template, made of several tasks.
CREATE TABLE campaigns (
    id         text        PRIMARY KEY,
    workflow   text        NOT NULL,
    input      jsonb       NOT NULL,
    status     text        NOT NULL CHECK (status IN ('running', 'succeeded', 'failed')),
    -- Where each step stands, as the campaign workflow last reported it.
    state      jsonb       NOT NULL,
    created_by text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX campaigns_created_at ON campaigns (created_at DESC);
