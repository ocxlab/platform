-- +goose Up
CREATE SCHEMA payment;
GRANT USAGE ON SCHEMA payment TO ocp_app;
CREATE TABLE payment.development_operations (
 merchant_id uuid NOT NULL,
 store_id uuid NOT NULL,
 order_id uuid NOT NULL,
 state text NOT NULL CHECK(state IN ('not_started','awaiting_customer','failed','paid','refunded')),
 attempts integer NOT NULL CHECK(attempts BETWEEN 0 AND 10),
 amount_minor bigint NOT NULL CHECK(amount_minor>=0),
 currency text NOT NULL CHECK(currency='EUR'),
 PRIMARY KEY(merchant_id,store_id,order_id),
 FOREIGN KEY(merchant_id,store_id,order_id) REFERENCES commerce.food_orders(merchant_id,store_id,id)
);
ALTER TABLE payment.development_operations ENABLE ROW LEVEL SECURITY;
ALTER TABLE payment.development_operations FORCE ROW LEVEL SECURITY;
CREATE POLICY merchant_scope ON payment.development_operations
 USING(merchant_id=nullif(current_setting('ocp.merchant_id',true),'')::uuid)
 WITH CHECK(merchant_id=nullif(current_setting('ocp.merchant_id',true),'')::uuid);
GRANT SELECT,INSERT ON payment.development_operations TO ocp_app;
GRANT UPDATE(state,attempts) ON payment.development_operations TO ocp_app;
ALTER TABLE commerce.food_order_events DROP CONSTRAINT food_order_events_event_check;
ALTER TABLE commerce.food_order_events ADD CHECK(event IN
 ('placed','accepted','rejected','cancelled','payment_started','payment_paid','payment_failed','payment_abandoned','cancelled_refunded'));
