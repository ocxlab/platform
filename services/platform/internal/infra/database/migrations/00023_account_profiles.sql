-- +goose Up
SET LOCAL lock_timeout = '2s';
SET LOCAL statement_timeout = '2min';
CREATE TABLE auth.accounts (
 id uuid PRIMARY KEY,
 kind text NOT NULL CHECK (kind IN ('merchant','customer')),
 merchant_id uuid REFERENCES auth.merchants(id),
 display_name text NOT NULL CHECK (length(btrim(display_name)) BETWEEN 1 AND 160),
 locale text NOT NULL CHECK (locale IN ('de','en','ar')),
 revision bigint NOT NULL DEFAULT 1 CHECK (revision>0),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 CHECK ((kind='merchant') = (merchant_id IS NOT NULL))
);
CREATE TABLE auth.account_grants (
 id uuid PRIMARY KEY,
 account_id uuid NOT NULL REFERENCES auth.accounts(id),
 token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash)=32),
 resource text NOT NULL CHECK (resource IN ('merchant-account-rest','customer-account-rest','merchant-account-mcp','customer-account-mcp')),
 scopes text[] NOT NULL,
 expires_at timestamptz NOT NULL,
 revoked boolean NOT NULL DEFAULT false,
 UNIQUE(account_id,id)
);
CREATE TABLE auth.account_replays (
 account_id uuid NOT NULL,
 grant_id uuid NOT NULL,
 replay_key text NOT NULL,
 request_hash bytea NOT NULL CHECK (octet_length(request_hash)=32),
 response_body bytea NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(account_id,grant_id,replay_key),
 FOREIGN KEY(account_id,grant_id) REFERENCES auth.account_grants(account_id,id)
);
CREATE TABLE auth.account_audit (
 account_id uuid NOT NULL REFERENCES auth.accounts(id),
 revision bigint NOT NULL,
 grant_id uuid NOT NULL,
 request_id uuid NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(account_id,revision),
 FOREIGN KEY(account_id,grant_id) REFERENCES auth.account_grants(account_id,id)
);
-- New empty table; establish isolation before granting runtime access.
-- squawk-ignore ban-alter-row-level-security
ALTER TABLE auth.accounts ENABLE ROW LEVEL SECURITY;
-- New empty table; establish isolation before granting runtime access.
-- squawk-ignore ban-alter-row-level-security
ALTER TABLE auth.account_grants ENABLE ROW LEVEL SECURITY;
-- The non-runtime table/function owner can resolve a hashed credential before
-- account scope exists, without requiring a superuser/BYPASSRLS migration role.
-- CheckRuntimeRole rejects ownership or membership in this owner's role.
-- New empty table; establish isolation before granting runtime access.
-- squawk-ignore ban-alter-row-level-security
ALTER TABLE auth.account_replays ENABLE ROW LEVEL SECURITY;
-- New empty table; establish isolation before granting runtime access.
-- squawk-ignore ban-alter-row-level-security
ALTER TABLE auth.account_replays FORCE ROW LEVEL SECURITY;
-- New empty table; establish isolation before granting runtime access.
-- squawk-ignore ban-alter-row-level-security
ALTER TABLE auth.account_audit ENABLE ROW LEVEL SECURITY;
-- New empty table; establish isolation before granting runtime access.
-- squawk-ignore ban-alter-row-level-security
ALTER TABLE auth.account_audit FORCE ROW LEVEL SECURITY;
CREATE POLICY account_scope ON auth.accounts
 USING(id=nullif(current_setting('ocp.account_id',true),'')::uuid)
 WITH CHECK(id=nullif(current_setting('ocp.account_id',true),'')::uuid);
CREATE POLICY account_scope ON auth.account_grants
 FOR SELECT USING(account_id=nullif(current_setting('ocp.account_id',true),'')::uuid);
-- Row locking requires an UPDATE privilege. Limit it to irreversible local
-- revocation, never token hashes, resources, scopes, or another account's grants.
CREATE POLICY account_revoke ON auth.account_grants FOR UPDATE
 USING(account_id=nullif(current_setting('ocp.account_id',true),'')::uuid AND NOT revoked)
 WITH CHECK(account_id=nullif(current_setting('ocp.account_id',true),'')::uuid AND revoked);
CREATE POLICY account_scope ON auth.account_replays
 USING(account_id=nullif(current_setting('ocp.account_id',true),'')::uuid)
 WITH CHECK(account_id=nullif(current_setting('ocp.account_id',true),'')::uuid);
CREATE POLICY account_scope ON auth.account_audit
 USING(account_id=nullif(current_setting('ocp.account_id',true),'')::uuid)
 WITH CHECK(account_id=nullif(current_setting('ocp.account_id',true),'')::uuid);
GRANT SELECT ON auth.accounts,auth.account_grants TO ocp_app;
GRANT UPDATE(revoked) ON auth.account_grants TO ocp_app;
GRANT UPDATE(display_name,locale,revision,updated_at) ON auth.accounts TO ocp_app;
GRANT SELECT,INSERT ON auth.account_replays TO ocp_app;
GRANT INSERT ON auth.account_audit TO ocp_app;
-- +goose StatementBegin
CREATE FUNCTION auth.lookup_account_grant(candidate bytea, requested_resource text)
RETURNS TABLE(id uuid,account_id uuid,kind text,scopes text[])
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT g.id,g.account_id,a.kind,g.scopes FROM auth.account_grants g
 JOIN auth.accounts a ON a.id=g.account_id
 WHERE g.token_hash=candidate AND g.resource=requested_resource
 AND split_part(requested_resource,'-',1)=a.kind
 AND NOT g.revoked AND g.expires_at>CURRENT_TIMESTAMP
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION auth.lookup_account_grant(bytea,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION auth.lookup_account_grant(bytea,text) TO ocp_app;
-- +goose Down
SELECT 1; -- Forward only; accounts and replay evidence must not be destroyed.
