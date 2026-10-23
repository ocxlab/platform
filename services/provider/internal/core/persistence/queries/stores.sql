-- name: CreateStore :one
INSERT INTO commerce.stores(merchant_id,id,display_name)
VALUES ($1,$2,$3) RETURNING created_at,('store-'||display_number)::text AS handle;

-- name: GetStore :one
SELECT id,merchant_id,display_name,created_at,('store-'||display_number)::text AS handle
FROM commerce.stores WHERE merchant_id=$1 AND id=$2;

-- name: ListManagedStores :many
SELECT s.id,s.merchant_id,s.display_name,s.created_at,s.offering_revision,
 (CASE WHEN s.offering_revision=0 THEN 0
 WHEN r.model_id='ocp.food.menu' AND r.model_version='1.0.0'
 AND jsonb_typeof(r.document->'items')='array' THEN jsonb_array_length(r.document->'items')
 ELSE -1 END)::bigint AS menu_item_count,
 COALESCE(r.created_at,s.created_at)::timestamptz AS updated_at,
 ('store-'||s.display_number)::text AS handle
FROM commerce.stores s LEFT JOIN commerce.offering_revisions r
 ON r.merchant_id=s.merchant_id AND r.store_id=s.id AND r.revision=s.offering_revision
WHERE s.merchant_id=sqlc.arg(merchant_id)
 AND (sqlc.narg(after_id)::uuid IS NULL OR s.id>sqlc.narg(after_id)::uuid)
 AND (sqlc.narg(display_number)::bigint IS NULL OR s.display_number=sqlc.narg(display_number)::bigint)
ORDER BY s.id LIMIT sqlc.arg(page_limit);
