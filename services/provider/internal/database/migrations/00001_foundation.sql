-- +goose Up
CREATE SCHEMA auth;
CREATE SCHEMA commerce;
REVOKE ALL ON SCHEMA auth, commerce FROM PUBLIC;

CREATE TABLE auth.merchants (
  id uuid PRIMARY KEY,
  display_name text NOT NULL CHECK (length(btrim(display_name)) BETWEEN 1 AND 160)
);
CREATE TABLE auth.grants (
  id uuid PRIMARY KEY,
  merchant_id uuid NOT NULL REFERENCES auth.merchants(id),
  token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
  scopes text[] NOT NULL,
  expires_at timestamptz NOT NULL,
  revoked boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  UNIQUE (merchant_id, id)
);

-- +goose StatementBegin
CREATE FUNCTION auth.lookup_grant(candidate bytea)
RETURNS TABLE (id uuid, merchant_id uuid, scopes text[])
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog
AS $$
  SELECT g.id, g.merchant_id, g.scopes FROM auth.grants AS g
  WHERE g.token_hash = candidate AND NOT g.revoked AND g.expires_at > CURRENT_TIMESTAMP
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION auth.lookup_grant(bytea) FROM PUBLIC;
GRANT USAGE ON SCHEMA auth TO ocp_app;
GRANT EXECUTE ON FUNCTION auth.lookup_grant(bytea) TO ocp_app;

CREATE TABLE commerce.stores (
  merchant_id uuid NOT NULL REFERENCES auth.merchants(id),
  id uuid NOT NULL,
  display_name text NOT NULL CHECK (length(btrim(display_name)) BETWEEN 1 AND 160),
  offering_revision bigint NOT NULL DEFAULT 0 CHECK (offering_revision >= 0),
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  PRIMARY KEY (merchant_id, id)
);
CREATE TABLE commerce.offering_revisions (
  merchant_id uuid NOT NULL,
  store_id uuid NOT NULL,
  revision bigint NOT NULL CHECK (revision > 0),
  model_id text NOT NULL,
  model_version text NOT NULL,
  document jsonb NOT NULL CHECK (jsonb_typeof(document) = 'object'),
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  PRIMARY KEY (merchant_id, store_id, revision),
  FOREIGN KEY (merchant_id, store_id) REFERENCES commerce.stores(merchant_id, id)
);
CREATE TABLE commerce.request_replays (
  merchant_id uuid NOT NULL REFERENCES auth.merchants(id),
  grant_id uuid NOT NULL,
  operation text NOT NULL,
  resource text NOT NULL,
  replay_key text NOT NULL,
  request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
  response_status integer NOT NULL,
  response_body bytea NOT NULL,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  PRIMARY KEY (merchant_id, grant_id, operation, resource, replay_key),
  FOREIGN KEY (merchant_id, grant_id) REFERENCES auth.grants(merchant_id, id)
);

ALTER TABLE commerce.stores ENABLE ROW LEVEL SECURITY;
ALTER TABLE commerce.stores FORCE ROW LEVEL SECURITY;
ALTER TABLE commerce.offering_revisions ENABLE ROW LEVEL SECURITY;
ALTER TABLE commerce.offering_revisions FORCE ROW LEVEL SECURITY;
ALTER TABLE commerce.request_replays ENABLE ROW LEVEL SECURITY;
ALTER TABLE commerce.request_replays FORCE ROW LEVEL SECURITY;
CREATE POLICY merchant_scope ON commerce.stores
  USING (merchant_id = nullif(current_setting('ocp.merchant_id', true), '')::uuid)
  WITH CHECK (merchant_id = nullif(current_setting('ocp.merchant_id', true), '')::uuid);
CREATE POLICY merchant_scope ON commerce.offering_revisions
  USING (merchant_id = nullif(current_setting('ocp.merchant_id', true), '')::uuid)
  WITH CHECK (merchant_id = nullif(current_setting('ocp.merchant_id', true), '')::uuid);
CREATE POLICY merchant_scope ON commerce.request_replays
  USING (merchant_id = nullif(current_setting('ocp.merchant_id', true), '')::uuid)
  WITH CHECK (merchant_id = nullif(current_setting('ocp.merchant_id', true), '')::uuid);

GRANT USAGE ON SCHEMA commerce TO ocp_app;
GRANT SELECT, INSERT, UPDATE ON commerce.stores TO ocp_app;
GRANT SELECT, INSERT ON commerce.offering_revisions, commerce.request_replays TO ocp_app;

-- No destructive down migration: the maintained development database may hold work.
