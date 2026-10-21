-- +goose Up
-- Agreed terms and original placement bodies remain immutable. Mutable progress
-- has its own row lock and an append-only event sequence.
CREATE TABLE commerce.food_order_operations (
  merchant_id uuid NOT NULL,
  store_id uuid NOT NULL,
  order_id uuid NOT NULL,
  revision bigint NOT NULL DEFAULT 1 CHECK (revision>0),
  acceptance text NOT NULL DEFAULT 'pending' CHECK (acceptance IN ('pending','accepted','rejected')),
  resolution text NOT NULL DEFAULT 'open' CHECK (resolution IN ('open','rejected','cancelled')),
  PRIMARY KEY (merchant_id,store_id,order_id),
  FOREIGN KEY (merchant_id,store_id,order_id) REFERENCES commerce.food_orders(merchant_id,store_id,id),
  CHECK ((acceptance='rejected') = (resolution='rejected'))
);
INSERT INTO commerce.food_order_operations(merchant_id,store_id,order_id)
SELECT merchant_id,store_id,id FROM commerce.food_orders;
ALTER TABLE commerce.food_order_operations ENABLE ROW LEVEL SECURITY;
ALTER TABLE commerce.food_order_operations FORCE ROW LEVEL SECURITY;
CREATE POLICY merchant_scope ON commerce.food_order_operations
  USING (merchant_id=nullif(current_setting('ocp.merchant_id',true),'')::uuid)
  WITH CHECK (merchant_id=nullif(current_setting('ocp.merchant_id',true),'')::uuid);
GRANT SELECT,INSERT ON commerce.food_order_operations TO ocp_app;
GRANT UPDATE(revision,acceptance,resolution) ON commerce.food_order_operations TO ocp_app;
ALTER TABLE commerce.food_order_events DROP CONSTRAINT food_order_events_event_check;
ALTER TABLE commerce.food_order_events ADD CHECK (event IN ('placed','accepted','rejected','cancelled'));
CREATE INDEX food_orders_inbox ON commerce.food_orders(merchant_id,store_id,created_at DESC,id DESC);
