-- name: LookupAccountGrant :one
SELECT id::uuid,account_id::uuid,kind::text,array_to_json(scopes)::text AS scopes_json
FROM auth.lookup_account_grant($1,$2);

-- name: InsertAccount :exec
INSERT INTO auth.accounts(id,kind,merchant_id,display_name,locale) VALUES($1,$2,$3,$4,$5);

-- name: InsertAccountGrant :exec
INSERT INTO auth.account_grants(id,account_id,token_hash,resource,scopes,expires_at)
VALUES($1,$2,$3,$4,ARRAY(SELECT jsonb_array_elements_text(sqlc.arg(scopes_json)::jsonb)),$5);

-- name: LockAccountGrant :one
SELECT id FROM auth.account_grants WHERE account_id=$1 AND id=$2 AND resource=$3
AND NOT revoked AND expires_at>clock_timestamp()
AND sqlc.arg(required_scope)::text=ANY(scopes) FOR SHARE;

-- name: ReadAccountProfile :one
SELECT id,kind,display_name,locale,revision,updated_at FROM auth.accounts WHERE id=$1;

-- name: LockAccountProfile :one
SELECT id,kind,display_name,locale,revision,updated_at FROM auth.accounts WHERE id=$1 FOR UPDATE;

-- name: UpdateAccountProfile :one
UPDATE auth.accounts SET display_name=$2,locale=$3,revision=revision+1,updated_at=clock_timestamp()
WHERE id=$1 RETURNING id,kind,display_name,locale,revision,updated_at;

-- name: ReadAccountReplay :one
SELECT request_hash,response_body FROM auth.account_replays WHERE account_id=$1 AND grant_id=$2 AND replay_key=$3;

-- name: InsertAccountReplay :exec
INSERT INTO auth.account_replays(account_id,grant_id,replay_key,request_hash,response_body) VALUES($1,$2,$3,$4,$5);

-- name: InsertAccountAudit :exec
INSERT INTO auth.account_audit(account_id,revision,grant_id,request_id) VALUES($1,$2,$3,$4);
