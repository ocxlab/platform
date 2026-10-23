-- +goose Up
CREATE SCHEMA discovery;
REVOKE ALL ON SCHEMA discovery FROM PUBLIC;
CREATE TABLE discovery.bindings (
 store_id uuid PRIMARY KEY,
 merchant_id uuid NOT NULL,
 provider_id text NOT NULL,
 api_base text NOT NULL,
 public_key text NOT NULL
);
-- Public projections only: no references into private commerce/identity tables.
CREATE TABLE discovery.catalogs (
 store_id uuid PRIMARY KEY REFERENCES discovery.bindings(store_id),
 generation bigint NOT NULL CHECK(generation>0),
 digest bytea NOT NULL CHECK(octet_length(digest)=32),
 body jsonb NOT NULL CHECK(jsonb_typeof(body)='object' AND octet_length(body::text)<=1048576),
 observed_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
GRANT USAGE ON SCHEMA discovery TO ocp_app,ocp_work_owner;
GRANT SELECT ON discovery.bindings,discovery.catalogs TO ocp_app,ocp_work_owner;
GRANT INSERT,UPDATE ON discovery.catalogs TO ocp_work_owner;
-- +goose StatementBegin
CREATE FUNCTION discovery.accept_catalog(provider text,merchant uuid,store uuid,endpoint text,key text,version bigint,hash bytea,document jsonb)
RETURNS boolean LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE existing discovery.catalogs;
BEGIN
 IF version IS NULL OR version<1 OR hash IS NULL OR octet_length(hash)<>32 OR document IS NULL
 OR jsonb_typeof(document)<>'object' OR octet_length(document::text)>1048576
 OR NOT EXISTS(SELECT FROM discovery.bindings b WHERE b.store_id=store AND b.merchant_id=merchant AND b.provider_id=provider AND b.api_base=endpoint AND b.public_key=key)
 OR (document->>'store_id') IS DISTINCT FROM store::text
 OR (document->>'merchant_id') IS DISTINCT FROM merchant::text
 OR (document->>'provider_id') IS DISTINCT FROM provider
 OR (document->>'api_base') IS DISTINCT FROM endpoint
 OR (document->>'public_key') IS DISTINCT FROM key
 OR (document->>'generation')::bigint IS DISTINCT FROM version
 THEN RAISE EXCEPTION 'catalog binding rejected'; END IF;
 -- A per-store advisory lock avoids granting workers UPDATE on enrollment.
 PERFORM pg_advisory_xact_lock(hashtextextended(store::text,0));
 SELECT * INTO existing FROM discovery.catalogs WHERE store_id=store;
 IF FOUND THEN
  IF version<existing.generation THEN RETURN false; END IF;
  IF version=existing.generation THEN
   IF hash<>existing.digest OR document<>existing.body THEN RAISE EXCEPTION 'generation content conflict'; END IF;
   UPDATE discovery.catalogs SET observed_at=clock_timestamp() WHERE store_id=store;
   RETURN false;
  END IF;
 END IF;
 INSERT INTO discovery.catalogs(store_id,generation,digest,body) VALUES(store,version,hash,document)
 ON CONFLICT(store_id) DO UPDATE SET generation=EXCLUDED.generation,digest=EXCLUDED.digest,body=EXCLUDED.body,observed_at=clock_timestamp();
 RETURN true;
END $$;
-- +goose StatementEnd
ALTER FUNCTION discovery.accept_catalog(text,uuid,uuid,text,text,bigint,bytea,jsonb) OWNER TO ocp_work_owner;
REVOKE ALL ON FUNCTION discovery.accept_catalog(text,uuid,uuid,text,text,bigint,bytea,jsonb) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION discovery.accept_catalog(text,uuid,uuid,text,text,bigint,bytea,jsonb) TO ocp_app;
