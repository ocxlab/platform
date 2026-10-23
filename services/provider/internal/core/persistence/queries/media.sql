-- name: GetMediaAsset :one
SELECT content_type,size_bytes,checksum,state,width,height,created_at,expires_at
FROM commerce.media_assets WHERE merchant_id=$1 AND store_id=$2 AND id=$3;

-- name: GetMediaAllocation :one
SELECT count(*) AS assets,coalesce(sum(size_bytes),0)::bigint AS allocated
FROM commerce.media_assets WHERE merchant_id=$1 AND store_id=$2;

-- name: InsertMediaAsset :exec
INSERT INTO commerce.media_assets(merchant_id,store_id,id,content_type,size_bytes,checksum,actor_id) VALUES($1,$2,$3,$4,$5,$6,$7);

-- name: CompleteMediaAsset :exec
UPDATE commerce.media_assets SET state='ready',width=sqlc.arg(width)::bigint::integer,height=sqlc.arg(height)::bigint::integer,completed_at=coalesce(completed_at,clock_timestamp()) WHERE merchant_id=$1 AND store_id=$2 AND id=$3;
