CREATE TABLE deployment_locks (
    product_id   UUID        NOT NULL REFERENCES products(id),
    env_id       UUID        NOT NULL REFERENCES environments(id),
    lock_holder  TEXT        NOT NULL,
    locked_since TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (product_id, env_id)
);
