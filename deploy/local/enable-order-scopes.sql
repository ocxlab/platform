-- Explicit development-only upgrade for existing walkthrough credentials.
-- Never run as an application migration or against a production database.
DO $$ BEGIN
  IF current_database() <> 'ocp_dev' OR current_user <> 'ocp_dev' THEN
    RAISE EXCEPTION 'This bootstrap is restricted to the local development database';
  END IF;
END $$;
UPDATE auth.grants
SET scopes = ARRAY(SELECT DISTINCT unnest(scopes || ARRAY['order:read','order:manage']))
WHERE NOT revoked AND expires_at>CURRENT_TIMESTAMP
  AND 'offering:write'=ANY(scopes) AND 'store:read'=ANY(scopes);
UPDATE auth.provider_grants
SET scopes = ARRAY(SELECT DISTINCT unnest(scopes || ARRAY['order:inspect']))
WHERE NOT revoked AND expires_at>CURRENT_TIMESTAMP AND 'publication:manage'=ANY(scopes);
