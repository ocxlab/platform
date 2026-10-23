-- REST deliberately retains its historical lookup and insert shapes. MCP grants
-- use resource-bound statements with no fallback to the REST credential audience.

-- name: LookupMerchantGrant :one
SELECT id::uuid,merchant_id::uuid,array_to_json(scopes)::text AS scopes_json FROM auth.lookup_grant($1);

-- name: LookupMerchantResourceGrant :one
SELECT id::uuid,merchant_id::uuid,array_to_json(scopes)::text AS scopes_json FROM auth.lookup_resource_grant($1,$2);

-- name: InsertMerchant :exec
INSERT INTO auth.merchants(id,display_name) VALUES($1,$2);

-- name: InsertMerchantGrant :exec
INSERT INTO auth.grants(id,merchant_id,token_hash,scopes,expires_at)
VALUES($1,$2,$3,ARRAY(SELECT jsonb_array_elements_text(sqlc.arg(scopes_json)::jsonb)),$4);

-- name: ProvisionMerchantGrant :execrows
INSERT INTO auth.grants(id,merchant_id,token_hash,scopes,expires_at)
SELECT $1,m.id,$2,ARRAY(SELECT jsonb_array_elements_text(sqlc.arg(scopes_json)::jsonb)),$3 FROM auth.merchants m WHERE m.id=sqlc.arg(merchant_id);

-- name: ProvisionMerchantResourceGrant :execrows
INSERT INTO auth.grants(id,merchant_id,token_hash,scopes,expires_at,resource)
SELECT $1,m.id,$2,ARRAY(SELECT jsonb_array_elements_text(sqlc.arg(scopes_json)::jsonb)),$3,$4 FROM auth.merchants m WHERE m.id=sqlc.arg(merchant_id);

-- name: LookupGuestGrant :one
SELECT id::uuid,merchant_id::uuid,store_id::uuid FROM auth.lookup_guest_grant($1);

-- name: LockActiveGuestGrant :one
SELECT id FROM commerce.guest_grants WHERE merchant_id=$1 AND store_id=$2 AND id=$3 AND NOT revoked AND expires_at>clock_timestamp() FOR SHARE;

-- name: LookupProviderGrant :one
SELECT id::uuid,array_to_json(scopes)::text AS scopes_json FROM auth.lookup_provider_grant($1);

-- name: InsertProviderGrant :exec
INSERT INTO auth.provider_grants(id,token_hash,scopes,expires_at)
VALUES($1,$2,ARRAY(SELECT jsonb_array_elements_text(sqlc.arg(scopes_json)::jsonb)),$3);
