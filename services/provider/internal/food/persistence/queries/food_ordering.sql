-- name: GetFoodOrdering :one
SELECT ordering_revision,accepting_orders,acceptance_mode,ordering_actor_id
FROM commerce.stores WHERE merchant_id=$1 AND id=$2
AND (NOT sqlc.arg(public_only)::boolean OR (published_revision IS NOT NULL AND NOT publication_suspended));

-- name: LockFoodOrdering :one
SELECT ordering_revision,accepting_orders,acceptance_mode,ordering_actor_id
FROM commerce.stores WHERE merchant_id=$1 AND id=$2
AND (NOT sqlc.arg(public_only)::boolean OR (published_revision IS NOT NULL AND NOT publication_suspended)) FOR UPDATE;

-- name: UpdateFoodOrdering :exec
UPDATE commerce.stores SET ordering_revision=$3,accepting_orders=$4,acceptance_mode=$5,ordering_actor_id=$6 WHERE merchant_id=$1 AND id=$2;

-- name: InsertFoodOrderingAudit :exec
INSERT INTO commerce.food_ordering_audit(merchant_id,store_id,revision,accepting_orders,acceptance_mode,actor_id,request_id) VALUES($1,$2,$3,$4,$5,$6,$7);
