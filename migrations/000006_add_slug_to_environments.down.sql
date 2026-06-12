DROP INDEX IF EXISTS idx_environments_product_slug;
ALTER TABLE environments DROP COLUMN IF EXISTS slug;
