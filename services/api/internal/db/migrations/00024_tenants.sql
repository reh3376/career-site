-- +goose Up
-- +goose StatementBegin

-- The tenancy seam, not the tenancy machinery.
--
-- The site is one person's and stays that way for now. What this
-- migration buys is that the rows accumulating today, the ones that
-- are the actual asset (the labeled decisions, the model telemetry,
-- the behaviour stream), can be attributed later without a rewrite.
-- Adding the column now costs one migration against small tables.
-- Adding it after a year of rows, and after every query and index has
-- been written without it, costs a great deal more.
--
-- Deliberately NOT here: row-level security, per-request tenant
-- resolution from the host, tenant-scoped auth. Those are the
-- expensive parts and they are not needed to serve one tenant. The
-- Go side calls tenant.FromContext everywhere instead of hardcoding
-- the id, so when resolution does arrive it changes one function.

CREATE TABLE tenants (
  id           bigserial PRIMARY KEY,
  -- Host label: the subdomain a seeker's site would answer on.
  slug         text        NOT NULL UNIQUE,
  display_name text        NOT NULL,
  -- active | suspended. No plan column yet; billing is not a thing.
  status       text        NOT NULL DEFAULT 'active',
  created_at   timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE tenants IS
  'One row per site served by this stack. Tenant 1 is the owner''s own site.';

INSERT INTO tenants (id, slug, display_name)
VALUES (1, 'rogerhenley', 'Roger Henley');
SELECT setval('tenants_id_seq', 1, true);

-- The data layer carries the tenant. These three tables are the ones
-- whose rows have value beyond serving a request: the event stream,
-- the labeled decisions, and the model telemetry. Every table D2
-- through D5 adds carries tenant_id from birth.
--
-- DEFAULT 1 keeps existing rows and any writer that has not been
-- updated correct rather than broken, but the Go writers pass the id
-- explicitly so the call sites are already tenant-aware.
ALTER TABLE events
  ADD COLUMN tenant_id bigint NOT NULL DEFAULT 1 REFERENCES tenants(id);
ALTER TABLE decision_log
  ADD COLUMN tenant_id bigint NOT NULL DEFAULT 1 REFERENCES tenants(id);
ALTER TABLE llm_usage
  ADD COLUMN tenant_id bigint NOT NULL DEFAULT 1 REFERENCES tenants(id);

-- Indexes lead with the tenant so they stay useful when there is more
-- than one. With a single tenant the leading constant costs nothing
-- measurable at this size, and it saves rebuilding them later.
DROP INDEX IF EXISTS idx_events_name_time;
CREATE INDEX idx_events_tenant_name_time ON events (tenant_id, name, occurred_at DESC);
CREATE INDEX idx_decision_log_tenant ON decision_log (tenant_id, id DESC);
CREATE INDEX idx_llm_usage_tenant_created ON llm_usage (tenant_id, created_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_llm_usage_tenant_created;
DROP INDEX IF EXISTS idx_decision_log_tenant;
DROP INDEX IF EXISTS idx_events_tenant_name_time;
CREATE INDEX idx_events_name_time ON events (name, occurred_at DESC);

ALTER TABLE llm_usage DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE decision_log DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE events DROP COLUMN IF EXISTS tenant_id;

DROP TABLE IF EXISTS tenants;

-- +goose StatementEnd
