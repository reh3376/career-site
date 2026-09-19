# packages/schema

JSON Schema definitions for the content model (`content/**`). Every `.md`, `.mdx`, or `.yaml` file under `content/` must validate against one of the type schemas here at build time; invalid content fails CI with a file- and field-level message (FSD FR-CNT-16).

## Layout

```
packages/schema/
├── content-common.schema.json    shared frontmatter every content item carries
└── types/                        one schema per content type (Phase 2)
```

## Status

**Phase 0 scaffold.** Only the shared-frontmatter schema exists today; the per-type schemas (`role`, `project`, `article`, `presentation`, `skill`, `credential`, `photo`, `resume`, `qa`) land alongside the content itself in Phase 2 (FSD FR-CNT-01…20).

## Editing

- Schema version: JSON Schema draft 2020-12.
- Add a required field: update `content-common.schema.json`, add fixtures under `packages/schema/fixtures/`, then update the validator in the sidecar CLI (`career-cli content validate`).
- Never widen a field's type without a migration for existing content.

## Where consumers live

- `services/sidecar` — `career-cli content validate` (Phase 2 wiring)
- `apps/web` — content-loading code will use the same schema to type MDX frontmatter at build time (Phase 2)
- CI — `content-validate.yml` (Phase 2) will fail on any schema violation before merge
