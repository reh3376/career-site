# In flight

What is running, what is half-built, and what is waiting on whom.

Not a backlog. `docs/backlog.md` holds work that is understood and not
started; this holds work that is underway, where stopping halfway loses
something. Delete an entry when it lands.

Last updated 2026-09-30, 00:40 UTC.

---

## 1. Sequencing: the scheduler comes before run 15

Owner, 2026-09-29: *"I want the scheduler, deployed, fully tested, live
tested, and working in this application before we start the next
eval."*

So run 15 waits. One useful side effect: with no evaluation in flight,
the "never deploy during a run" rule is not binding, and the scheduler
can ship in as many small rollouts as it needs.

Run 14 was stopped at 5 of 9 and its row is closed. Four postings
scored correctly; Blue Origin errored in the same place as run 13. The
diagnosis and the fix are in the 2026-09-29 entry of
[`llm-tuning-log.md`](llm-tuning-log.md).

When run 15 does start, it changes one thing, `requirement_judge` v12,
against an unchanged corpus, making it comparable with run 12 (9 of 9,
0 inversions, margin 0.1071) and a direct test of Blue Origin's r11.
The study guide is ingested only after that, so it stays a single
variable.

## 2. Deploy queue, cleared

**Done 2026-09-29 23:25.** Production is on `7a3b6a22a83d`, database
migrated to version 42, 20 live checks passed and none failed.
Migrations 00040, 00041 and 00042 are applied.

Two notes from the rollout. `FailStrandedEvalRuns` did not fire,
correctly: runs 13 and 14 had already been closed by hand, and it only
touches rows still marked running, so that code is verified against a
scratch Postgres but not yet in production. Their counters were
repaired by hand and now read 4 scored, 1 error. And `phase_ms` is
empty on all 113 existing runs, as expected; it fills on the next
submission.

Queued, roughly in order merged: content hash in the eval manifest,
failed-call visibility across three surfaces, the evaluation guide and
`eval_compare.sh`, per-service image tags, gate evidence links, the
timeline attribution and live-elapsed fixes, the ops list auto-refresh,
and the scheduler's availability, calendar seam and bookings table.

## 3. The distillation study guide, converted and not ingested

`docs/personal/corpus/us_spirits_distillation_column_study_guide.md`,
6,939 words, already in `docs/personal/corpus-manifest.txt` as
`article`. **Not synced.**

The owner wrote it, and confirmed so on 2026-09-29, which changed its
framing from borrowed reference material to a work sample. Two edits
were made: a provenance header stating he wrote it and what it
evidences, and nine checklist stems reworded from "I can X" to "Able to
X", because a retrieved chunk reading `- [ ] I can identify a DP/flooding
knee from historian data` is indistinguishable from him claiming it.

**It gets its own run**, at the owner's request, so run 15 changes
exactly one thing and is a genuine single-variable test.

## 4. Meeting scheduler, complete in code, waiting on credentials

FR-CNT-23/26/27/28, FR-ADM-13, decision D-22. Branch
`feat/scheduler-google`, PR to follow. PR 200 (everything but the
provider) is merged.

**Built and tested:** availability and clearance arithmetic, the
confirmation re-check, `meeting_bookings` with a gist exclusion
constraint, the repository, the settings store, `MeetingService` and
its handler, `/meetings`, the `.ics` endpoint, `/admin/scheduler`, the
Google provider, AES-256-GCM credential storage, and the OAuth connect
flow. Both landing surfaces, the member home in both modes, the
hamburger menu and the admin nav all point at it.

**What is left is not code.** Three environment variables and one
button:

    GOOGLE_CLIENT_ID       from a Google Cloud OAuth client (Web application)
    GOOGLE_CLIENT_SECRET   the same client
    SECRETS_KEY            openssl rand -base64 32

Redirect URI, character for character:
`https://rogerhenley.dev/admin/scheduler/callback`. Consent screen may
stay in Testing with the owner as the only test user, which avoids
Google's verification review. Full runbook in `deploy/README.md`.

Then `/admin/scheduler` → Connect Google Calendar.

**Known limitation worth remembering:** an unverified app in Testing
has its refresh tokens expire after seven days. If booking stops about
a week after connecting, that is why, and Reconnect fixes it. Publishing
the consent screen removes the limit and triggers Google's review.

**Still open from FR-ADM-13:** the recent-bookings list with cancel
links in the admin surface.

**Deploy plan:** ship provider and all, then live-test a real booking
against the owner's calendar, then start run 15.

## 5. Diagnostics for the next failure, built, awaiting deploy

All on `feat/eval-substage-visibility`, all building and tested.

- **`requirement_judge` v12** bounds every open field in the schema, so
  a judgment cannot run away. Verified twice against the model on the
  box.
- **`phase_ms` on `jd_runs`** (migration 00042): milliseconds per
  pipeline phase, read from the progress reports the pipeline already
  makes, so nothing new is threaded through it and a failed run keeps
  its breakdown.
- **Live sub-stage reporting**, so a job shows "scoring blue-origin
  (5 of 9), judging requirement 7 of 14" instead of one line per
  posting. `jobs.Fn` now takes a `Reporter{Report, Status}`:
  a milestone appends a timeline entry, a status only replaces the
  summary, which keeps the nine lines that carry the pace readable.
- **Stranded evaluations closed at boot**, counters recovered from
  `eval_items`.

## 6. Finer-grained pipeline data, partly done

Asked for on 2026-09-29, for two purposes the owner named: better
decisions, and training material for the Ask Roger fine-tuning.

Most of it already exists and is simply not joined up. `jd_runs` holds
per-attempt configuration and totals, `llm_usage` holds per-call
prompt, tokens, latency and outcome with a `run_id`, and `decision_log`
holds the full prompt and response per judgment plus, since 00039, the
failures.

**The gap was where time goes inside a run**, and `phase_ms` (00042)
closes it: `duration_ms` was the whole pipeline, and retrieval makes no
model call so it left no `llm_usage` row to infer from.

Still open: nothing aggregates across runs yet, so "is judging getting
slower" is still a hand-written query.

## 7. Ask Roger consumes all of this, at two access levels

Stated by the owner on 2026-09-29: the observability work here is not
only for the admin console. It is the surface Ask Roger will answer
from, which is why Ask Roger has **two access levels, member and
admin**.

**Applies to every surface built from here on:** operational data gets
a real RPC with the access level enforced server side, not a
console-only query in a page handler. Anything reachable only by
hand-written SQL is something the assistant will never be able to
answer, and retrofitting is the debt this avoids.

Also settled: the CPX41's spare memory is **reserved for Ask Roger**,
not spare capacity for the reviewer. `OLLAMA_MEM_LIMIT` stays at `7g`.

See `project_ask_roger_phase4` in memory, which this supersedes in two
places.

## 8. MDEMG, investigated and not adopted

Investigated 2026-09-29 at the owner's request: see
[`mdemg-evaluation.md`](mdemg-evaluation.md).

Outcome: two questions, two different answers. MDEMG does not do code
complexity and does not claim to; the cheap answer there is
golangci-lint / gocyclo / knip, none of which this repo runs, and
ahead of those, the web app has **zero tests across 117 files**. The
Jiminy guardrail is real, cheap to attach (hooks live in `.claude/`,
blocking off by default, fails open) and worth a bounded advisory-mode
experiment, but deferred until the eval and scheduler work lands.

Nothing installed, nothing changed.

## 9. Open questions for the owner

- Which Whiskey House systems incorporate an LLM at runtime, as opposed
  to having been built with AI assistance. Only the on-prem SME chat
  agent is claimed today, deliberately.
- Whether the golden set should grow toward ten, which needs real
  postings.
- Scope for the nightly metrics export to object storage.
