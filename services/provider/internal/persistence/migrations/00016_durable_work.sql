-- +goose Up
CREATE SCHEMA runtime;
REVOKE ALL ON SCHEMA runtime FROM PUBLIC;
-- +goose StatementBegin
DO $$ BEGIN
 IF NOT EXISTS(SELECT FROM pg_roles WHERE rolname='ocp_work_owner') THEN
  CREATE ROLE ocp_work_owner NOLOGIN NOSUPERUSER NOBYPASSRLS;
 END IF;
END $$;
-- +goose StatementEnd
CREATE TABLE runtime.effects (
 id uuid PRIMARY KEY,
 merchant_id uuid NOT NULL,
 store_id uuid NOT NULL,
 domain text NOT NULL CHECK(domain IN ('food','shopping','lodging')),
 kind text NOT NULL CHECK(domain='food' AND kind='catalog.publish'),
 subject_id uuid NOT NULL,
 effect_key text NOT NULL CHECK(length(effect_key) BETWEEN 1 AND 200),
 payload jsonb NOT NULL CHECK(jsonb_typeof(payload)='object' AND octet_length(payload::text)<=1048576),
 state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','leased','completed','failed')),
 available_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 attempts integer NOT NULL DEFAULT 0 CHECK(attempts BETWEEN 0 AND 20),
 lease_token uuid,
 lease_generation bigint NOT NULL DEFAULT 0,
 lease_until timestamptz,
 completed_at timestamptz,
 failure_code text CHECK(failure_code IN ('transient','rejected','exhausted')),
 UNIQUE(merchant_id,store_id,domain,kind,effect_key),
 FOREIGN KEY(merchant_id,store_id) REFERENCES commerce.stores(merchant_id,id)
);
CREATE INDEX effect_due ON runtime.effects(domain,available_at,id) WHERE state IN ('pending','leased');
ALTER TABLE runtime.effects ENABLE ROW LEVEL SECURITY;
ALTER TABLE runtime.effects FORCE ROW LEVEL SECURITY;
CREATE POLICY effect_tenant ON runtime.effects TO ocp_app
 USING(merchant_id=nullif(current_setting('ocp.merchant_id',true),'')::uuid)
 WITH CHECK(merchant_id=nullif(current_setting('ocp.merchant_id',true),'')::uuid);
CREATE POLICY effect_worker ON runtime.effects TO ocp_work_owner USING(true) WITH CHECK(true);
GRANT USAGE ON SCHEMA runtime TO ocp_app,ocp_work_owner;
GRANT SELECT,INSERT ON runtime.effects TO ocp_app;
GRANT SELECT,UPDATE ON runtime.effects TO ocp_work_owner;

-- All cross-tenant updates are bounded and fenced. The non-login function owner
-- is not a member role of the application. No bearer tokens belong in payloads.
-- +goose StatementBegin
CREATE FUNCTION runtime.claim_effects(domains text[],token uuid,batch_size integer,lease_seconds integer)
RETURNS SETOF runtime.effects LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF token IS NULL OR token='00000000-0000-0000-0000-000000000000' OR batch_size IS NULL OR batch_size NOT BETWEEN 1 AND 20
 OR lease_seconds IS NULL OR lease_seconds NOT BETWEEN 1 AND 120 OR domains IS NULL OR cardinality(domains) NOT BETWEEN 1 AND 3
 OR NOT domains <@ ARRAY['food','shopping','lodging']::text[] THEN RAISE EXCEPTION 'invalid work claim'; END IF;
 WITH exhausted AS (
 SELECT e.id FROM runtime.effects e WHERE e.domain=ANY(domains) AND e.state='leased'
 AND e.lease_until<=clock_timestamp() AND e.attempts>=20
 ORDER BY e.available_at,e.id FOR UPDATE SKIP LOCKED LIMIT batch_size
 ) UPDATE runtime.effects e SET state='failed',failure_code='exhausted',lease_token=NULL,lease_until=NULL
 FROM exhausted WHERE e.id=exhausted.id;
 RETURN QUERY WITH due AS (
  SELECT e.id FROM runtime.effects e WHERE e.domain=ANY(domains) AND e.attempts<20 AND
   ((e.state='pending' AND e.available_at<=clock_timestamp()) OR (e.state='leased' AND e.lease_until<=clock_timestamp()))
  ORDER BY e.available_at,e.id FOR UPDATE SKIP LOCKED LIMIT batch_size
 ) UPDATE runtime.effects e SET state='leased',attempts=e.attempts+1,lease_token=token,
 lease_generation=e.lease_generation+1,lease_until=clock_timestamp()+make_interval(secs=>lease_seconds)
 FROM due WHERE e.id=due.id RETURNING e.*;
END $$;
-- +goose StatementEnd
ALTER FUNCTION runtime.claim_effects(text[],uuid,integer,integer) OWNER TO ocp_work_owner;
REVOKE ALL ON FUNCTION runtime.claim_effects(text[],uuid,integer,integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION runtime.claim_effects(text[],uuid,integer,integer) TO ocp_app;

-- +goose StatementBegin
CREATE FUNCTION runtime.finish_effect(effect uuid,token uuid,generation bigint,success boolean,retry_seconds integer)
RETURNS boolean LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE affected integer;
BEGIN
 IF retry_seconds IS NULL OR success IS NULL OR retry_seconds NOT BETWEEN 0 AND 3600 THEN RAISE EXCEPTION 'invalid retry delay'; END IF;
 UPDATE runtime.effects SET
 state=CASE WHEN success THEN 'completed' WHEN attempts>=20 OR retry_seconds=0 THEN 'failed' ELSE 'pending' END,
 available_at=clock_timestamp()+make_interval(secs=>retry_seconds),
 completed_at=CASE WHEN success THEN clock_timestamp() ELSE NULL END,
 failure_code=CASE WHEN success THEN NULL WHEN attempts>=20 THEN 'exhausted' WHEN retry_seconds=0 THEN 'rejected' ELSE 'transient' END,
 lease_token=NULL,lease_until=NULL
 WHERE id=effect AND state='leased' AND lease_token=token AND lease_generation=generation AND lease_until>clock_timestamp();
 GET DIAGNOSTICS affected=ROW_COUNT;
 RETURN affected=1;
END $$;
-- +goose StatementEnd
ALTER FUNCTION runtime.finish_effect(uuid,uuid,bigint,boolean,integer) OWNER TO ocp_work_owner;
REVOKE ALL ON FUNCTION runtime.finish_effect(uuid,uuid,bigint,boolean,integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION runtime.finish_effect(uuid,uuid,bigint,boolean,integer) TO ocp_app;
