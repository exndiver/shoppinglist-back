DROP INDEX IF EXISTS idx_lists_active_owner_updated;
DROP INDEX IF EXISTS idx_offers_active_owner_updated;
DROP INDEX IF EXISTS idx_stores_active_owner_updated;
DROP INDEX IF EXISTS idx_goods_active_owner_updated;
DROP INDEX IF EXISTS idx_categories_active_owner_updated;

ALTER TABLE shopping_lists DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE offers DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE stores DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE goods DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE categories DROP COLUMN IF EXISTS deleted_at;
