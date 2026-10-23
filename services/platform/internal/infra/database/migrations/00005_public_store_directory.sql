-- +goose Up
-- Deliberately narrow cross-tenant public projection. Never return a whole store
-- row or disable tenant RLS for application queries. The trusted migration role
-- owns this function, just as it owns the credential lookup functions.
-- +goose StatementBegin
CREATE FUNCTION commerce.list_public_stores(query text, after_merchant uuid, after_store uuid, page_limit integer)
RETURNS TABLE (merchant_id uuid, id uuid, display_name text)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog
AS $$
  SELECT s.merchant_id,s.id,s.display_name FROM commerce.stores s
  WHERE s.published_revision IS NOT NULL AND s.public_document IS NOT NULL
    AND NOT s.publication_suspended
    AND position(lower(coalesce(query,'')) in lower(s.display_name))>0
    AND (after_merchant IS NULL OR (s.merchant_id,s.id)>(after_merchant,after_store))
  ORDER BY s.merchant_id,s.id LIMIT least(greatest(page_limit,1),101)
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION commerce.list_public_stores(text,uuid,uuid,integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION commerce.list_public_stores(text,uuid,uuid,integer) TO ocp_app;
