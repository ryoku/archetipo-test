ALTER TABLE environments ADD COLUMN slug VARCHAR(255);
UPDATE environments SET slug =
    CASE
        WHEN trim('-' FROM lower(regexp_replace(name, '[^a-zA-Z0-9]+', '-', 'g'))) = ''
        THEN 'env-' || left(replace(id::text, '-', ''), 8)
        ELSE trim('-' FROM lower(regexp_replace(name, '[^a-zA-Z0-9]+', '-', 'g')))
    END;
ALTER TABLE environments ALTER COLUMN slug SET NOT NULL;
CREATE UNIQUE INDEX idx_environments_product_slug ON environments(product_id, slug);
