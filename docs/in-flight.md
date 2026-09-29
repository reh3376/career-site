# In flight

What is running, what is half-built, and what is waiting on whom.

Not a backlog. `docs/backlog.md` holds work that is understood and not
started; this holds work that is underway, where stopping halfway loses
something. Delete an entry when it lands.

Last updated 2026-09-29, 23:10 UTC.

---

## 1. Evaluation run 14, stopped. Run 15 is the next one

Stopped by hand at 5 of 9 on the owner's instruction, so that the
diagnostics land before another five hours are spent. Its row is
closed.

Four postings scored correctly. Blue Origin errored in the same place
as run 13, judge batch 11 of 14, having filled 1200 tokens and then
2400 on the retry. See the 2026-09-29 entry in
[`llm-tuning-log.md`](llm-tuning-log.md).

**Run 15 starts only after the whole deploy queue is out**, because
both times this failed, the capture that would have explained it was
sitting undeployed behind the run that needed it.

Run 15 changes one thing, `requirement_judge` v12, against an
unchanged corpus. That makes it comparable with run 12 (9 of 9, 0
inversions, margin 0.1071) and a direct test of Blue Origin's r11.

**The bar the owner set:** similar or better than run 12.

**Nothing may deploy during a run.** All three services share one
`IMAGE_TAG`, so any rollout recreates the api and the job runner keeps
jobs in memory.

## 2. Deploy queue, now unblocked and next to go out

Production is on `ccad59262fe5`. Main is many merges ahead and carries
**migrations 00040, 00041 and 00042**, so this is not a no-op rollout.

Run 14 is stopped, so nothing is holding this any more. It goes out
before run 15 starts, and that ordering is the point: the capture that
would have explained this failure was written after run 13 and was
still queued here when run 14 hit the same wall.

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

## 4. Meeting scheduler, actively being built

FR-CNT-23/26/27/28, FR-ADM-13, decision D-22.

**Settled by the owner 2026-09-29:**

- **Members only**, not the public page the FSD originally specified.
  His reason: the target is his **main personal calendar**, so an
  unauthenticated visitor could block real time on it.
- **Free/busy only**, never `events.list`, for the same reason: this
  application must never hold the contents of his private calendar.
- **15, 30 or 45 minutes**, the member's choice.
- **15 minutes of clearance** between meetings, not held in advance. A
  15-minute meeting consumes 30 of the day, a 30 consumes 45, a 45
  consumes 60.
- **The calendar is re-queried immediately before booking**, because
  the list the member saw is stale by the time they submit.
- **America/New_York, stated next to every time**, not converted to the
  visitor's zone. Stored as an IANA name, never an offset.
- **A downloadable `.ics`** beside the calendar event.
- Windows: Tue, Wed, Thu, 09:00 to 12:00 and 14:00 to 16:00.

**Built and merged:** availability computation, clearance arithmetic,
confirmation re-check, the `calendar.Provider` seam with a stub and
`NotConnected`, `meeting_bookings` with a gist exclusion constraint,
and the repository translating `23P01` into a taken slot. 32 tests,
including both DST offsets either side of 2026-11-01 and the
constraint verified against `pgvector:pg16`.

**Not built:** the Google provider, the admin windows surface, the
member booking UI, the `.ics` download, and OAuth token storage
(needs AES-GCM; only HMAC exists today).

**Waiting on the owner:** a Google Cloud OAuth client (Web
application), redirect `https://rogerhenley.dev/admin/scheduler/callback`.
The consent screen can stay in Testing with his address as the only
test user, which avoids Google's verification review.

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

## 8. Open questions for the owner

- Which Whiskey House systems incorporate an LLM at runtime, as opposed
  to having been built with AI assistance. Only the on-prem SME chat
  agent is claimed today, deliberately.
- Whether the golden set should grow toward ten, which needs real
  postings.
- Scope for the nightly metrics export to object storage.
