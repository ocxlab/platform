-- +goose Up
CREATE EXTENSION IF NOT EXISTS btree_gist;
CREATE SCHEMA commerce_internal;
CREATE SCHEMA shopping;
CREATE SCHEMA lodging;
CREATE SCHEMA payments;
REVOKE ALL ON SCHEMA commerce_internal,shopping,lodging,payments FROM PUBLIC;
-- +goose StatementBegin
DO $$ BEGIN
 IF NOT EXISTS(SELECT FROM pg_roles WHERE rolname='ocp_internal_auth_owner') THEN
  CREATE ROLE ocp_internal_auth_owner NOLOGIN NOSUPERUSER NOBYPASSRLS;
 END IF;
END $$;
-- +goose StatementEnd
CREATE TABLE commerce_internal.grants (
 id uuid PRIMARY KEY,
 merchant_id uuid NOT NULL,
 store_id uuid NOT NULL,
 customer_id uuid NOT NULL,
 domain text NOT NULL CHECK(domain IN ('shopping','lodging')),
 audience text NOT NULL CHECK(audience='ucp-internal'),
 token_hash bytea NOT NULL UNIQUE CHECK(octet_length(token_hash)=32),
 profile_uri text NOT NULL,
 profile jsonb NOT NULL CHECK(jsonb_typeof(profile)='object'),
 expires_at timestamptz NOT NULL,
 revoked boolean NOT NULL DEFAULT false,
 management boolean NOT NULL DEFAULT false,
 FOREIGN KEY(merchant_id,store_id) REFERENCES commerce.stores(merchant_id,id)
);
-- +goose StatementBegin
CREATE FUNCTION commerce_internal.lookup_grant(candidate bytea)
RETURNS TABLE(id uuid,merchant_id uuid,store_id uuid,customer_id uuid,domain text,profile_uri text,profile jsonb,management boolean)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT g.id,g.merchant_id,g.store_id,g.customer_id,g.domain,g.profile_uri,g.profile,g.management
 FROM commerce_internal.grants g WHERE g.token_hash=candidate AND g.audience='ucp-internal' AND NOT g.revoked AND g.expires_at>clock_timestamp()
$$;
-- +goose StatementEnd
GRANT USAGE ON SCHEMA commerce_internal TO ocp_internal_auth_owner;
GRANT SELECT ON commerce_internal.grants TO ocp_internal_auth_owner;
ALTER FUNCTION commerce_internal.lookup_grant(bytea) OWNER TO ocp_internal_auth_owner;
REVOKE ALL ON FUNCTION commerce_internal.lookup_grant(bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION commerce_internal.lookup_grant(bytea) TO ocp_app;
CREATE TABLE commerce_internal.replays (
 merchant_id uuid NOT NULL,
 grant_id uuid NOT NULL REFERENCES commerce_internal.grants(id),
 domain text NOT NULL CHECK(domain IN ('shopping','lodging')),
 operation text NOT NULL,
 resource text NOT NULL,
 replay_key uuid NOT NULL,
 digest bytea NOT NULL CHECK(octet_length(digest)=32),
 response_status int NOT NULL,
 response_body bytea NOT NULL,
 PRIMARY KEY(merchant_id,grant_id,domain,operation,resource,replay_key)
);
CREATE TABLE shopping.catalogs (
 merchant_id uuid NOT NULL,
 store_id uuid NOT NULL,
 revision bigint NOT NULL CHECK(revision>0),
 document jsonb NOT NULL CHECK(jsonb_typeof(document)='object'),
 PRIMARY KEY(merchant_id,store_id),
 FOREIGN KEY(merchant_id,store_id) REFERENCES commerce.stores(merchant_id,id)
);
CREATE TABLE shopping.stock (
 merchant_id uuid NOT NULL,
 store_id uuid NOT NULL,
 sku text NOT NULL,
 available bigint NOT NULL CHECK(available>=0),
 PRIMARY KEY(merchant_id,store_id,sku),
 FOREIGN KEY(merchant_id,store_id) REFERENCES shopping.catalogs(merchant_id,store_id)
);
CREATE TABLE shopping.sessions (
 merchant_id uuid NOT NULL,
 store_id uuid NOT NULL,
 customer_id uuid NOT NULL,
 id uuid NOT NULL,
 revision bigint NOT NULL,
 body jsonb NOT NULL,
 request jsonb NOT NULL,
 PRIMARY KEY(merchant_id,store_id,id),
 FOREIGN KEY(merchant_id,store_id) REFERENCES shopping.catalogs(merchant_id,store_id)
);
CREATE TABLE shopping.orders (
 merchant_id uuid NOT NULL,
 store_id uuid NOT NULL,
 id uuid NOT NULL,
 session_id uuid NOT NULL,
 terms jsonb NOT NULL,
 PRIMARY KEY(merchant_id,store_id,id),
 UNIQUE(merchant_id,store_id,session_id),
 FOREIGN KEY(merchant_id,store_id,session_id) REFERENCES shopping.sessions(merchant_id,store_id,id)
);
CREATE TABLE lodging.properties (
 merchant_id uuid NOT NULL,
 store_id uuid NOT NULL,
 revision bigint NOT NULL CHECK(revision>0),
 document jsonb NOT NULL CHECK(jsonb_typeof(document)='object'),
 PRIMARY KEY(merchant_id,store_id),
 FOREIGN KEY(merchant_id,store_id) REFERENCES commerce.stores(merchant_id,id)
);
CREATE TABLE lodging.units (
 merchant_id uuid NOT NULL,
 store_id uuid NOT NULL,
 unit_id text NOT NULL,
 PRIMARY KEY(merchant_id,store_id,unit_id),
 FOREIGN KEY(merchant_id,store_id) REFERENCES lodging.properties(merchant_id,store_id)
);
CREATE TABLE lodging.sessions (
 merchant_id uuid NOT NULL,
 store_id uuid NOT NULL,
 customer_id uuid NOT NULL,
 id uuid NOT NULL,
 revision bigint NOT NULL,
 body jsonb NOT NULL,
 request jsonb NOT NULL,
 PRIMARY KEY(merchant_id,store_id,id),
 FOREIGN KEY(merchant_id,store_id) REFERENCES lodging.properties(merchant_id,store_id)
);
CREATE TABLE lodging.bookings (
 merchant_id uuid NOT NULL,
 store_id uuid NOT NULL,
 id uuid NOT NULL,
 session_id uuid NOT NULL,
 terms jsonb NOT NULL,
 PRIMARY KEY(merchant_id,store_id,id),
 UNIQUE(merchant_id,store_id,session_id),
 FOREIGN KEY(merchant_id,store_id,session_id) REFERENCES lodging.sessions(merchant_id,store_id,id)
);
CREATE TABLE lodging.allocations (
 merchant_id uuid NOT NULL,
 store_id uuid NOT NULL,
 unit_id text NOT NULL,
 booking_id uuid NOT NULL,
 check_in date NOT NULL,
 check_out date NOT NULL CHECK(check_out>check_in),
 PRIMARY KEY(merchant_id,store_id,unit_id,booking_id,check_in),
 FOREIGN KEY(merchant_id,store_id,unit_id) REFERENCES lodging.units(merchant_id,store_id,unit_id),
 FOREIGN KEY(merchant_id,store_id,booking_id) REFERENCES lodging.bookings(merchant_id,store_id,id),
 EXCLUDE USING gist (merchant_id WITH =,store_id WITH =,unit_id WITH =,daterange(check_in,check_out,'[)') WITH &&)
);
CREATE INDEX lodging_overlap ON lodging.allocations(merchant_id,store_id,unit_id,check_in,check_out);
-- Pay-later obligations are not successful charges. Shared payment references
-- avoid baking a Food order foreign key into the common financial boundary.
CREATE TABLE payments.obligations (
 merchant_id uuid NOT NULL,
 store_id uuid NOT NULL,
 domain text NOT NULL CHECK(domain IN ('shopping','lodging')),
 subject_id uuid NOT NULL,
 amount_minor bigint NOT NULL CHECK(amount_minor>=0 AND amount_minor<=100000000),
 currency text NOT NULL CHECK(currency='EUR'),
 condition text NOT NULL CHECK(condition='pay_later'),
 state text NOT NULL CHECK(state='outstanding'),
 PRIMARY KEY(merchant_id,store_id,domain,subject_id),
 FOREIGN KEY(merchant_id,store_id) REFERENCES commerce.stores(merchant_id,id)
);
-- +goose StatementBegin
DO $$
DECLARE target text;
BEGIN
 FOREACH target IN ARRAY ARRAY['commerce_internal.replays','shopping.catalogs','shopping.stock','shopping.sessions','shopping.orders','lodging.properties','lodging.units','lodging.sessions','lodging.bookings','lodging.allocations','payments.obligations'] LOOP
  EXECUTE format('ALTER TABLE %s ENABLE ROW LEVEL SECURITY',target);
  EXECUTE format('ALTER TABLE %s FORCE ROW LEVEL SECURITY',target);
  EXECUTE format('CREATE POLICY merchant_scope ON %s USING (merchant_id=nullif(current_setting(''ocp.merchant_id'',true),'''')::uuid) WITH CHECK (merchant_id=nullif(current_setting(''ocp.merchant_id'',true),'''')::uuid)',target);
  EXECUTE format('GRANT SELECT,INSERT ON %s TO ocp_app',target);
 END LOOP;
END $$;
-- +goose StatementEnd
GRANT USAGE ON SCHEMA commerce_internal,shopping,lodging,payments TO ocp_app;
GRANT UPDATE ON shopping.catalogs,shopping.stock,shopping.sessions,lodging.properties,lodging.sessions TO ocp_app;
