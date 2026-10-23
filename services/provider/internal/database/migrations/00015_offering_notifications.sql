-- +goose Up
-- Public state changes only, never drafts, customer baskets or order activity.
-- NOTIFY delivers on commit; clients re-read the authoritative public projection.
-- +goose StatementBegin
CREATE FUNCTION commerce.notify_public_offering_change() RETURNS trigger
LANGUAGE plpgsql SET search_path=pg_catalog AS $$
BEGIN
  PERFORM pg_notify('ocp_public_offering_changed', json_build_object(
    'Merchant', NEW.merchant_id, 'Store', NEW.id)::text);
  RETURN NEW;
END;
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION commerce.notify_public_offering_change() FROM PUBLIC;
CREATE TRIGGER public_offering_change AFTER UPDATE ON commerce.stores
FOR EACH ROW WHEN (OLD.publication_generation IS DISTINCT FROM NEW.publication_generation
  OR OLD.ordering_revision IS DISTINCT FROM NEW.ordering_revision)
EXECUTE FUNCTION commerce.notify_public_offering_change();
