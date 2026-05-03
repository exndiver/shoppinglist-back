ALTER INDEX idx_good_identities_good RENAME TO idx_product_identities_product;
ALTER TABLE good_identities RENAME COLUMN good_id TO product_id;
ALTER TABLE good_identities RENAME TO product_identities;

ALTER TABLE list_items RENAME COLUMN good_id TO product_id;

ALTER INDEX idx_offers_owner_good RENAME TO idx_offers_owner_product;
ALTER TABLE offers DROP CONSTRAINT offers_owner_id_good_id_store_id_key;
ALTER TABLE offers RENAME COLUMN good_id TO product_id;
ALTER TABLE offers ADD CONSTRAINT offers_owner_id_product_id_store_id_key UNIQUE (owner_id, product_id, store_id);

ALTER INDEX idx_goods_owner RENAME TO idx_products_owner;
ALTER INDEX idx_goods_owner_norm_canonical RENAME TO idx_products_owner_norm_canonical;

ALTER TABLE goods RENAME CONSTRAINT goods_no_self_merge TO products_no_self_merge;
ALTER TABLE goods RENAME TO products;
