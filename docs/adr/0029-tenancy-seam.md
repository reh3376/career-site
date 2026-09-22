# 0029. A tenancy seam, not a tenancy layer

Status: Accepted 2026-09-22

## Context

The site serves one person and is expected to keep doing so for a
while. A go-to-market plan drafted the same day proposes turning the
JD reviewer into a multi-tenant product, with a site per job seeker,
cross-tenant identity for hiring-side users, and the labeled decision
data as the asset that makes the product worth something.

The owner's decision on that plan was deliberate: build depth on one
corpus first, treat the working product as the proof, and defer the
platform until there is funding to build it properly. He also asked
that the work done in the meantime should not have to be undone.

Those two things pull against each other. Multi-tenancy touched
casually is a liability: half-applied tenant checks read as isolation
without providing it. Multi-tenancy ignored entirely is also a
liability, because the rows accumulating right now are the asset, and a
row with no owner cannot be given one later with any confidence.

The cost asymmetry decides it. `events`, `decision_log` and `llm_usage`
are small today, tens to hundreds of rows. Adding a column and moving
three indexes costs one migration. Doing the same after a year of
rows, and after every query, view and index has been written without
it, costs a migration plus an audit of everything that reads them.

## Decision

Add the seam. Do not add the machinery.

The seam is: a `tenants` table with the owner's site as row 1; a
`tenant_id` column on the data-layer tables, which every table added by
D2 through D5 also carries from birth; and an `internal/tenant` package
whose `FromContext` every writer calls instead of hardcoding the id.
Indexes lead with `tenant_id` so they keep their shape later.

The machinery, explicitly not built: row-level security, tenant
resolution from the request host, tenant-scoped authentication and
authorization, per-tenant settings, and any user interface that admits
more than one site exists.

`FromContext` returns the default today because nothing sets anything
else. The value is that the call sites are already correct, so the day
a host resolver is written, one function changes rather than every
insert statement.

## Consequences

Writers must take a context and ask it for the tenant, which they
already do for cancellation, so this costs nothing at the call site.

The column defaults to 1 at the database level as well as being passed
explicitly. The default is for rows written by anything not yet
updated, and for the existing rows the migration backfills. The
explicit pass is what makes the code honest. Both together mean a
missed call site is a wrong attribution rather than a failed insert.

This buys attribution, not isolation. Knowing which tenant a row
belongs to is not the same as preventing one tenant from reading
another's, and nothing here should be mistaken for the latter. When
isolation is needed it belongs in Postgres row-level security keyed on
this same id, enforced by the database rather than by a condition a
handler might forget. That work is a project of its own and is out of
scope until the platform is funded.

The core tables that predate this decision, `users`, `jd_submissions`,
`corpus_documents` and the rest, do not carry `tenant_id` yet. They are
request-serving tables rather than the analytical asset, and they will
be retrofitted as part of the isolation work if it ever happens. The
data layer is the part that could not be reconstructed, so it is the
part that gets the column now.
