-- name: ReadPublication :one
SELECT publication_generation, published_revision, publication_suspended
FROM commerce.stores
WHERE merchant_id = $1 AND id = $2;

-- name: LockPublication :one
SELECT publication_generation, published_revision, publication_suspended
FROM commerce.stores
WHERE merchant_id = $1 AND id = $2
FOR UPDATE;

-- name: SetPublicationSuspended :exec
UPDATE commerce.stores
SET publication_suspended = $3, publication_generation = publication_generation + 1
WHERE merchant_id = $1 AND id = $2;
