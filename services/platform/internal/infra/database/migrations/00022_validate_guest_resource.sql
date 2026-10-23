-- +goose Up
SET LOCAL lock_timeout = '2s';
SET LOCAL statement_timeout = '2min';
-- Separate transaction from ADD CONSTRAINT: validation does not retain its
-- ACCESS EXCLUSIVE lock while scanning existing guest rows.
ALTER TABLE commerce.guest_grants VALIDATE CONSTRAINT guest_resource;
-- +goose Down
SELECT 1;
