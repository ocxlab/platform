-- name: LockPropertyKey :exec
SELECT pg_advisory_xact_lock(hashtextextended(sqlc.arg(lock_key)::text,0));

-- name: LockPropertyRevision :one
SELECT revision FROM lodging.properties WHERE merchant_id=$1 AND store_id=$2 FOR UPDATE;

-- name: SaveProperty :exec
INSERT INTO lodging.properties(merchant_id,store_id,revision,document) VALUES($1,$2,$3,$4) ON CONFLICT(merchant_id,store_id) DO UPDATE SET revision=EXCLUDED.revision,document=EXCLUDED.document;

-- name: InsertUnit :exec
INSERT INTO lodging.units(merchant_id,store_id,unit_id) VALUES($1,$2,$3) ON CONFLICT DO NOTHING;

-- name: ReadProperty :one
SELECT revision,document FROM lodging.properties WHERE merchant_id=$1 AND store_id=$2 FOR SHARE;
