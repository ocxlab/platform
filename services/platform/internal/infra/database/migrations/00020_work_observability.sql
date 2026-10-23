-- +goose Up
SET LOCAL lock_timeout = '2s';
SET LOCAL statement_timeout = '2min';

-- Legacy rows start their age at this upgrade; their original enqueue time is unknown.
ALTER TABLE runtime.effects ADD COLUMN created_at timestamptz NOT NULL DEFAULT statement_timestamp();

-- +goose StatementBegin
CREATE FUNCTION runtime.work_snapshot(domains text[])
RETURNS TABLE(domain text,state text,amount bigint,oldest_seconds double precision)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE observed timestamptz := clock_timestamp();
BEGIN
 IF domains IS NULL OR cardinality(domains) NOT BETWEEN 1 AND 3
 OR array_position(domains,NULL) IS NOT NULL
 OR NOT domains <@ ARRAY['food','shopping','lodging']::text[]
 THEN RAISE EXCEPTION 'invalid work snapshot domains'; END IF;
 RETURN QUERY
 SELECT d.name,v.state,v.amount,
 CASE WHEN v.state='pending' THEN coalesce(greatest(0,extract(epoch FROM observed-s.oldest)),0)::double precision ELSE 0::double precision END
 FROM (SELECT DISTINCT unnest(domains) AS name) d
 CROSS JOIN LATERAL (
  SELECT count(*) FILTER(WHERE e.state='pending') AS pending,
   count(*) FILTER(WHERE e.state='pending' AND e.available_at>observed) AS delayed,
   count(*) FILTER(WHERE e.state='leased') AS leased,
   count(*) FILTER(WHERE e.state='leased' AND e.lease_until<=observed) AS expired,
   count(*) FILTER(WHERE e.state='failed') AS failed,
   min(e.created_at) FILTER(WHERE e.state='pending') AS oldest
  FROM runtime.effects e WHERE e.domain=d.name AND e.state IN ('pending','leased','failed')
 ) s
 CROSS JOIN LATERAL (VALUES
  ('pending',s.pending),('delayed',s.delayed),('leased',s.leased),('expired',s.expired),('failed',s.failed)
 ) v(state,amount);
END;
$$;
-- +goose StatementEnd
ALTER FUNCTION runtime.work_snapshot(text[]) OWNER TO ocp_work_owner;
REVOKE ALL ON FUNCTION runtime.work_snapshot(text[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION runtime.work_snapshot(text[]) TO ocp_app;
