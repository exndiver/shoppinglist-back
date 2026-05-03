-- Domain: owner-scoped shopping catalog + lists + price history

CREATE TABLE products (
    id UUID PRIMARY KEY,
    owner_id UUID NOT NULL,
    name TEXT NOT NULL,
    normalized_name TEXT NOT NULL,
    merged_into UUID REFERENCES products (id),
    created_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT products_no_self_merge CHECK (merged_into IS DISTINCT FROM id)
);

CREATE INDEX idx_products_owner_norm_canonical ON products (owner_id, normalized_name)
    WHERE merged_into IS NULL;
CREATE INDEX idx_products_owner ON products (owner_id);

CREATE TABLE stores (
    id UUID PRIMARY KEY,
    owner_id UUID NOT NULL,
    name TEXT NOT NULL,
    normalized_name TEXT NOT NULL,
    merged_into UUID REFERENCES stores (id),
    created_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT stores_no_self_merge CHECK (merged_into IS DISTINCT FROM id)
);

CREATE INDEX idx_stores_owner_norm_canonical ON stores (owner_id, normalized_name)
    WHERE merged_into IS NULL;
CREATE INDEX idx_stores_owner ON stores (owner_id);

CREATE TABLE offers (
    id UUID PRIMARY KEY,
    owner_id UUID NOT NULL,
    product_id UUID NOT NULL REFERENCES products (id),
    store_id UUID NOT NULL REFERENCES stores (id),
    created_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (owner_id, product_id, store_id)
);

CREATE INDEX idx_offers_owner_product ON offers (owner_id, product_id);
CREATE INDEX idx_offers_offer_store ON offers (owner_id, store_id);

CREATE TABLE price_records (
    id UUID PRIMARY KEY,
    owner_id UUID NOT NULL,
    offer_id UUID NOT NULL REFERENCES offers (id) ON DELETE CASCADE,
    price NUMERIC(14, 4) NOT NULL,
    pack_size NUMERIC(14, 4),
    unit TEXT,
    recorded_at TIMESTAMPTZ NOT NULL,
    recorded_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_price_offer_recorded ON price_records (offer_id, recorded_at DESC);

CREATE TABLE shopping_lists (
    id UUID PRIMARY KEY,
    owner_id UUID NOT NULL,
    name TEXT NOT NULL,
    created_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_lists_owner ON shopping_lists (owner_id);

CREATE TABLE list_items (
    id UUID PRIMARY KEY,
    owner_id UUID NOT NULL,
    list_id UUID NOT NULL REFERENCES shopping_lists (id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES products (id),
    offer_id UUID REFERENCES offers (id),
    quantity NUMERIC(14, 4) NOT NULL DEFAULT 1,
    price_snapshot NUMERIC(14, 4),
    is_purchased BOOLEAN NOT NULL DEFAULT false,
    created_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_list_items_list ON list_items (owner_id, list_id);

CREATE TABLE product_identities (
    owner_id UUID NOT NULL,
    product_id UUID NOT NULL REFERENCES products (id),
    external_id TEXT NOT NULL,
    source TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (owner_id, external_id, source)
);

CREATE INDEX idx_product_identities_product ON product_identities (owner_id, product_id);
