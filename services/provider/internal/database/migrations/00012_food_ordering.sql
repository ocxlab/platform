-- +goose Up
-- Ordering availability is independent of menu publication and existing orders.
ALTER TABLE commerce.stores
  ADD COLUMN ordering_revision bigint NOT NULL DEFAULT 0 CHECK (ordering_revision>=0),
  ADD COLUMN accepting_orders boolean NOT NULL DEFAULT true,
  ADD COLUMN acceptance_mode text NOT NULL DEFAULT 'manual' CHECK (acceptance_mode IN ('manual','automatic')),
  ADD COLUMN ordering_actor_id uuid;
GRANT UPDATE(ordering_revision,accepting_orders,acceptance_mode,ordering_actor_id) ON commerce.stores TO ocp_app;
CREATE TABLE commerce.food_ordering_audit (
  merchant_id uuid NOT NULL,
  store_id uuid NOT NULL,
  revision bigint NOT NULL,
  accepting_orders boolean NOT NULL,
  acceptance_mode text NOT NULL,
  actor_id uuid NOT NULL,
  request_id uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  PRIMARY KEY(merchant_id,store_id,revision),
  FOREIGN KEY(merchant_id,store_id) REFERENCES commerce.stores(merchant_id,id)
);
ALTER TABLE commerce.food_ordering_audit ENABLE ROW LEVEL SECURITY;
ALTER TABLE commerce.food_ordering_audit FORCE ROW LEVEL SECURITY;
CREATE POLICY merchant_scope ON commerce.food_ordering_audit
  USING (merchant_id=nullif(current_setting('ocp.merchant_id',true),'')::uuid)
  WITH CHECK (merchant_id=nullif(current_setting('ocp.merchant_id',true),'')::uuid);
GRANT SELECT,INSERT ON commerce.food_ordering_audit TO ocp_app;
