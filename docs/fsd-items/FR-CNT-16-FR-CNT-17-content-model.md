# FR-CNT-16 + FR-CNT-17: the content model

**Built 2026-10-12.** The state this was written from is kept below the
outcome, unedited.

## Outcome

Decision #1 was taken as "close the validation half, skip the fields
nothing reads". Shipped 2026-10-12.

**What was built.** `scripts/check_content.py` validates every file
under `apps/web/content/` against a per-kind schema and runs in CI
before the build. It fails on a missing required key, an unknown key (a
typo), a value outside the kind's enum, a non-numeric value where a
number belongs, a `type` that disagrees with the directory, and a
duplicate `id`. All 16 files pass. The schema is authored from the
front matter that exists, per the 2026-10-02 deletion commit's own
guidance, not from §7.2.

**`id` and `type` added to all 16 files** and to both readers' types.
`id` is authored and survives a rename; the slug is derived, from
`PUBLIC_ARTICLES` for articles and from the filename for photos. That
is the distinction FR-CNT-17 draws by listing both.

**Three fields deliberately not added**, and named as deferred in the
FSD row rather than left implied: `tracks`, `related`, `summary`.
Nothing reads any of them. `tracks` waits on FR-PROF-03, the only
requirement that would consume per-track weights, which is not built, so
its shape would be guessed now with no consumer to check it against.
`hero_tracks` is already that mistake, parsed and typed on every photo
and read by nothing.

**The parsers had drifted, which is the finding this item turned up.**
Decision #4 of the checklist asked for either one shared parsing layer
or a test comparing them. Both, in the end:

- The two TypeScript parsers were near-identical duplicates and are now
  one, `apps/web/src/lib/frontmatter.ts`.
- That one, the Go walker and this script's Python parser are asserted
  against a single fixture, `testdata/frontmatter/`.

Comparing them found two real differences, neither of which mattered for
the content committed today, which is exactly why neither was noticed:
`articles.ts` did not normalise CRLF, so a file saved with Windows line
endings parsed to an empty map there and a full one everywhere else; and
it stripped a lone `"` to an empty string where the other two left it
alone. The Go parser's comment claimed the two were "kept in sync by
convention".

**Decision #3, the three `visibility` vocabularies**, was answered by
enforcement rather than by collapsing them. `member`/`public` gates the
gallery, `public`/`corpus_only` gates corpus retrieval, and the FSD's
own row says `landing`/`member`. Collapsing them would change what the
assistant retrieves and what the gallery shows, which is a behaviour
change and does not belong in a validation pass. The checker now refuses
a value that is not valid for that kind, so the three stay distinct by
enforcement instead of by nobody noticing, and the reasoning is written
at the top of the script.

**Decision #5, photos in the corpus, was deliberately not taken here.**
Adding `photos` to `publicCorpusSubdirs` is one line, but it changes
what Ask Roger and the JD reviewer can see, and three of the nine photos
are `visibility: member`. It is in `docs/backlog.md` with that
reasoning rather than slipped in alongside a schema change.

**Decision #6, normalising article front matter**, was answered by
making `author`/`series`/`part`/`tags` optional for the kind. The
five-part series carries them and the standalone teaser post does not;
forcing a series name onto a standalone post would be inventing data to
satisfy a schema.

**Verified by mutation, not by assumption.** A typo'd key, a bad enum
value and a non-numeric `order` each produce a named failure; removing
the quote-length guard or the CRLF normalisation from the TypeScript
parser fails the parity test, and weakening the Go parser's guard fails
the Go half.

**Also corrected:** `articles.ts` pointed at "backlog item #11", which
does not match the backlog's structure; it now names the section and
entry.

---

**State as of 2026-10-12, before the build.** Both requirements describe one artifact — a content model with a shared, build-time-enforced schema (CNT-16) and a defined field set every item carries (CNT-17) — and neither exists as specified. Sixteen content items (6 articles, 1 site-guide document, 9 photos) live as Markdown files with hand-authored YAML frontmatter under `apps/web/content/`, read by two independent, minimal, hand-written TypeScript parsers that each type only the two or three fields their page needs and silently discard the rest. Nothing validates frontmatter against a schema, at build time or otherwise: the one artifact that tried, `packages/schema/`, was deleted on 2026-10-02 because nothing imported it and it required six fields (`id`, `slug`, `type`, `summary`, `visibility`, `chatbot_include`) that not one committed file carried. Of the eleven fields CNT-17 lists, roughly half exist today in some form, under inconsistent names, inconsistent types, and inconsistent presence across items of the same type; `id`, `type`, `summary`, and `related` exist nowhere, and `tracks` (per-item weights) exists nowhere, which is consistent with FR-PROF-03 — the only requirement that would consume `tracks` — also being unbuilt.

## What the FSD requires

- **FR-CNT-16** (`docs/FSD.md:350`): "**Content model.** All content shall be stored under `content/` as Markdown/MDX or YAML with frontmatter validated against a shared schema at build time." Status column: "Must (**Partial**)."
- **FR-CNT-17** (`docs/FSD.md:351`): "Each content item shall carry: `id`, `slug`, `type`, `title`, `summary`, dates, `tracks` (weights), `tags`, `visibility` (`landing` or `member`), `chatbot_include`, `assets`, and `related` links." Status column: "Must (partially **Shipped**)."
- **§7.2 "Content types and frontmatter"** (`docs/FSD.md:728-772`) is the fuller specification both rows point at: a common-fields YAML block (`id`, `slug`, `type`, `title`, `summary`, `published`/`updated`, `tracks` as a weight map, `tags`, `visibility: member`, `chatbot_include`, `related`, `assets`) plus a type-specific-fields table for nine content types (`role`, `project`, `article`, `presentation`, `skill`, `credential`, `photo`, `resume`, `qa`). The section's own status line (`docs/FSD.md:732`) states: "The content model below is specification and has not been implemented." `content/` as a top-level directory does not exist; the real path is `apps/web/content/`.
- **FR-PROF-03** (`docs/FSD.md:325`), coupled to CNT-17's `tracks` field: "Every content item shall carry track weights (0–1 per track) and the home page and section listings shall order and highlight items by relevance to the member's declared tracks." Status: "**Not built.** Content items carry no track weights."

## What exists today

**Directory shape.** `apps/web/content/` has three subdirectories and no other top-level files:
- `articles/` — 6 files: `better-business-decisions-part-1.md` through `-part-5.md`, and `manufacturing-hub-teaser-post.md`.
- `other/` — 1 file: `career-site-guide.md` (the site's own "how this site works" document, fed to the assistant as corpus, not published as an article).
- `photos/` — 9 Markdown files (one per photo) plus `manifest.json`, a derivative-dimensions index written by `scripts/photos-build.mjs`, not a content item itself.

**Frontmatter actually carried, by type, read from the files directly:**

- *Articles* (`apps/web/content/articles/better-business-decisions-part-1.md:1-14`, `-part-5.md:1-11`): `title`, `subtitle`, `author`, `date` (a bare year, e.g. `2026`), `series`, `part`, `tags[]`. But this shape is not uniform across the type: `manufacturing-hub-teaser-post.md:1-5` carries only `title`, `subtitle`, `date` — no `author`, `series`, `part`, or `tags`. Two files of the same declared type carry different field sets.
- *Photos* (e.g. `apps/web/content/photos/bootcamp-1987.md:1-11`, `column-top-bubbletray.md:1-11`): `title`, `caption`, `context`, `taken` (a bare year or short date string), `alt`, `permission`, `hero_tracks` (a comma-separated string, e.g. `"automation, distilling"`), `visibility` (`"member"` or `"public"`), `order` (integer), `source` (an image filename). This set is uniform across all 9 photo files.
- *Other* (`apps/web/content/other/career-site-guide.md:1-4`): `title`, `visibility` only.
- No file of any type carries `id`, `type`, `summary`, or `related`, anywhere in the corpus.

**How frontmatter is parsed and typed — two independent readers, neither validating:**

- `apps/web/src/lib/articles.ts:85-102` (`parseFrontMatter`) is a hand-rolled line-by-line `key: value` regex parser (no YAML library), producing an untyped `Record<string, string>`. `readOne()` (`articles.ts:112-130`) reads that map and extracts exactly three fields — `meta.title`, `meta.subtitle`, `meta.date` — into the typed `ArticleSummary`/`Article` (`articles.ts:53-67`). `series`, `part`, `tags`, and `author` are parsed into the intermediate map and then discarded; nothing in the codebase reads them (confirmed by grep across `apps/web/src/lib/articles.ts` and the `/articles` page components — the only matches are the literal strings `"part-1"`…`"part-5"` in the slug allow-list, not the frontmatter field). Published articles are additionally gated by an explicit allow-list, `PUBLIC_ARTICLES` (`articles.ts:14-45`), not by any frontmatter field — a file dropped into `content/articles/` without an entry here is never served.
- `apps/web/src/lib/photos.ts:30-49` has its own separate, near-identical hand-rolled parser. `listPhotos()` (`photos.ts:51-91`) types all nine photo fields into `Photo` (`photos.ts:9-24`): `title`, `caption`, `context`, `taken`, `alt`, `permission`, `heroTracks` (split from the comma string), `visibility`, `order`, plus `width`/`height` looked up from `manifest.json` by slug. A file missing `permission` or `alt`, or missing a `manifest.json` entry, is silently skipped (`photos.ts:66,68`) rather than failing the build.
- Neither parser is schema validation: both are compile-time TypeScript typing of the one or two fields each call site happens to read. A file with a typo'd key, a missing required-by-convention field, or an extra unexpected field builds and deploys without any signal. There is no `career-cli content validate`, no `content-validate.yml` CI step, and no other validator anywhere in the repository (confirmed by the `packages/schema/` deletion commit's own audit, see below).

**`packages/schema/` — deleted 2026-10-02, commit `a5c8df6` ("chore: delete packages/schema").** It held one JSON Schema file (`content-common.schema.json`) and a README; nothing imported either — no validator, no `career-cli content validate`, no `content-validate.yml` in CI, no fixtures directory, no reference from any workspace or build config. The commit message states deleting it "changes no behaviour." It was removed for two reasons: it was unused, and it was wrong — it required six frontmatter fields (`id`, `slug`, `type`, `summary`, `visibility`, `chatbot_include`) that no committed article carried, so not one content file would have validated against it had it ever been wired up. The commit also corrected the FSD: FR-CNT-16 had been marked Shipped until that commit, which overstated it, since storage was done but validation never was. Two of the deleted schema's ideas survived elsewhere, as database columns rather than frontmatter: `visibility` and `chatbot_include` are live on the corpus tables (see below), not on content files. The commit's own guidance for a future rebuild: trigger it from the admin content CMS that replaces the allow-list, and re-author the schema against the frontmatter that actually exists rather than restoring the deleted file.

**FR-CNT-17's eleven fields, checked individually against the above:**

| Field | Exists today? | Detail |
|---|---|---|
| `id` | No | Not present in any file's frontmatter, any type definition, or any reader. |
| `slug` | Partial | Articles: yes, but as a reader-side constant in `PUBLIC_ARTICLES` (`articles.ts:14-45`), not frontmatter. Photos: yes, derived from the filename (`photos.ts:64`), not frontmatter. Neither is a frontmatter field. |
| `type` | No | No file declares its own content type; type is implicit in which directory/reader it lives under. |
| `title` | Yes | Present and read for all three content kinds. |
| `summary` | No | Articles have `subtitle` (a different field, not a ≤240-char summary per §7.2); photos have `caption`/`context`; neither is named or shaped as `summary`. |
| dates | Partial, inconsistent | Articles: single `date` field, a bare year or ISO-like string (`articles.ts:58-61`). Photos: single `taken` field, same loose shape. §7.2's fuller spec wants a `published`/`updated` pair; neither type has it. |
| `tracks` (weights) | No | Photos have `hero_tracks`, a flat comma-separated tag string (`photos.ts:17,77-80`), not a `{track: weight}` map. It is parsed into the `Photo` type's `heroTracks: string[]` but nothing downstream reads it — no component, page, or sort references `heroTracks` outside its own definition and parse site (confirmed by grep across `apps/web/src`). FR-CNT-12 ("hero image may vary by track," `docs/FSD.md:346`) is the one feature that would consume it and is itself "**Not built.** Depends on FR-PROF tracks, which are not built." Articles have no track-like field at all. |
| `tags` | Partial | Articles: yes, a flat list (present on 5 of 6 article files; the teaser post has none). Photos: no `tags` field (has `hero_tracks` instead, a different vocabulary). |
| `visibility` (`landing`\|`member`) | Partial, and overloaded | Photos: yes, a frontmatter field, but valued `"public"`/`"member"` (`photos.ts:81`), not CNT-17's literal `landing`/`member`. Articles: no frontmatter `visibility` field at all — publication is controlled entirely by the `PUBLIC_ARTICLES` allow-list, which has no gated state. This is a second, unrelated meaning of "visibility" from the one used for corpus retrieval (see below) — same word, two different mechanisms, two different value sets. |
| `chatbot_include` | No (frontmatter) / Yes (DB column) | Never a frontmatter field on any content file. It is a live column on the corpus tables, read at retrieval time in `services/api/internal/users/chat_retrieval.go:21,55` and gated in `services/api/internal/chat/answer.go:264` — set when a document is ingested into the corpus, not declared per-item in `apps/web/content/`. |
| `assets` | No | No frontmatter field by this name. Photos have `source` (an original filename) and a separate `manifest.json` carrying derivative sizes/dimensions, built by `scripts/photos-build.mjs` — a different mechanism (a build-time derivative pipeline) from a declared `assets:` list. |
| `related` | No | No file, type, or reader has a related-items field. |

**What reads content frontmatter, which constrains any future schema:**

1. `apps/web/src/lib/articles.ts` → the `/articles` list and `/articles/[slug]` detail pages, and `apps/web/src/app/sitemap.ts:26-39` (reads `listArticles()`'s `slug`/`date` to build sitemap entries, falling back to "now" on an unparseable date, `sitemap.ts:44-49`).
2. `apps/web/src/lib/photos.ts` → the `/gallery` page, which also applies its own member/public filter on `p.visibility` in TypeScript (`apps/web/src/app/(site)/gallery/page.tsx:22`) — a second visibility check on top of whatever `listPhotos()` already parsed.
3. The Go corpus ingester, `services/api/internal/ingest/walker.go`, has its own third, independent, regex-based frontmatter parser (`walker.go:187-227`), kept "in sync by convention" with the TypeScript one per its own comment (`walker.go:201`) — not by any shared schema or code. It extracts exactly two keys from frontmatter: `title` and `visibility` (`walker.go:95,105`). Every other frontmatter field (`subtitle`, `date`, `series`, `tags`, `caption`, `hero_tracks`, etc.) is invisible to the corpus/RAG path entirely; only the title and a visibility override reach the database. `visibility` here means a third thing again: `public` vs. `corpus_only` (a DB-level retrieval-permission axis, `walker.go:19-23,169-180`), distinct from both the photos' `member`/`public` display gate and CNT-17's `landing`/`member` vocabulary.
4. Which directories the ingester reads at all is governed by a separate, hand-maintained map, `publicCorpusSubdirs` (`services/api/internal/handlers/admin.go:612-625`): `{"article": "articles", "other": "other"}`. **`photos/` has no entry in this map and is never walked by the public corpus reindex** — photo frontmatter (captions, context, permissions) is not retrievable by Ask Roger or the JD reviewer at all, confirmed independently of the field-level gaps above. The map's own comment (`admin.go:609-611,619-623`) documents why: the committed content tree predates the kind-naming scheme, so an un-mapped directory is silently skipped rather than erroring, and this already happened once in practice ("the first attempt added the file and reindexed, and the job reported 'kinds article' and ingested nothing").

## What is missing

- A single, authored schema (any format — JSON Schema, Zod, Pydantic) that any reader actually imports and checks against. None exists; `packages/schema/` was the one attempt and was unused before it was deleted.
- A build step, CI job, or CLI command that runs that schema against every content file and fails the build on a mismatch. No `content-validate.yml`, no `career-cli`, no equivalent exists anywhere in the repository's CI configuration.
- A unified frontmatter reader. Three independent hand-rolled parsers exist today (`apps/web/src/lib/articles.ts`, `apps/web/src/lib/photos.ts`, `services/api/internal/ingest/walker.go`), each recognizing a different subset of keys, kept aligned only by convention and comment, not by shared code or a shared type.
- `id` and `type` fields, on every content file, with any value at all.
- A `summary` field distinct from the display-oriented `subtitle`/`caption`/`context` fields that exist today.
- A `tracks` weight map on any content item, and therefore the one thing FR-PROF-03 needs to do anything at all.
- A `related` field and any mechanism for cross-linking content items.
- An `assets` field in the §7.2 sense (a declared list of referenced asset paths), separate from the photo-specific `source`/`manifest.json` derivative-build mechanism that exists today.
- A `chatbot_include` value settable per content item at authoring time — today it is set at ingest time as a DB column, with no path back to a frontmatter declaration.
- One consistent `visibility` vocabulary. Three different value sets currently share the word: photo frontmatter (`member`/`public`), corpus DB rows (`public`/`corpus_only`), and the FSD's own target (`landing`/`member`).
- A mapped, ingested `photos/` corpus directory, if photo content is meant to be retrievable by the assistant at all (currently it is not, by omission from `publicCorpusSubdirs`, independent of any frontmatter-field gap).
- Uniform frontmatter within a single declared type — the article type alone currently has two different field sets in active use (5 files with `author`/`series`/`part`/`tags`, 1 file without).

## Dependencies and constraints

- **ADR 0031** (`docs/adr/0031-declared-controls-are-enforced-controls.md`), accepted 2026-10-12, names the general failure pattern this pair of requirements exhibits — "a control that is declared is enforced, or it is not declared" — though CNT-16/17 are not among the ADR's own ten named examples (that list is security controls: UFW, auth levels, MFA, rate limits, `buf.validate`, Turnstile, CSRF, audit log). The ADR's distinction between a control and "a roadmap item, a design intention, an FSD row describing something unbuilt" (ADR 0031, "Consequences") is the relevant test here: §7.2's full schema is explicitly marked as specification, not a claim about current behavior, so it is not itself a violation of the ADR — but the *FSD status line* for FR-CNT-16, before the 2026-10-02 correction, was exactly that failure mode (a row that read as "Shipped" describing validation that had never run).
- **FR-PROF-03 is not built**, and it is CNT-17's `tracks` field's only consumer. Adding a `tracks` weight map to the content model today would have zero code reading it, the same shape `hero_tracks` is already in (parsed, typed, never consumed) — this is a real argument for sequencing `tracks` after FR-PROF-03's design is settled, or for building both together, rather than adding an inert field now.
- **The admin content management UI** (`docs/backlog.md:659-663`, under "4. Admin console") is the trigger §7.2 names (`docs/FSD.md:738`) for building the content model: "upload articles, presentations and documents to MinIO plus a `content` row; the public articles reader then reads from the database instead of the filesystem allow-list." Validation stops being optional once content is admin-authored rather than hand-edited by the owner, per that same note. This backlog entry is an unnumbered bullet, not item "11" as §7.2's comment and `apps/web/src/lib/articles.ts:11` both describe it — the numbering in those two references does not match the current `docs/backlog.md` structure (see "What could not be verified").
- **Three independent consumers would all need to change together**: the TypeScript articles reader, the TypeScript photos reader, and the Go corpus walker each have their own parsing logic and their own subset of fields they care about. A schema change that does not also unify (or explicitly keep separate) these three readers reproduces the current drift in a new shape.
- **The corpus walker's `publicCorpusSubdirs` map is hand-maintained and kind-naming predates the committed content tree** (`admin.go:609-611`) — any new content type or renamed directory needs a matching map entry or it is silently never ingested, as already happened with `photos/`.
- **`visibility` and `chatbot_include` already exist as DB columns**, independent of frontmatter, and are load-bearing for retrieval gating today (`chat_retrieval.go`, `answer.go:264`). Any schema redesign that tries to move these back to being frontmatter-authored has to either keep the DB columns as the enforced copy (frontmatter as intent, DB as the derived fact — the ADR 0031-preferred shape) or replace the DB read path, which is a larger, separate change.

## Decisions the owner must make

1. **Whether to build the content model in §7.2's shape at all, build a smaller one, or formally descope it** — §7.2 itself is explicitly unimplemented specification, and nothing commits the project to that exact field set versus a leaner one matched to what the three existing readers actually use.
2. **Sequencing against FR-PROF-03.** Add `tracks` now (an inert field with no consumer, repeating the `hero_tracks` pattern) or hold it until FR-PROF-03's design — the home-page/listing ordering logic — is settled, so the field's shape (weights per track, their range, how ties are broken) is driven by a real consumer rather than guessed in advance.
3. **One `visibility` vocabulary or three.** Collapse the photo-display gate (`member`/`public`), the corpus-retrieval gate (`public`/`corpus_only`), and the FSD's target vocabulary (`landing`/`member`) into one shared enum, or keep them deliberately separate and document why each exists — but stop letting one word carry three unrelated value sets with no cross-reference.
4. **Whether validation is hand-authored now or deferred to the admin CMS.** §7.2's own note recommends deferring: validation "stops being optional" once content is database-backed and admin-authored rather than owner-edited files. Building a schema before that CMS exists means validating a format (hand-edited Markdown + YAML) that is itself expected to be replaced.
5. **Whether `photos/` should be added to `publicCorpusSubdirs` and ingested into the corpus** — a decision independent of the schema question, since it is a one-line map addition, not a frontmatter change, but one that changes what Ask Roger and the JD reviewer can see.
6. **Whether to normalize the existing article frontmatter (add `tags`/`series`/`part`/`author` to the teaser post, or decide those fields are genuinely optional per-article)** before or as part of any schema authoring — a schema written against the current inconsistent set either has to make four of CNT-17's listed fields optional-by-type or the teaser post needs updating first.

## Task checklist

1. Resolve decision #2 and #3 above (FR-PROF-03 sequencing; the `visibility` vocabulary) before authoring anything, since both change the field list a schema would need to cover.
2. Normalize existing article frontmatter so every file of type `article` carries the same key set (add `series`/`part`/`tags`/`author` to `manufacturing-hub-teaser-post.md`, or explicitly mark them type-optional in the schema design) — `apps/web/content/articles/manufacturing-hub-teaser-post.md:1-5`.
3. Author a minimal schema (format per decision #1) against the frontmatter that actually exists today across all three content kinds — not against §7.2's full specification — per the 2026-10-02 deletion commit's own guidance to re-author from current content rather than restore the deleted file.
4. Pick one shared parsing/typing layer (or a small shared library) for frontmatter, replacing the three independent hand-rolled parsers in `apps/web/src/lib/articles.ts:85-102`, `apps/web/src/lib/photos.ts:30-49`, and `services/api/internal/ingest/walker.go:187-227` — or, if they must stay separate (TypeScript build-time vs. Go runtime ingest), add a test that fails when their recognized key sets diverge, following the ADR 0031 pattern ("where the two must stay separate, a test compares them").
5. Wire the schema into a build-time check (a `pnpm`/`npm` script or CI step) that fails the build on a mismatch, closing the validation half of FR-CNT-16.
6. Add `id` and `type` to every content file's frontmatter and to both TypeScript readers' types, closing the two fields CNT-17 names that exist nowhere today.
7. Decide and implement the `photos/` corpus-ingestion gap: add an entry to `publicCorpusSubdirs` (`services/api/internal/handlers/admin.go:612-625`) if photo content should be retrievable, or document the omission as intentional if not.
8. Update `docs/FSD.md` rows for FR-CNT-16 (`:350`) and FR-CNT-17 (`:351`) to reflect whatever subset actually ships, following the same "say what changed and why" pattern the 2026-10-02 commit used rather than silently moving the status cell.
9. Correct or resolve the "backlog item 11" references in `docs/FSD.md:738` and `apps/web/src/lib/articles.ts:11-12`, which both point at a numbered backlog item that does not match the current `docs/backlog.md` structure (see below).
10. Revisit FR-CNT-12 (hero-by-track, `docs/FSD.md:346`) once `tracks`/FR-PROF-03 exist, since `hero_tracks` is already parsed and typed on every photo and only needs a consumer.

## What could not be verified

- Whether `docs/backlog.md`'s "Content management UI" bullet (`docs/backlog.md:659-663`, under "4. Admin console") is the same item that `docs/FSD.md:738` and `apps/web/src/lib/articles.ts:11-12` both call "backlog item 11" or "backlog item #11" — the bullet is unnumbered in the file as it stands today, and no numbered list in `docs/backlog.md` currently assigns it the number 11. This may be a stale reference from a prior backlog revision rather than a live discrepancy; it was not resolved here.
- Whether any draft or in-progress schema work exists outside this repository (a design doc, a Pydantic model sketch, a conversation) that would supersede re-authoring from scratch — only the repository's committed history was checked.
- Whether `docs/personal/` (gitignored, referenced by `apps/web/src/lib/articles.ts:7-9` as the source of draft/unpublished articles that get hand-copied into `apps/web/content/articles/`) contains additional frontmatter fields or conventions not visible in the public repository — that directory was not read.
- Whether the private corpus mount (`CORPUS_PRIVATE_ROOT`, §7.4) uses frontmatter consistent with, or different from, what is documented here for the public mount — only the public corpus and its ingestion path were traced in detail.
- Whether any currently-running production reindex job has already picked up a prior, now-superseded version of `publicCorpusSubdirs` with different entries — only the committed source was read, not the deployed container's actual behavior.
