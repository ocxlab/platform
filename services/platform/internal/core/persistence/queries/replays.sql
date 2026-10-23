-- name: LockReplayKey :exec
SELECT pg_advisory_xact_lock(hashtextextended(sqlc.arg(lock_key)::text,0));

-- name: ReadMerchantReplay :one
SELECT request_hash,response_status,response_body FROM commerce.request_replays
WHERE merchant_id=$1 AND grant_id=$2 AND operation=$3 AND resource=$4 AND replay_key=$5;

-- name: InsertMerchantReplay :exec
INSERT INTO commerce.request_replays(merchant_id,grant_id,operation,resource,replay_key,request_hash,response_status,response_body)
VALUES($1,$2,$3,$4,$5,$6,$7,$8);

-- name: ReadProviderReplay :one
SELECT request_hash,response_status,response_body FROM commerce.provider_request_replays
WHERE merchant_id=$1 AND grant_id=$2 AND operation=$3 AND resource=$4 AND replay_key=$5;

-- name: InsertProviderReplay :exec
INSERT INTO commerce.provider_request_replays(merchant_id,grant_id,operation,resource,replay_key,request_hash,response_status,response_body)
VALUES($1,$2,$3,$4,$5,$6,$7,$8);

-- name: ReadGuestReplay :one
SELECT request_hash,response_status,response_body FROM commerce.guest_request_replays
WHERE merchant_id=$1 AND grant_id=$2 AND operation=$3 AND resource=$4 AND replay_key=$5;

-- name: InsertGuestReplay :exec
INSERT INTO commerce.guest_request_replays(merchant_id,grant_id,operation,resource,replay_key,request_hash,response_status,response_body)
VALUES($1,$2,$3,$4,$5,$6,$7,$8);
