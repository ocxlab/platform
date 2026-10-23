-- name: GetPublicStoreByNumber :one
SELECT merchant_id::uuid,id::uuid,display_name::text,handle::text FROM commerce.public_store_by_number($1);

-- name: ListPublicStores :many
SELECT merchant_id::uuid,id::uuid,display_name::text,handle::text
FROM commerce.list_public_stores($1,sqlc.narg(after_merchant)::uuid,sqlc.narg(after_store)::uuid,$2);

-- name: ListAdminStores :many
SELECT body::jsonb,sequence_number::bigint
FROM commerce.admin_stores($1,$2,$3,sqlc.narg(after_number)::bigint,$4);

-- name: ListAdminOrders :many
SELECT body::jsonb,sequence_number::bigint
FROM commerce.admin_orders($1,$2,$3,sqlc.narg(after_number)::bigint,$4);
