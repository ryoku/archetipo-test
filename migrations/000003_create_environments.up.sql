CREATE TABLE IF NOT EXISTS environments (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id   UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    name         VARCHAR(255) NOT NULL,
    type         VARCHAR(50) NOT NULL CHECK (type IN ('dev', 'integration', 'production')),
    overlay_path VARCHAR(500) NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (product_id, name)
);

CREATE INDEX IF NOT EXISTS idx_environments_product_id ON environments(product_id);
