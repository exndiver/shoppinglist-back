DROP INDEX IF EXISTS idx_list_items_active_owner_updated;
ALTER TABLE list_items DROP COLUMN IF EXISTS deleted_at;
