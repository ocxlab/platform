-- +goose Up
-- IDs only: NOTIFY is database-wide, not a private commerce-data channel.
-- Delivery occurs on commit; durable order history remains authoritative.
-- +goose StatementBegin
CREATE FUNCTION commerce.notify_food_order_change() RETURNS trigger
LANGUAGE plpgsql SET search_path=pg_catalog AS $$
BEGIN
  PERFORM pg_notify('ocp_food_order_changed', json_build_object(
    'Merchant', NEW.merchant_id, 'Store', NEW.store_id, 'Order', NEW.order_id)::text);
  RETURN NEW;
END;
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION commerce.notify_food_order_change() FROM PUBLIC;
CREATE TRIGGER food_order_change AFTER INSERT ON commerce.food_order_events
FOR EACH ROW EXECUTE FUNCTION commerce.notify_food_order_change();
