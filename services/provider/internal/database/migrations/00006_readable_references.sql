-- +goose Up
-- Independent display references: UUIDs and immutable placement/replay bodies
-- retain their meaning. Sequences never reset; gaps after rollback are normal.
ALTER TABLE commerce.stores ADD COLUMN display_number bigint GENERATED ALWAYS AS IDENTITY UNIQUE;
ALTER TABLE commerce.food_orders ADD COLUMN display_number bigint GENERATED ALWAYS AS IDENTITY UNIQUE;
GRANT USAGE ON SEQUENCE commerce.stores_display_number_seq,commerce.food_orders_display_number_seq TO ocp_app;

DROP FUNCTION commerce.list_public_stores(text,uuid,uuid,integer);
-- +goose StatementBegin
CREATE FUNCTION commerce.list_public_stores(query text, after_merchant uuid, after_store uuid, page_limit integer)
RETURNS TABLE (merchant_id uuid, id uuid, display_name text, handle text)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog
AS $$
 SELECT s.merchant_id,s.id,s.display_name,'store-'||s.display_number FROM commerce.stores s
 WHERE s.published_revision IS NOT NULL AND s.public_document IS NOT NULL
   AND NOT s.publication_suspended
   AND (position(lower(coalesce(query,'')) in lower(s.display_name))>0
        OR lower(coalesce(query,''))='store-'||s.display_number)
   AND (after_merchant IS NULL OR (s.merchant_id,s.id)>(after_merchant,after_store))
 ORDER BY s.merchant_id,s.id LIMIT least(greatest(page_limit,1),101)
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION commerce.list_public_stores(text,uuid,uuid,integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION commerce.list_public_stores(text,uuid,uuid,integer) TO ocp_app;
