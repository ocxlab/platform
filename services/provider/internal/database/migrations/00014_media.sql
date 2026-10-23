-- +goose Up
CREATE TABLE commerce.media_assets (
 merchant_id uuid NOT NULL,
 store_id uuid NOT NULL,
 id uuid NOT NULL,
 content_type text NOT NULL CHECK(content_type IN ('image/jpeg','image/png','image/webp')),
 size_bytes bigint NOT NULL CHECK(size_bytes BETWEEN 1 AND 10485760),
 checksum text NOT NULL,
 state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','ready')),
 width integer,
 height integer,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 expires_at timestamptz NOT NULL DEFAULT clock_timestamp()+interval '1 day',
 completed_at timestamptz,
 actor_id uuid NOT NULL,
 PRIMARY KEY(merchant_id,store_id,id),
 FOREIGN KEY(merchant_id,store_id) REFERENCES commerce.stores(merchant_id,id),
 CHECK ((state='ready')=(completed_at IS NOT NULL)),
 CHECK ((state='ready')=(width IS NOT NULL AND height IS NOT NULL))
);
ALTER TABLE commerce.media_assets ENABLE ROW LEVEL SECURITY;
ALTER TABLE commerce.media_assets FORCE ROW LEVEL SECURITY;
CREATE POLICY merchant_scope ON commerce.media_assets
 USING(merchant_id=nullif(current_setting('ocp.merchant_id',true),'')::uuid)
 WITH CHECK(merchant_id=nullif(current_setting('ocp.merchant_id',true),'')::uuid);
GRANT SELECT,INSERT,UPDATE ON commerce.media_assets TO ocp_app;
