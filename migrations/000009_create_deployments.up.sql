CREATE TABLE deployments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id),
    environment_id UUID NOT NULL REFERENCES environments(id),
    actor_display_name TEXT NOT NULL,
    component_name TEXT NOT NULL,
    environment_name TEXT NOT NULL,
    tag TEXT NOT NULL,
    deployed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    commit_sha TEXT,
    outcome TEXT NOT NULL CHECK (outcome IN ('success', 'failure')),
    error_message TEXT
);
CREATE INDEX deployments_product_id_idx ON deployments(product_id, deployed_at DESC);
CREATE INDEX deployments_deployed_at_idx ON deployments(deployed_at DESC);
