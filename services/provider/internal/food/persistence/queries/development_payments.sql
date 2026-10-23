-- name: GetDevelopmentPayment :one
SELECT state,attempts FROM payment.development_operations WHERE merchant_id=$1 AND store_id=$2 AND order_id=$3;

-- name: SaveDevelopmentPayment :exec
INSERT INTO payment.development_operations(merchant_id,store_id,order_id,state,attempts,amount_minor,currency)
VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(merchant_id,store_id,order_id) DO UPDATE SET state=EXCLUDED.state,attempts=EXCLUDED.attempts;
