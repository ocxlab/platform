-- name: DatabaseTime :one
SELECT clock_timestamp()::timestamptz AS now;

-- name: LockSession :one
SELECT revision,body,request FROM lodging.sessions WHERE merchant_id=$1 AND store_id=$2 AND customer_id=$3 AND id=$4 FOR UPDATE;

-- name: SaveSession :exec
INSERT INTO lodging.sessions(merchant_id,store_id,customer_id,id,revision,body,request) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(merchant_id,store_id,id) DO UPDATE SET revision=EXCLUDED.revision,body=EXCLUDED.body,request=EXCLUDED.request;
