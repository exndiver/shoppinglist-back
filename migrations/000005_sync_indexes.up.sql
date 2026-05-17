CREATE INDEX IF NOT EXISTS idx_categories_owner_updated
    ON categories (owner_id, updated_at);

CREATE INDEX IF NOT EXISTS idx_categories_owner_normalized
    ON categories (owner_id, normalized_name);

CREATE INDEX IF NOT EXISTS idx_goods_category
    ON goods (category_id);

CREATE INDEX IF NOT EXISTS idx_goods_owner_updated
    ON goods (owner_id, updated_at);

CREATE INDEX IF NOT EXISTS idx_stores_owner_updated
    ON stores (owner_id, updated_at);

CREATE INDEX IF NOT EXISTS idx_offers_owner_updated
    ON offers (owner_id, updated_at);

CREATE INDEX IF NOT EXISTS idx_lists_owner_updated
    ON shopping_lists (owner_id, updated_at);

CREATE INDEX IF NOT EXISTS idx_list_items_owner_updated
    ON list_items (owner_id, updated_at);

CREATE INDEX IF NOT EXISTS idx_price_records_owner_created
    ON price_records (owner_id, created_at);
