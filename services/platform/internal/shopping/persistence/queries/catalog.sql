-- name: LockCatalogKey :exec
SELECT pg_advisory_xact_lock(hashtextextended(sqlc.arg(lock_key)::text,0));

-- name: LockCatalogRevision :one
SELECT revision FROM shopping.catalogs WHERE merchant_id=$1 AND store_id=$2 FOR UPDATE;

-- name: SaveCatalog :exec
INSERT INTO shopping.catalogs(merchant_id,store_id,revision,document) VALUES($1,$2,$3,$4) ON CONFLICT(merchant_id,store_id) DO UPDATE SET revision=EXCLUDED.revision,document=EXCLUDED.document;

-- name: SaveStock :exec
INSERT INTO shopping.stock(merchant_id,store_id,sku,available) VALUES($1,$2,$3,$4) ON CONFLICT(merchant_id,store_id,sku) DO UPDATE SET available=EXCLUDED.available;

-- name: ReadCatalog :one
SELECT revision,document FROM shopping.catalogs WHERE merchant_id=$1 AND store_id=$2 FOR SHARE;
