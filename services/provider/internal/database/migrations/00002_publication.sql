-- +goose Up
CREATE TABLE auth.provider_grants (
  id uuid PRIMARY KEY,
  token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash)=32),
  scopes text[] NOT NULL,
  expires_at timestamptz NOT NULL,
  revoked boolean NOT NULL DEFAULT false
);
-- +goose StatementBegin
CREATE FUNCTION auth.lookup_provider_grant(candidate bytea)
RETURNS TABLE (id uuid, scopes text[])
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog
AS $$
  SELECT g.id, g.scopes FROM auth.provider_grants AS g
  WHERE g.token_hash=candidate AND NOT g.revoked AND g.expires_at>CURRENT_TIMESTAMP
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION auth.lookup_provider_grant(bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION auth.lookup_provider_grant(bytea) TO ocp_app;

ALTER TABLE commerce.stores
  ADD COLUMN publication_generation bigint NOT NULL DEFAULT 0 CHECK (publication_generation>=0),
  ADD COLUMN published_revision bigint,
  ADD COLUMN public_document jsonb CHECK (jsonb_typeof(public_document)='object'),
  ADD COLUMN publication_suspended boolean NOT NULL DEFAULT false,
  ADD CONSTRAINT published_revision_exists FOREIGN KEY (merchant_id,id,published_revision)
    REFERENCES commerce.offering_revisions(merchant_id,store_id,revision),
  ADD CONSTRAINT publication_snapshot_pair CHECK ((published_revision IS NULL)=(public_document IS NULL));

CREATE TABLE commerce.publication_audit (
  merchant_id uuid NOT NULL,
  store_id uuid NOT NULL,
  generation bigint NOT NULL CHECK (generation>0),
  actor_id uuid NOT NULL,
  actor_kind text NOT NULL CHECK (actor_kind IN ('merchant','provider')),
  request_id uuid NOT NULL,
  action text NOT NULL CHECK (action IN ('publish','withdraw','suspend','restore')),
  reason text NOT NULL,
  revision bigint,
  suspended boolean NOT NULL,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  PRIMARY KEY (merchant_id,store_id,generation),
  FOREIGN KEY (merchant_id,store_id) REFERENCES commerce.stores(merchant_id,id)
);
CREATE TABLE commerce.provider_request_replays (
  merchant_id uuid NOT NULL REFERENCES auth.merchants(id),
  grant_id uuid NOT NULL REFERENCES auth.provider_grants(id),
  operation text NOT NULL,
  resource text NOT NULL,
  replay_key text NOT NULL,
  request_hash bytea NOT NULL CHECK (octet_length(request_hash)=32),
  response_status integer NOT NULL,
  response_body bytea NOT NULL,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  PRIMARY KEY (merchant_id,grant_id,operation,resource,replay_key)
);
CREATE TABLE commerce.food_quotes (
  merchant_id uuid NOT NULL,
  store_id uuid NOT NULL,
  id uuid NOT NULL,
  body jsonb NOT NULL CHECK (jsonb_typeof(body)='object'),
  expires_at timestamptz NOT NULL,
  PRIMARY KEY (merchant_id,id),
  FOREIGN KEY (merchant_id,store_id) REFERENCES commerce.stores(merchant_id,id)
);
CREATE INDEX food_quotes_expiry ON commerce.food_quotes(merchant_id,store_id,expires_at);
ALTER TABLE commerce.publication_audit ENABLE ROW LEVEL SECURITY;
ALTER TABLE commerce.publication_audit FORCE ROW LEVEL SECURITY;
ALTER TABLE commerce.provider_request_replays ENABLE ROW LEVEL SECURITY;
ALTER TABLE commerce.provider_request_replays FORCE ROW LEVEL SECURITY;
ALTER TABLE commerce.food_quotes ENABLE ROW LEVEL SECURITY;
ALTER TABLE commerce.food_quotes FORCE ROW LEVEL SECURITY;
CREATE POLICY merchant_scope ON commerce.publication_audit
  USING (merchant_id=nullif(current_setting('ocp.merchant_id',true),'')::uuid)
  WITH CHECK (merchant_id=nullif(current_setting('ocp.merchant_id',true),'')::uuid);
CREATE POLICY merchant_scope ON commerce.provider_request_replays
  USING (merchant_id=nullif(current_setting('ocp.merchant_id',true),'')::uuid)
  WITH CHECK (merchant_id=nullif(current_setting('ocp.merchant_id',true),'')::uuid);
CREATE POLICY merchant_scope ON commerce.food_quotes
  USING (merchant_id=nullif(current_setting('ocp.merchant_id',true),'')::uuid)
  WITH CHECK (merchant_id=nullif(current_setting('ocp.merchant_id',true),'')::uuid);
GRANT SELECT,INSERT ON commerce.publication_audit,commerce.provider_request_replays,commerce.food_quotes TO ocp_app;
-- Expired quote snapshots are ephemeral, not purchase history. A bounded local
-- creation-time cleanup prunes them; no DELETE privilege on audit or replays.
GRANT DELETE ON commerce.food_quotes TO ocp_app;
