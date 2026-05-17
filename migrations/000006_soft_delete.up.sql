ALTER TABLE categories ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;
ALTER TABLE goods ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;
ALTER TABLE stores ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;
ALTER TABLE offers ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;
ALTER TABLE shopping_lists ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;

CREATE INDEX IF NOT EXISTS idx_categories_active_owner_updated
    ON categories (owner_id, updated_at) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_goods_active_owner_updated
    ON goods (owner_id, updated_at) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_stores_active_owner_updated
    ON stores (owner_id, updated_at) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_offers_active_owner_updated
    ON offers (owner_id, updated_at) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_lists_active_owner_updated
    ON shopping_lists (owner_id, updated_at) WHERE deleted_at IS NULL;
