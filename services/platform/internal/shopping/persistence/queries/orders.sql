-- name: ReadCustomerOrder :one
SELECT o.session_id,o.terms FROM shopping.orders o JOIN shopping.sessions s ON s.merchant_id=o.merchant_id AND s.store_id=o.store_id AND s.id=o.session_id WHERE o.merchant_id=$1 AND o.store_id=$2 AND s.customer_id=$3 AND o.id=$4;

-- name: LockStock :one
SELECT available FROM shopping.stock WHERE merchant_id=$1 AND store_id=$2 AND sku=$3 FOR UPDATE;

-- name: AllocateStock :exec
UPDATE shopping.stock SET available=available-$4 WHERE merchant_id=$1 AND store_id=$2 AND sku=$3;

-- name: InsertOrder :exec
INSERT INTO shopping.orders(merchant_id,store_id,id,session_id,terms) VALUES($1,$2,$3,$4,$5);
