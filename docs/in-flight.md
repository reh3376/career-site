# In flight

What is running, what is half-built, and what is waiting on whom.

Not a backlog. `docs/backlog.md` holds work that is understood and not
started; this holds work that is underway, where stopping halfway loses
something. Delete an entry when it lands.

Last updated 2026-10-02. Production is `f13edf9fc83a`, database at
migration 48.

---

## Almost nothing, and that is the point

On 2026-09-30 this file ran to nine sections. Eight of them have landed
and have been deleted under the rule above. What follows is the short
version of what they said, because an empty file reads as neglected
rather than finished.

**Shipped and verified since:**

- **Ask Roger.** The entry that said "nothing member-facing exists yet,
  there is no `/ask` page and no side panel, the bank is empty so it
  never fires" is false in every clause. `/ask` is live, the panel is on
  every page, the Q&A bank holds 13 approved entries, and every one of
  the nine chat and meeting events was confirmed firing on production.
  Full state in [`ask-roger.md`](ask-roger.md).
- **The diagnostics branch.** `feat/eval-substage-visibility` merged as
  PR 199 and deployed; `requirement_judge` v12 and `phase_ms` are live.
- **The distillation study guide.** Was "converted and not ingested".
  Synced 2026-10-01 and ingested: 17 chunks, 17 embedded.
- **The meeting scheduler.** Booking, cancellation, calendar write and
  both notification emails exercised end to end on production.
- **The JD reviewer**, closed by the owner on 2026-09-30 and untouched
  since.

**Open questions that have since been answered:**

- *Seed the Q&A bank.* Done: 13 entries, all approved by the owner, all
  phrasings embedded. A bank answer returns in about 0.1 s against a
  39.4 s mean on the model path.
- *Deploy migrations 00045 to 00048?* Deployed. Production and local dev
  are both at 48.
- *The local dev database has drifted and is stuck at 31.* No longer
  true; local is at 48 and matches the migration history.

---

## 1. Still open, and genuinely waiting on the owner

- **Should Whiskey House / MDEMG material inform Ask Roger?** About 10
  corpus documents. `chatbot_include` defaults to true, so today the
  answer is yes by default; excluding them is one `UPDATE`.
- **Which Whiskey House systems incorporate an LLM at runtime**, as
  opposed to having been built with AI assistance. Only the on-prem SME
  chat agent is claimed today, deliberately.
- **Scope for the nightly metrics export** to object storage. Note that
  `minio` cannot currently be pulled, so this needs a storage decision
  before it needs code.

**Answered since:** the golden set reached ten on 2026-09-30, when
`cornerstone` was added and labelled above the gate, so the question of
whether it should grow toward ten is closed by having done it. The
composition is now four chosen and six random, seven above the gate and
three below. Run 15 scored 100 percent gate accuracy with no ordering
violations and a margin of 0.179, up from 0.107 on run 12. The thin side
is the negatives: three is not enough to claim the gate rejects
correctly, and that is the next thing the set needs.

## 2. MDEMG, investigated and not adopted

Investigated 2026-09-29 at the owner's request: see
[`mdemg-evaluation.md`](mdemg-evaluation.md). Nothing installed, nothing
changed, and that still holds.

The Jiminy guardrail remains worth a bounded advisory-mode experiment
and remains deferred.

One line of that evaluation has since been overtaken. It said the web
app had **zero tests across 117 files**. It now has a test file and a
`pnpm test` step in CI, covering the access policy, which is the surface
where a mistake serves a member page to anyone or 404s a real one. That
is the start of the gap rather than the end of it: `docs/backlog.md`
§4c names booking slot arithmetic, JD upload form states and the admin
scheduler's save-whole-or-refuse, and none of those are written.

## 3. The one thing left in the backlog

`minio` is no longer anonymously pullable from quay.io. Verified a
policy change rather than a blip, and pinning a tag does not help. See
[`backlog.md`](backlog.md).
