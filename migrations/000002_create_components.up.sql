CREATE TABLE IF NOT EXISTS components (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id     UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    name           VARCHAR(255) NOT NULL,
    slug           VARCHAR(255) NOT NULL,
    gcr_image_path VARCHAR(500) NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (product_id, slug)
);

CREATE INDEX IF NOT EXISTS idx_components_product_id ON components(product_id);
