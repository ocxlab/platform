-- name: InsertGrant :exec
INSERT INTO commerce_internal.grants(id,merchant_id,store_id,customer_id,domain,audience,token_hash,profile_uri,profile,expires_at,management) VALUES($1,$2,$3,$4,$5,'ucp-internal',$6,$7,$8,$9,$10);

-- name: LookupGrant :one
SELECT id::uuid,merchant_id::uuid,store_id::uuid,customer_id::uuid,domain::text,profile_uri::text,profile::jsonb,management::boolean FROM commerce_internal.lookup_grant($1::bytea);
