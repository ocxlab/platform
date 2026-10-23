-- +goose Up
-- Existing developer REST credentials retain their original audience. MCP grants
-- share ownership, expiry, revocation, and replay storage, not bearer authority.
ALTER TABLE auth.grants ADD COLUMN resource text NOT NULL DEFAULT 'merchant-rest'
 CHECK (resource IN ('merchant-rest','merchant-mcp'));

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION auth.lookup_grant(candidate bytea)
RETURNS TABLE(id uuid,merchant_id uuid,scopes text[])
LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT g.id,g.merchant_id,g.scopes FROM auth.grants g
 WHERE g.token_hash=candidate AND g.resource='merchant-rest'
 AND NOT g.revoked AND g.expires_at>now()
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION auth.lookup_resource_grant(candidate bytea, requested_resource text)
RETURNS TABLE(id uuid,merchant_id uuid,scopes text[])
LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT g.id,g.merchant_id,g.scopes FROM auth.grants g
 WHERE g.token_hash=candidate AND g.resource=requested_resource
 AND NOT g.revoked AND g.expires_at>now()
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION auth.lookup_resource_grant(bytea,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION auth.lookup_resource_grant(bytea,text) TO ocp_app;

-- +goose Down
SELECT 1; -- Forward-only: do not turn MCP credentials into REST credentials.
