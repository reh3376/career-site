# packages/schema

JSON Schema for a content model that was designed in Phase 0 and never wired up.

**Nothing reads this file.** Audited 2026-10-02: no validator imports it, there is no `career-cli content validate`, no `content-validate.yml` in CI, no `fixtures/` directory, and no build step references the package. It is one schema file and this README.

## Why it is still here

The schema describes a database-backed content model: every item carrying `id`, `slug`, `type`, `summary`, `visibility` and `chatbot_include`, validated at build time, with per-type schemas for `role`, `project`, `article`, `presentation`, `skill`, `credential`, `photo`, `resume` and `qa`. That is still a reasonable design, and `visibility` plus `chatbot_include` in particular turned out to matter: both concepts are live today, as columns on the corpus tables rather than as frontmatter.

What actually shipped is simpler. Articles are markdown files under `apps/web/content/articles/`, published through an explicit allow-list in `apps/web/src/lib/articles.ts`, with frontmatter typed in TypeScript at the point of use. Their real frontmatter is `title`, `subtitle`, `author`, `date`, `series`, `part`, `tags` — so none of the committed articles would validate against this schema, which requires six fields they do not have. The corpus that Ask Roger retrieves from is a separate path again: Postgres rows with embeddings, not files.

## Before using or removing it

Two honest options, and the choice has not been made:

- **Remove the package.** An unused schema that disagrees with the content it nominally describes is worse than no schema, because the next person to find it will reasonably assume it is enforced.
- **Wire it up as part of the content CMS** (`docs/backlog.md` item 11, the admin content management UI that replaces the allow-list). If the allow-list becomes a database, validation stops being optional, and this file is a sound starting point — but it would need reconciling against the frontmatter that exists and against the corpus columns that already implement `visibility` and `chatbot_include`.

Either way, do not treat the schema as authoritative about current content. `apps/web/src/lib/articles.ts` is.

## If it is revived

- Schema version: JSON Schema draft 2020-12.
- Never widen a field's type without a migration for existing content.
