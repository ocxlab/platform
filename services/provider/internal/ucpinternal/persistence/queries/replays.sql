-- name: LockReplayKey :exec
SELECT pg_advisory_xact_lock(hashtextextended(sqlc.arg(lock_key)::text,0));

-- name: ReadReplay :one
SELECT digest,response_status,response_body FROM commerce_internal.replays WHERE merchant_id=$1 AND grant_id=$2 AND domain=$3 AND operation=$4 AND resource=$5 AND replay_key=$6;

-- name: InsertReplay :exec
INSERT INTO commerce_internal.replays(merchant_id,grant_id,domain,operation,resource,replay_key,digest,response_status,response_body) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9);
