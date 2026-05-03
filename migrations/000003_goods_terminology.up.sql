-- Терминология: products → goods, product_id → good_id, product_identities → good_identities

ALTER TABLE products RENAME TO goods;
ALTER TABLE goods RENAME CONSTRAINT products_no_self_merge TO goods_no_self_merge;

ALTER INDEX idx_products_owner_norm_canonical RENAME TO idx_goods_owner_norm_canonical;
ALTER INDEX idx_products_owner RENAME TO idx_goods_owner;

ALTER TABLE offers DROP CONSTRAINT offers_owner_id_product_id_store_id_key;
ALTER TABLE offers RENAME COLUMN product_id TO good_id;
ALTER TABLE offers ADD CONSTRAINT offers_owner_id_good_id_store_id_key UNIQUE (owner_id, good_id, store_id);

ALTER INDEX idx_offers_owner_product RENAME TO idx_offers_owner_good;

ALTER TABLE list_items RENAME COLUMN product_id TO good_id;

ALTER TABLE product_identities RENAME TO good_identities;
ALTER TABLE good_identities RENAME COLUMN product_id TO good_id;
ALTER INDEX idx_product_identities_product RENAME TO idx_good_identities_good;
