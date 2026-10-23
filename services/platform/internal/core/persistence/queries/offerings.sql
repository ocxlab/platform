-- name: LockOfferingHead :one
SELECT offering_revision FROM commerce.stores WHERE merchant_id=$1 AND id=$2 FOR UPDATE;

-- name: InsertOfferingRevision :one
INSERT INTO commerce.offering_revisions(merchant_id,store_id,revision,model_id,model_version,document)
VALUES ($1,$2,$3,$4,$5,$6) RETURNING created_at;

-- name: UpdateOfferingHead :exec
UPDATE commerce.stores SET offering_revision=$3 WHERE merchant_id=$1 AND id=$2;

-- name: GetOffering :one
SELECT r.merchant_id,r.store_id,r.revision,r.model_id,r.model_version,r.document,r.created_at
FROM commerce.offering_revisions r JOIN commerce.stores s ON (r.merchant_id,r.store_id)=(s.merchant_id,s.id)
WHERE r.merchant_id=sqlc.arg(merchant_id) AND r.store_id=sqlc.arg(store_id)
AND r.revision=CASE WHEN sqlc.arg(requested_revision)::bigint=0 THEN s.offering_revision ELSE sqlc.arg(requested_revision)::bigint END;
