-- +goose Up
-- Narrow public projection using the unique display-number index.
-- +goose StatementBegin
CREATE FUNCTION commerce.public_store_by_number(store_number bigint)
RETURNS TABLE (merchant_id uuid, id uuid, display_name text, handle text)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog
AS $$
 SELECT s.merchant_id,s.id,s.display_name,'store-'||s.display_number
 FROM commerce.stores s WHERE s.display_number=store_number
 AND s.published_revision IS NOT NULL AND s.public_document IS NOT NULL
 AND NOT s.publication_suspended
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION commerce.public_store_by_number(bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION commerce.public_store_by_number(bigint) TO ocp_app;
