# Evaluating the JD reviewer

How to run an evaluation, how to read one, and what the numbers do and
do not mean.

The reviewer is a model pipeline. Whether a change to a prompt, a model
or the corpus made it better is not answerable by reading the diff. It
is answerable by scoring a fixed set of postings the owner has already
judged, under one configuration, before and after. Everything here
exists to make that pair of runs the evidence.

Related: [`docs/llm-tuning-log.md`](llm-tuning-log.md) is the history,
one entry per run with what changed and what it cost. The FSD specifies
the harness in §5.13 (FR-EVAL). This document is the operating manual.

---

## 1. The golden set

A fixed group of postings, each carrying one label the owner supplied:
**above** the hiring gate or **below** it.

The label is about capability only. Never whether the level, the pay,
the location or the hours suit him; those are parameter filters and not
this system's business. An unlabelled posting is excluded, because an
evaluation compares against an expectation and a posting nobody has
judged has none.

Nine postings today, three chosen and six drawn at random from a
job-board search and taken in the order results came back. The random
ones matter more than the chosen ones: a set assembled to be passed
tells you nothing.

Managed on `/admin/evals`. Tables: `golden_postings` (migrations 00027,
00030).

## 2. Running one

`/admin/evals`, the "run an evaluation" panel, with a note saying what
is being tested. The note is not decoration: a run whose purpose is not
recorded is very hard to interpret a week later.

It scores every active posting through the **production pipeline**, one
at a time. Not a copy of the pipeline, so a run cannot pass against
something that is not serving members.

    about 30 to 50 minutes per posting on the CPX41
    about 5 hours for nine
    one posting at a time; PipelineConcurrency = 1

Per posting the pipeline makes roughly 16 model calls: one
`posting_check`, one `jd_requirements`, one `requirement_judge` per
requirement (usually 14), then `resume_tailor` if it lands above the
gate. The eval row is written only after all of that, so the gap
between the last judgment and the row appearing is résumé generation,
not a hang.

Résumé generation is the long pole and only happens above the gate.
A posting that scores 1.0 costs about 25 minutes of judging plus about
10 minutes of résumé, which is why a strong posting takes 40 minutes
and a weak one takes 25. A posting running longer than its neighbours
is usually scoring well, not stuck.

**The run order is below-gate first, then above-gate alphabetically.**
`ListGoldenPostings` orders by `expected_gate DESC, name`, so the
position in "scoring X (5 of 9)" follows that and not the order
postings were added or their ids. Today that is nexus, profluent, xai,
then ati, blue-origin, heaven-hill, cai, dover, orca. Worth knowing
before counting positions off any other list.

**A progress report is written when a step begins, not when it ends.**
So on `/admin/ops` the duration belongs to the step named on the line,
measured to the next report; the number sitting behind a line is the
previous step's. That caused a wrong reading on 2026-09-29, where a 42
minute posting appeared to be the one that had just started. Since migration 00042 the per-phase split is recorded on the run
itself, so this no longer has to be reconstructed:

    SELECT r.run_id, p.key AS phase, (p.value::bigint/1000) AS seconds
      FROM jd_runs r, jsonb_each_text(r.phase_ms) p
     WHERE r.submission_id = $1 ORDER BY p.value::bigint DESC;

`phase_ms` is empty for runs from before that migration, and for a run
that died in the queue. It covers the time between progress reports, so
it does not sum to `duration_ms`. Check `llm_usage` for per-call
detail:

    SELECT created_at::time, prompt_id, ok, latency_ms, completion_tokens
      FROM llm_usage WHERE created_at > now() - interval '50 minutes'
     ORDER BY created_at;

### Never deploy during a run

The job runner keeps jobs in memory. Recreating the api container kills
whatever is running, and all three services share one `IMAGE_TAG`, so
there is no partial deploy that avoids it. A CSS-only change still
takes the api down with it.

`/admin/ops` answers "is anything running" before a rollout. This is
not hypothetical: it happened twice on 2026-09-24, the second time
losing a run seven postings in.

There is no cancel RPC. Stopping a run means restarting the api, and
the `eval_runs` row then has to be closed by hand, because a container
killed outright never reaches the code that closes it properly.

### Verifying a reindex before you start

Wait for the **job** to report done, not for the embedding backlog to
reach zero. A reindex processes one document at a time, so there are
brief moments between documents when no chunk is unembedded and the
work is not finished. Polling for `embedding IS NULL = 0` therefore
returns early and gives a partial count.

That happened on 2026-09-29: the corpus was reported at 300 chunks and
was actually still working its way to 303. The run started after it had
genuinely finished, so nothing was lost, and the discrepancy only
surfaced later because `eval_run_documents` had captured the real
number. Read the job through `/admin/ops` or `GetJob`, or compare the
count against the manifest of the run that follows.

## 3. Reading the result

Three numbers, and they are read together.

**Gate accuracy.** How many postings landed on their expected side.
`9 of 9` is the target and has been met since run 8.

**Ordering violations.** A below-gate posting outscoring an above-gate
one. Worse than a gate miss, because moving the gate cannot fix an
inversion. Zero since run 8.

**Margin.** The gap between the lowest above-gate score and the highest
below-gate score, computed by `separation()` in
`services/api/internal/jd/evaluate.go`. It is not an average and not a
confidence score: it is the width of the empty band around the
threshold.

    run 12   Blue Origin  0.8214   lowest above the gate
             ----------- gate 0.70
             nexus        0.6429   highest below
             margin       0.1786

Margin matters because the other two are step functions and hide
trouble until it is already failure. A change can push a posting from
0.93 to 0.82 and the run still reports a clean 9 of 9. Margin moves
first.

**A margin is only comparable when every posting scored.** `separation()`
skips items with an error, so a run that lost a posting computes its
margin from a different set. Run 7's 0.214 looked better than run 10's
0.179 and was not comparable: two above-gate postings had errored and
were simply absent from the calculation.

## 4. The noise floor, which is the thing most worth knowing

**About one requirement per posting moves per run, at temperature 0,
with nothing changed.**

Established by run 12 against run 11 with the corpus identical. CAI
scored 0.9286 in both runs with two different requirements swapped
underneath it. Orca's entire movement was one requirement on HR
partnering. Profluent has gone 0.2500, 0.3571, 0.2500, 0.3571 across
four runs on the same single requirement, with its evidence untouched.

Three postings held at exactly 1.0000 across all of those runs, so this
is not general instability. It is marginal requirements, the ones whose
evidence sits near the judge's decision boundary, flipping repeatedly.

Two consequences:

- **A single-posting delta below about 0.04 is not evidence.** One
  requirement on a 28-point scale is 0.0357.
- **Diff verdicts by requirement text, never by id.** Requirements are
  re-extracted each run and the ids are not stable. A score that has
  not moved can still have two requirements swapped inside it, and
  reading ids across runs will attribute a change to the wrong
  requirement. That mistake was made on run 11 and corrected on run 12.

`scripts/eval_compare.sh <new> [old]` does all of this in one pass:
scores with deltas, both run headers, the verdict diff matched by text,
verdict totals, rationale issues, and which corpus documents differ
between the two runs. Read-only and safe to run against a live
evaluation.

## 5. What a run records

Enough to compare it with another one.

    model, num_ctx, threshold, app_commit
    prompt fingerprints        v{version}:{sha256(system+schema)[:4]}
    corpus fingerprint         whether two runs read the same corpus
    eval_run_documents         which documents, with content_hash
                               (migrations 00037, 00040)

The corpus fingerprint answers "same corpus or not". The manifest
answers "which documents", and the content hash answers "and were they
the same documents", which matters because a re-index that edits text
without changing the chunk count is otherwise invisible. That is not
hypothetical either: the ontology article had 362 asterisk artifacts
removed and came back with exactly the 26 chunks it had before.

## 6. Failure modes seen so far

**A judgment will not stop writing.** The judge averages 123 completion
tokens. On run 13 one judgment filled its 1200-token cap, returned
incomplete JSON, and aborted the whole posting four hours in. The
requirement was the one the run existed to test: evidence had finally
arrived for it and the model had a great deal to say.

A retry at double budget was added, and run 14 measured what that
bought. The same requirement filled 1200, retried at 2400, and filled
that too, both times without closing the object, costing thirteen
minutes an attempt and losing the same posting a second time.

**So a budget was the wrong instrument, and the limit moved into the
grammar.** Ollama compiles the prompt's JSON schema into a decoding
grammar, and a bound it can express cannot be exceeded whatever the
model intends. `requirement_judge` v12 bounds every open field:
`rationale` at 400 characters, the arrays at 12 and 8 entries,
`judgments` at 1. Field order is load-bearing, because the grammar
emits properties in schema order, so `verdict` and `evidence_ids` are
settled before `rationale` can misbehave.

Verified on the box against `qwen3:4b-q8_0`: asked for an exhaustive
multi-paragraph rationale with `num_predict` at 2000, it stopped at 233
tokens with `done_reason` "stop" and valid JSON, the rationale cut at
exactly 400 characters. `TestJudgeSchemaBoundsEveryOpenField` keeps the
bounds present; `TestJudgeSchemaBoundsHoldLive` re-checks against a real
model when `JUDGE_SCHEMA_LIVE` is set.

Rule 10 already asked for 200 characters and had done since version 1.
The model obeys it almost always, and ignored it precisely on the
requirement where it had most to say, which is why the instruction
could not be the enforcement.

Malformed JSON well under budget still fails immediately, because that
is a model that cannot follow the schema and retrying it more
expensively helps nobody. A recovered truncation records nothing; a
terminal one writes the prompt and the whole response to `decision_log`
with `error` set, readable on `/admin/decisions` under "failed calls
only".

**Both times, the response itself was unrecoverable.** The capture that
would have kept it (migration 00039) was written after run 13 and was
still sitting undeployed behind run 14 when run 14 hit the same wall.
A fix for a failure mode is worth nothing until it is on the box, and
"deploy after the run" means the next occurrence is also unreadable.

**The health probe passes while the judge is unavailable.** On
2026-09-25 a run scored eight postings on retrieval similarity alone
with no verdicts behind any of them. The probe now asks whether the
model can be served, not whether configuration exists.

**Adding evidence is not a safe operation.** Run 11 added four
documents and a met verdict disappeared on an unrelated posting: 27 new
chunks displaced the four that had been supporting it. A capability the
corpus only implies is one the corpus will eventually lose.

## 7. After a run

Write the entry in [`docs/llm-tuning-log.md`](llm-tuning-log.md) in the
same session. It carries what changed, what moved, what did not, and
what was concluded wrongly. Several entries exist mainly to retire an
earlier reading, which is the point of keeping it.

Then deploy whatever was held during the run.
