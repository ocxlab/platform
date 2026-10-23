-- +goose Up
-- Narrow, read-only cross-tenant projections. Tenant tables keep forced RLS.
-- Grant identity and scope are rechecked here as well as at the HTTP boundary.
-- +goose StatementBegin
CREATE FUNCTION commerce.admin_stores(grant_id uuid, query text, state text, after_number bigint, page_limit integer)
RETURNS TABLE (body jsonb, sequence_number bigint)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog
AS $$
 SELECT jsonb_build_object('id',s.id,'merchant_id',s.merchant_id,
   'display_name',s.display_name,'merchant_name',m.display_name,
   'handle','store-'||s.display_number,'draft_revision',s.offering_revision,
   'publication_status',CASE WHEN s.publication_suspended THEN 'suspended'
     WHEN s.published_revision IS NOT NULL THEN 'published' ELSE 'unpublished' END,
   'updated_at',coalesce(r.created_at,s.created_at)),s.display_number
 FROM commerce.stores s JOIN auth.merchants m ON m.id=s.merchant_id
 LEFT JOIN commerce.offering_revisions r ON r.merchant_id=s.merchant_id
   AND r.store_id=s.id AND r.revision=s.offering_revision
 WHERE EXISTS (SELECT 1 FROM auth.provider_grants g WHERE g.id=grant_id
   AND NOT g.revoked AND g.expires_at>CURRENT_TIMESTAMP AND 'publication:read'=ANY(g.scopes))
 AND (position(lower(coalesce(query,'')) in lower(s.display_name||' '||m.display_name||' store-'||s.display_number))>0)
 AND (state='' OR state=CASE WHEN s.publication_suspended THEN 'suspended'
   WHEN s.published_revision IS NOT NULL THEN 'published' ELSE 'unpublished' END)
 AND (after_number IS NULL OR s.display_number<after_number)
 ORDER BY s.display_number DESC LIMIT least(greatest(page_limit,1),101)
$$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION commerce.admin_orders(grant_id uuid, query text, state text, after_number bigint, page_limit integer)
RETURNS TABLE (body jsonb, sequence_number bigint)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog
AS $$
 SELECT jsonb_build_object('id',o.id,'merchant_id',o.merchant_id,'store_id',o.store_id,
   'store_name',s.display_name,'reference','O-'||lpad(o.display_number::text,greatest(6,length(o.display_number::text)),'0'),
   'created_at',o.created_at,'acceptance',p.acceptance,'resolution',p.resolution,
   'payment',coalesce(pay.state,'not_started'),'fulfilment_progress',p.fulfilment_progress),o.display_number
 FROM commerce.food_orders o JOIN commerce.stores s ON s.merchant_id=o.merchant_id AND s.id=o.store_id
 JOIN commerce.food_order_operations p ON p.merchant_id=o.merchant_id AND p.store_id=o.store_id AND p.order_id=o.id
 LEFT JOIN payment.development_operations pay ON pay.merchant_id=o.merchant_id AND pay.store_id=o.store_id AND pay.order_id=o.id
 WHERE EXISTS (SELECT 1 FROM auth.provider_grants g WHERE g.id=grant_id
   AND NOT g.revoked AND g.expires_at>CURRENT_TIMESTAMP AND 'order:inspect'=ANY(g.scopes))
 AND position(lower(coalesce(query,'')) in lower(s.display_name||' O-'||lpad(o.display_number::text,greatest(6,length(o.display_number::text)),'0')))>0
 AND ($3='' OR $3=CASE WHEN p.resolution<>'open' THEN p.resolution ELSE p.acceptance END)
 AND (after_number IS NULL OR o.display_number<after_number)
 ORDER BY o.display_number DESC LIMIT least(greatest(page_limit,1),101)
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION commerce.admin_stores(uuid,text,text,bigint,integer),commerce.admin_orders(uuid,text,text,bigint,integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION commerce.admin_stores(uuid,text,text,bigint,integer),commerce.admin_orders(uuid,text,text,bigint,integer) TO ocp_app;
