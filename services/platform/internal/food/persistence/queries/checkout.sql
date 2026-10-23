-- name: PruneExpiredFoodQuotes :exec
DELETE FROM commerce.food_quotes AS expired WHERE expired.merchant_id=$1 AND expired.store_id=$2 AND expired.id IN
 (SELECT id FROM commerce.food_quotes WHERE merchant_id=$1 AND store_id=$2 AND expires_at<=clock_timestamp() ORDER BY expires_at LIMIT 1000);

-- name: CountActiveFoodQuotes :one
SELECT count(*) FROM commerce.food_quotes WHERE merchant_id=$1 AND store_id=$2 AND expires_at>clock_timestamp();

-- name: InsertFoodQuote :exec
INSERT INTO commerce.food_quotes(merchant_id,store_id,id,body,expires_at,guest_id) VALUES($1,$2,$3,$4,$5,$6);

-- name: PruneAbandonedGuestGrants :exec
DELETE FROM commerce.guest_grants AS abandoned WHERE abandoned.merchant_id=$1 AND abandoned.id IN
 (SELECT g.id FROM commerce.guest_grants g WHERE g.merchant_id=$1 AND g.store_id=$2
 AND (g.expires_at<=clock_timestamp() OR g.revoked)
 AND NOT EXISTS(SELECT FROM commerce.food_quotes q WHERE q.merchant_id=g.merchant_id AND q.guest_id=g.id)
 AND NOT EXISTS(SELECT FROM commerce.food_orders o WHERE o.merchant_id=g.merchant_id AND o.guest_id=g.id)
 AND NOT EXISTS(SELECT FROM commerce.guest_request_replays r WHERE r.merchant_id=g.merchant_id AND r.grant_id=g.id)
 ORDER BY g.expires_at LIMIT 1000);

-- name: CountActiveGuestGrants :one
SELECT count(*) FROM commerce.guest_grants WHERE merchant_id=$1 AND store_id=$2 AND expires_at>clock_timestamp() AND NOT revoked;

-- name: InsertGuestGrant :one
INSERT INTO commerce.guest_grants(merchant_id,store_id,id,token_hash,expires_at) VALUES($1,$2,$3,$4,clock_timestamp()+interval '7 days') RETURNING expires_at;

-- name: InsertGuestMCPGrant :one
INSERT INTO commerce.guest_grants(merchant_id,store_id,id,token_hash,expires_at,resource)
VALUES($1,$2,$3,$4,clock_timestamp()+interval '1 hour','customer-food-mcp') RETURNING expires_at;
