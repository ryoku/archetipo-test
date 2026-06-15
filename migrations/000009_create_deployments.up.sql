CREATE TABLE deployments (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_sub      TEXT        NOT NULL,
    product_id     UUID        NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    environment_id UUID        NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    workload       TEXT        NOT NULL,
    tag            TEXT        NOT NULL,
    deployed_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    commit_sha     TEXT        NOT NULL DEFAULT '',
    outcome        VARCHAR(16) NOT NULL CHECK (outcome IN ('success', 'failure')),
    error_message  TEXT        NOT NULL DEFAULT ''
);
