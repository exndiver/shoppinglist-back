ALTER TABLE list_items ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;

CREATE INDEX IF NOT EXISTS idx_list_items_active_owner_updated
    ON list_items (owner_id, updated_at) WHERE deleted_at IS NULL;
