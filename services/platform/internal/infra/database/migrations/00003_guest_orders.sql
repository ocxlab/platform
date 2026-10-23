-- +goose Up
CREATE TABLE commerce.guest_grants (
  merchant_id uuid NOT NULL,
  store_id uuid NOT NULL,
  id uuid NOT NULL,
  token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash)=32),
  expires_at timestamptz NOT NULL,
  revoked boolean NOT NULL DEFAULT false,
  PRIMARY KEY (merchant_id,id),
  UNIQUE (merchant_id,store_id,id),
  FOREIGN KEY (merchant_id,store_id) REFERENCES commerce.stores(merchant_id,id)
);
CREATE INDEX guest_grants_expiry ON commerce.guest_grants(merchant_id,store_id,expires_at);
-- +goose StatementBegin
CREATE FUNCTION auth.lookup_guest_grant(candidate bytea)
RETURNS TABLE (id uuid, merchant_id uuid, store_id uuid)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog
AS $$
  SELECT g.id,g.merchant_id,g.store_id FROM commerce.guest_grants g
  WHERE g.token_hash=candidate AND NOT g.revoked AND g.expires_at>CURRENT_TIMESTAMP
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION auth.lookup_guest_grant(bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION auth.lookup_guest_grant(bytea) TO ocp_app;

ALTER TABLE commerce.food_quotes ADD COLUMN guest_id uuid;
ALTER TABLE commerce.food_quotes ADD CONSTRAINT quote_guest FOREIGN KEY (merchant_id,store_id,guest_id)
  REFERENCES commerce.guest_grants(merchant_id,store_id,id);

CREATE TABLE commerce.food_orders (
  merchant_id uuid NOT NULL,
  store_id uuid NOT NULL,
  id uuid NOT NULL,
  guest_id uuid NOT NULL,
  quote_id uuid NOT NULL,
  body jsonb NOT NULL CHECK (jsonb_typeof(body)='object'),
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  PRIMARY KEY (merchant_id,store_id,id),
  UNIQUE (merchant_id,quote_id),
  FOREIGN KEY (merchant_id,store_id,guest_id) REFERENCES commerce.guest_grants(merchant_id,store_id,id)
);
-- Terms are copied, not linked to ephemeral quotes. No DELETE/UPDATE permission.
CREATE TABLE commerce.food_order_events (
  merchant_id uuid NOT NULL,
  store_id uuid NOT NULL,
  order_id uuid NOT NULL,
  sequence bigint NOT NULL CHECK (sequence>0),
  event text NOT NULL CHECK (event='placed'),
  actor_id uuid NOT NULL,
  request_id uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  PRIMARY KEY (merchant_id,store_id,order_id,sequence),
  FOREIGN KEY (merchant_id,store_id,order_id) REFERENCES commerce.food_orders(merchant_id,store_id,id)
);
CREATE TABLE commerce.guest_request_replays (
  merchant_id uuid NOT NULL,
  grant_id uuid NOT NULL,
  operation text NOT NULL,
  resource text NOT NULL,
  replay_key text NOT NULL,
  request_hash bytea NOT NULL CHECK (octet_length(request_hash)=32),
  response_status integer NOT NULL,
  response_body bytea NOT NULL,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  PRIMARY KEY (merchant_id,grant_id,operation,resource,replay_key),
  FOREIGN KEY (merchant_id,grant_id) REFERENCES commerce.guest_grants(merchant_id,id)
);
ALTER TABLE commerce.guest_grants ENABLE ROW LEVEL SECURITY;
ALTER TABLE commerce.guest_grants FORCE ROW LEVEL SECURITY;
ALTER TABLE commerce.food_orders ENABLE ROW LEVEL SECURITY;
ALTER TABLE commerce.food_orders FORCE ROW LEVEL SECURITY;
ALTER TABLE commerce.food_order_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE commerce.food_order_events FORCE ROW LEVEL SECURITY;
ALTER TABLE commerce.guest_request_replays ENABLE ROW LEVEL SECURITY;
ALTER TABLE commerce.guest_request_replays FORCE ROW LEVEL SECURITY;
CREATE POLICY merchant_scope ON commerce.guest_grants
  USING (merchant_id=nullif(current_setting('ocp.merchant_id',true),'')::uuid)
  WITH CHECK (merchant_id=nullif(current_setting('ocp.merchant_id',true),'')::uuid);
CREATE POLICY merchant_scope ON commerce.food_orders
  USING (merchant_id=nullif(current_setting('ocp.merchant_id',true),'')::uuid)
  WITH CHECK (merchant_id=nullif(current_setting('ocp.merchant_id',true),'')::uuid);
CREATE POLICY merchant_scope ON commerce.food_order_events
  USING (merchant_id=nullif(current_setting('ocp.merchant_id',true),'')::uuid)
  WITH CHECK (merchant_id=nullif(current_setting('ocp.merchant_id',true),'')::uuid);
CREATE POLICY merchant_scope ON commerce.guest_request_replays
  USING (merchant_id=nullif(current_setting('ocp.merchant_id',true),'')::uuid)
  WITH CHECK (merchant_id=nullif(current_setting('ocp.merchant_id',true),'')::uuid);
GRANT SELECT,INSERT ON commerce.guest_grants,commerce.food_orders,commerce.food_order_events,commerce.guest_request_replays TO ocp_app;
-- Only abandoned, unreferenced guest grants are pruned by the application.
GRANT DELETE ON commerce.guest_grants TO ocp_app;
-- PostgreSQL row locks require an UPDATE privilege. Restrict it to revocation;
-- grant identity, digest, scope and expiry remain immutable to the runtime.
GRANT UPDATE(revoked) ON commerce.guest_grants TO ocp_app;
