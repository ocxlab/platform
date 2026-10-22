-- name: ReadPublication :one
SELECT publication_generation, published_revision, publication_suspended
FROM commerce.stores
WHERE merchant_id = $1 AND id = $2;

-- name: GetPublicationSource :one
SELECT model_id,model_version,document FROM commerce.offering_revisions
WHERE merchant_id=$1 AND store_id=$2 AND revision=$3;

-- name: UpdatePublishedOffering :exec
UPDATE commerce.stores SET published_revision=sqlc.narg(revision),public_document=sqlc.narg(document),
publication_generation=publication_generation+1 WHERE merchant_id=sqlc.arg(merchant_id) AND id=sqlc.arg(store_id);

-- name: InsertPublicationAudit :exec
INSERT INTO commerce.publication_audit(merchant_id,store_id,generation,actor_id,actor_kind,action,reason,revision,suspended,request_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10);

-- name: GetCatalogSnapshot :one
SELECT r.model_id,r.model_version,s.display_name,coalesce(s.public_document,'null'::jsonb)::jsonb AS document
FROM commerce.offering_revisions r JOIN commerce.stores s ON (s.merchant_id,s.id)=(r.merchant_id,r.store_id)
WHERE r.merchant_id=$1 AND r.store_id=$2 AND r.revision=$3;

-- name: LockPublication :one
SELECT publication_generation, published_revision, publication_suspended
FROM commerce.stores
WHERE merchant_id = $1 AND id = $2
FOR UPDATE;

-- name: SetPublicationSuspended :exec
UPDATE commerce.stores
SET publication_suspended = $3, publication_generation = publication_generation + 1
WHERE merchant_id = $1 AND id = $2;
