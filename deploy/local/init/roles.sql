-- Local-only role provisioning, executed explicitly through task db:prepare.
-- Reapplication preserves user data; these are development credentials only.
DO $$ BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'ocp_app') THEN
    CREATE ROLE ocp_app LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS;
  END IF;
END $$;
ALTER ROLE ocp_app PASSWORD 'ocp_app_local_only';
