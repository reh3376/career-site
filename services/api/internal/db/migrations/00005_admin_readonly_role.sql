-- +goose Up
-- +goose StatementBegin

-- Dedicated read-only role for the /admin/db surface. The app's
-- normal pool connects as `career` (owner of the schema, full DML).
-- The SQL-console (adminquery.Run) enforces SELECT-only in Go, but a
-- bug there would let an admin session touch anything the connecting
-- role can touch. This role bounds the blast radius: even a total
-- guard bypass can't INSERT / UPDATE / DELETE / DDL.
--
-- The role is created NOLOGIN so an unattended `CREATE ROLE` never
-- opens a network foothold before the deploy owner sets a password.
-- The API's boot code flips it to LOGIN + PASSWORD when
-- DB_READONLY_PASSWORD is present in the environment.

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'career_admin_readonly') THEN
    CREATE ROLE career_admin_readonly
      NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE INHERIT;
  END IF;
END
$$;

-- USAGE on the schema itself is required before any table grant works.
GRANT USAGE ON SCHEMA public TO career_admin_readonly;

-- SELECT on every existing public table. Sequences too — some queries
-- pull nextval() for informational purposes; giving USAGE-only (not
-- UPDATE) keeps it read-only.
GRANT SELECT ON ALL TABLES IN SCHEMA public TO career_admin_readonly;
GRANT USAGE ON ALL SEQUENCES IN SCHEMA public TO career_admin_readonly;

-- Default privileges so tables added by *future* migrations
-- (whichever role runs them) also carry SELECT for the readonly role.
-- We set both defaults: FOR ROLE career covers the app-run migrations,
-- and unqualified covers anything a DBA runs by hand.
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT SELECT ON TABLES TO career_admin_readonly;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT USAGE ON SEQUENCES TO career_admin_readonly;
ALTER DEFAULT PRIVILEGES FOR ROLE career IN SCHEMA public
  GRANT SELECT ON TABLES TO career_admin_readonly;
ALTER DEFAULT PRIVILEGES FOR ROLE career IN SCHEMA public
  GRANT USAGE ON SEQUENCES TO career_admin_readonly;

-- Belt and suspenders: explicitly REVOKE any write privileges that a
-- future migration might carelessly grant via PUBLIC. This is a no-op
-- today (we never GRANT ... TO PUBLIC) but keeps the guard honest.
REVOKE INSERT, UPDATE, DELETE, TRUNCATE, REFERENCES, TRIGGER
  ON ALL TABLES IN SCHEMA public FROM career_admin_readonly;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA public FROM career_admin_readonly;
REVOKE ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public FROM career_admin_readonly;
REVOKE USAGE ON SCHEMA public FROM career_admin_readonly;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  REVOKE SELECT ON TABLES FROM career_admin_readonly;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  REVOKE USAGE ON SEQUENCES FROM career_admin_readonly;
ALTER DEFAULT PRIVILEGES FOR ROLE career IN SCHEMA public
  REVOKE SELECT ON TABLES FROM career_admin_readonly;
ALTER DEFAULT PRIVILEGES FOR ROLE career IN SCHEMA public
  REVOKE USAGE ON SEQUENCES FROM career_admin_readonly;
DROP ROLE IF EXISTS career_admin_readonly;

-- +goose StatementEnd
