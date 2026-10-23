-- name: GetGuestFoodQuote :one
SELECT body,expires_at FROM commerce.food_quotes WHERE merchant_id=$1 AND store_id=$2 AND id=$3 AND guest_id=$4;

-- name: FoodQuoteUsed :one
SELECT EXISTS(SELECT FROM commerce.food_orders WHERE merchant_id=$1 AND quote_id=$2);

-- name: DatabaseTime :one
SELECT clock_timestamp()::timestamptz AS now;

-- name: InsertFoodOrder :one
INSERT INTO commerce.food_orders(merchant_id,store_id,id,guest_id,quote_id,body,created_at) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING display_number;

-- name: InsertFoodOrderOperations :exec
INSERT INTO commerce.food_order_operations(merchant_id,store_id,order_id) VALUES($1,$2,$3);

-- name: UpdateFoodOrderOperations :exec
UPDATE commerce.food_order_operations SET revision=$4,acceptance=$5,resolution=$6,fulfilment_progress=$7 WHERE merchant_id=$1 AND store_id=$2 AND order_id=$3;

-- name: InsertFoodOrderEvent :one
INSERT INTO commerce.food_order_events(merchant_id,store_id,order_id,sequence,event,actor_id,request_id) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING created_at;

-- name: GetGuestFoodOrderID :one
SELECT id FROM commerce.food_orders WHERE merchant_id=$1 AND store_id=$2 AND guest_id=$3 AND display_number=$4;

-- name: ReadFoodOrder :one
SELECT o.body,s.revision,s.acceptance,s.resolution,o.display_number,s.fulfilment_progress FROM commerce.food_orders o
JOIN commerce.food_order_operations s ON (s.merchant_id,s.store_id,s.order_id)=(o.merchant_id,o.store_id,o.id)
WHERE o.merchant_id=$1 AND o.store_id=$2 AND o.id=$3 AND (sqlc.narg(guest_id)::uuid IS NULL OR o.guest_id=sqlc.narg(guest_id)) FOR SHARE OF s;

-- name: LockFoodOrder :one
SELECT o.body,s.revision,s.acceptance,s.resolution,o.display_number,s.fulfilment_progress FROM commerce.food_orders o
JOIN commerce.food_order_operations s ON (s.merchant_id,s.store_id,s.order_id)=(o.merchant_id,o.store_id,o.id)
WHERE o.merchant_id=$1 AND o.store_id=$2 AND o.id=$3 AND (sqlc.narg(guest_id)::uuid IS NULL OR o.guest_id=sqlc.narg(guest_id)) FOR UPDATE OF s;

-- name: ListFoodOrderEvents :many
SELECT sequence,event,created_at FROM commerce.food_order_events WHERE merchant_id=$1 AND store_id=$2 AND order_id=$3 ORDER BY sequence;

-- name: FoodStoreExists :one
SELECT EXISTS(SELECT FROM commerce.stores WHERE merchant_id=$1 AND id=$2);

-- name: ListFoodOrders :many
SELECT o.body,s.revision,s.acceptance,s.resolution,o.display_number,
 coalesce(p.state,'not_started')::text AS payment_state,(p.state IS NOT NULL)::boolean AS development,s.fulfilment_progress
FROM commerce.food_orders o
JOIN commerce.food_order_operations s ON (s.merchant_id,s.store_id,s.order_id)=(o.merchant_id,o.store_id,o.id)
LEFT JOIN payment.development_operations p ON (p.merchant_id,p.store_id,p.order_id)=(o.merchant_id,o.store_id,o.id)
WHERE o.merchant_id=$1 AND o.store_id=$2
 AND (sqlc.narg(after_created_at)::timestamptz IS NULL OR (o.created_at,o.id)<(sqlc.narg(after_created_at),sqlc.narg(after_id)::uuid))
ORDER BY o.created_at DESC,o.id DESC LIMIT sqlc.arg(page_limit);
