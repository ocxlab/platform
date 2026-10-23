-- +goose Up
SET LOCAL lock_timeout = '2s';
SET LOCAL statement_timeout = '2min';
-- Constant default is metadata-only on our PostgreSQL baseline. Existing guest
-- credentials remain REST-only; no token exchange or broader authority is added.
-- squawk-ignore adding-field-with-default
ALTER TABLE commerce.guest_grants ADD COLUMN resource text NOT NULL DEFAULT 'guest-rest';
ALTER TABLE commerce.guest_grants ADD CONSTRAINT guest_resource
 CHECK (resource IN ('guest-rest','customer-food-mcp')) NOT VALID;
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION auth.lookup_guest_grant(candidate bytea)
RETURNS TABLE(id uuid,merchant_id uuid,store_id uuid)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT g.id,g.merchant_id,g.store_id FROM commerce.guest_grants g
 WHERE g.token_hash=candidate AND g.resource='guest-rest'
 AND NOT g.revoked AND g.expires_at>CURRENT_TIMESTAMP
$$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION auth.lookup_guest_resource_grant(candidate bytea, requested_resource text)
RETURNS TABLE(id uuid,merchant_id uuid,store_id uuid)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT g.id,g.merchant_id,g.store_id FROM commerce.guest_grants g
 WHERE g.token_hash=candidate AND g.resource=requested_resource
 AND NOT g.revoked AND g.expires_at>CURRENT_TIMESTAMP
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION auth.lookup_guest_resource_grant(bytea,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION auth.lookup_guest_resource_grant(bytea,text) TO ocp_app;
-- +goose Down
SELECT 1; -- Forward only: never widen a credential's audience on rollback.
