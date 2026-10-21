-- +goose Up
ALTER TABLE commerce.food_order_operations ADD COLUMN fulfilment_progress text NOT NULL DEFAULT 'not_started'
 CHECK(fulfilment_progress IN ('not_started','preparing','ready','collected'));
GRANT UPDATE(fulfilment_progress) ON commerce.food_order_operations TO ocp_app;
ALTER TABLE commerce.food_order_events DROP CONSTRAINT food_order_events_event_check;
ALTER TABLE commerce.food_order_events ADD CHECK(event IN
 ('placed','accepted','rejected','cancelled','payment_started','payment_paid','payment_failed','payment_abandoned','cancelled_refunded','preparation_started','ready_for_pickup','collected'));
