# Decision log: human-in-the-loop labels

**Purpose (owner, 2026-09-21):** "collect logs on decisions, then I will
review the decision logs to create human-in-the-loop training data."

Every decision the JD reviewer makes is written to one table,
`decision_log`, with everything a person needs to make the same call
independently: the requirement, the exact evidence the model was
shown, the model's verdict and rationale, the raw prompt and response,
and the model / prompt version that produced it. The owner reviews
rows in `/admin/decisions`, records his own verdict and a note, and
exports the reviewed rows as JSONL. That export is the training and
evaluation set for the Ask Roger adapter.

**Since 2026-09-30 the table also holds the assistant's own answers**
(`kind = chat_answer`), which changed what a row has to carry. The JD
reviewer's task is classification, so the human verdict *is* the gold
output and a label was enough. An answer is prose, and a label about
prose cannot be trained on. Migration `00048` adds the three columns
that close that gap; see "Generative kinds" below and
[`ask-roger.md`](ask-roger.md) §5.

## What is logged

| kind | one row per | input (what it was decided from) | output (what was decided) |
|---|---|---|---|
| `jd_requirement_verdict` | requirement judged | requirement (id, text, category, weight) and the evidence as rendered to the model: the career facts sheet chunks (`source_kind = profile`, first, whole) then the retrieved chunks (chunk id, kind, access, title, similarity, capped text) | `verdict` (validated), `evidence_ids` (validated against what was offered), `rationale`, `raw_verdict` and `raw_evidence_ids` (before validation) |
| `jd_gate` | submission scored | match score, retrieval score, threshold (the live "strong" fit band), requirement count, weight total, verdict counts, whether the assessor ran | `outcome` (`above_threshold` or `below_threshold`) |
| `jd_call_failed` | model call that produced no usable verdict | the prompt exactly as sent (system + user) | the whole raw response, untruncated, with `error` saying why it was unusable |
| `chat_answer` | answer the assistant gave | question, persona fingerprint, history turns, corpus scope, **the whole retrieval set** (similarity, citable, shown, marker offered), **the Q&A bank lookup whether or not it matched**, per-stage timings | the answer text, surviving citations, markers offered / written / dropped, the four flags (`out_of_scope`, `no_support`, `degraded`, `qa_match`), finish reason |

The `chat_answer` shapes are pinned in Go
(`users/chat_decision.go`) rather than left to each caller, because an
export nobody can join on in six months is worth nothing. Three things
there are not in the JD rows and each earns its place:

- **The path taken** (`qa_bank`, `model`, `degraded`, `out_of_scope`,
  `no_support`, `error`). Those are different systems working or
  failing and they render almost identically to a reader. Without it,
  "the assistant got worse" is unanswerable.
- **What retrieval found but did not show.** An answer that missed an
  obvious fact sitting below the cut is a tuning problem, not a model
  problem, and only this distinguishes them.
- **The Q&A bank's near misses.** They are the only thing that can
  calibrate `QAMatchThreshold`, and they exist only because misses are
  logged as well as hits.

An answer served from the bank, a refusal and a no-support reply all
have `model = code` and no prompt text. They are still logged: "did the
bank fire when it should not have" is one of the most valuable labels
the owner can give, and it cannot be asked about a row that was never
written.

Every row also carries: `model` (for example `ollama:qwen3:4b-q8_0`),
`prompt_id`, `prompt_version`, `num_ctx`, `prompt_text` (system + user,
exactly as sent), `response_text` (raw model output), token counts,
latency, and the submission it belongs to (`ref_kind = jd_submission`,
`ref_id`, `key` = requirement id).

Code decisions (`jd_gate`) have `model = code` and empty prompt text.

`first_token_ms` (migration `00048`) sits alongside `latency_ms` and is
zero where it does not apply. The two are separate because on a
CPU-only box they move for different reasons: the first tracks how much
context was retrieved, the second how much was written. One number
cannot tell a retrieval regression from a verbose one.

### Calls that failed (migration 00039)

Every row above is written *after* the model's output decodes, which
meant the one response nobody could examine was the response that
broke.

That is not a theoretical loss. Run 13 lost a posting four hours in to
a judgment that filled its token budget and returned incomplete JSON,
and the response that did it was gone: all that survived was a row in
`llm_usage` saying 1200 tokens and 448 seconds, from which the cause
had to be inferred rather than read. Run 14 lost the same posting the
same way, because the capture built in answer to run 13 was still
waiting to deploy.

A `jd_call_failed` row keeps the prompt and the **whole** response,
untruncated, because the behaviour at and near a failure is the
behaviour least understood and most worth keeping. These rows are rare
by construction: one call in 1,343 had ever hit this path when it was
built.

Two columns support them:

| column | meaning |
|---|---|
| `error` | why the call produced no usable verdict. Empty on every successful row, so nothing needed a backfill and no existing query changed. |
| `ok` | generated, `error = ''`. Stored rather than computed by each caller so the convention cannot be forgotten. |

`idx_decision_log_failures` is a partial index on `created_at DESC
WHERE error <> ''`, which keeps "show me the failures" cheap on a table
that is almost entirely successes.

**A recovered failure records nothing.** A truncated judgment retries
once at double budget, and if the retry decodes, the run carries on and
no row is written: the owner asked for the incomplete output only while
it matters, which is when the call ends up failing. Only a terminal
failure is kept.

Read them on `/admin/decisions` under "failed calls only", or:

    SELECT created_at, prompt_id, prompt_version, completion_tokens, error
      FROM decision_log WHERE NOT ok ORDER BY created_at DESC;

The reviewer's vocabulary mirrors the model's (`met` / `partial` /
`unmet` for verdicts, `above_threshold` / `below_threshold` for the
gate) so agreement is a direct comparison, plus one value the model does
not have: **`insufficient_evidence`**, meaning the reviewer could not
judge the row from what they were shown. It applies to both kinds.

That option exists because the first real review hit a row where neither
of the two answers was true, and forcing one manufactures a wrong label.
A wrong label is worse than no label: the agreement rate counts it.
Rows graded this way are excluded from agreement and reported on their
own, because they say something different and more actionable.
Disagreement means the judge reasoned badly, which a prompt can fix.
Ungradeable means the right documents never reached the judge, which no
prompt will fix.

Since 2026-09-22 every row also carries `run_id`, joining it to the
pipeline run in `jd_runs` that produced it: the build, the judge model
and its context size, the prompt fingerprints, and the corpus
fingerprint. That is what makes two labelled decisions comparable. Two
verdicts on the same requirement mean something different if one was
judged against a corpus the other never saw, and before `jd_runs` there
was no way to tell. Rows written before that date have a null `run_id`.

Since migration 00042 the run also carries **`phase_ms`**: milliseconds
per pipeline phase, as jsonb keyed by the phases in
`services/api/internal/jd/phases.go` (`setup`, `posting_check`,
`extract`, `retrieval`, `judge`, `score`, `resume`, `render`, and
`other`). It answers "where did the time go", which `duration_ms` and
`queued_ms` between them could not: they separate waiting from working
but say nothing about what the work was.

The timings are derived from the progress reports the pipeline already
makes for the member's benefit, so no phase boundary has to be
maintained twice. A report marks the *start* of a stage, so a phase
runs from its own report to the next one, the same convention the job
timeline uses. Consequences worth knowing:

- It does **not** sum to `duration_ms`. It only covers the span between
  the first and last progress reports.
- It is empty for runs before 00042, and for a run that died waiting
  for a pipeline slot. Empty means unmeasured, not instant.
- An `other` key means a stage was reworded and no longer matches the
  mapping. It is deliberately visible rather than folded into whichever
  phase happened to be adjacent.
- A failed run keeps what it measured, which is the case it exists for:
  the phase a run died in is the phase that holds the time.

<!-- -->

    SELECT p.key AS phase, (p.value::bigint/1000) AS seconds
      FROM jd_runs r, jsonb_each_text(r.phase_ms) p
     WHERE r.run_id = $1 ORDER BY p.value::bigint DESC;

Later kinds (`resume_item`, `chat_answer`) are planned to use the same
table and the same review flow; nothing here is JD-specific except the
input shape. Neither is written yet.

## Review

`/admin/decisions` lists unreviewed rows newest first, grouped by
submission (`?all=1` includes reviewed rows; `?kind=` and `?ref=` filter
by kind and submission). For each verdict the reviewer sees the
requirement, the evidence excerpts, the model's verdict and rationale,
and sets his own verdict (`met` / `partial` / `unmet`) with an optional
note. A row is "reviewed" once `reviewed_at` is set; the reviewer's id
is kept. Reviewing is idempotent: saving again overwrites the label.
The owner's `jd_outcome` email links straight to the decision review
for that submission.

### Generative kinds (migration `00048`)

A `chat_answer` row is graded differently, because a verdict alone
cannot be trained on.

**Verdicts** (`ReviewVocabulary["chat_answer"]`): `good`, `needs_edit`,
`wrong`, `rejected_correctly`, `insufficient_evidence`.

`rejected_correctly` exists so a correct refusal or a correct "I don't
know" is not graded as a failure; without it the assistant scores worst
exactly where it behaved best. `needs_edit` is kept apart from `wrong`
because it is the outcome that yields the best preference pair: the
correction and the model's answer then differ in precisely the way an
adapter needs to learn.

**Dimensions** (`ReviewDimensions["chat_answer"]`), each marked `yes`,
`partial`, `no` or `n/a`:

    grounded    citations    voice    scope    length

Grounding at 90 % (FR-CHAT-03) and citation validity at 95 %
(FR-CHAT-04) are stated acceptance criteria, and neither can be
computed from a single overall verdict. Unknown dimension keys and
values are **refused**, not dropped: a grade nobody can interpret still
lands in a rate.

**`human_answer` is the training target.** It is the wording the owner
would have given, and it is deliberately a separate column from
`human_note`:

- `human_note` explains the grade to a human reader.
- `human_answer` is text a model is meant to imitate.

An exporter cannot tell one from the other if they share a column, and
a note like "too long, and he never worked at Amazon" would be exported
as though it were a model answer. With `response_text` it is a
preference pair (`human_answer` chosen, `response_text` rejected); on
its own it is a supervised target.

This is the highest-value thing the console can ask for. A graded
answer with no correction can be counted; a graded answer with one can
be learned from.

**Citation validity is computed, not marked.** `markers_dropped`
against `markers_written` across rows. A human will not mark several
hundred answers; a query will.

Rules the review surface keeps:

- Private (`corpus_only`) evidence is shown to the admin in full. It
  never leaves the admin surface; the export is admin-only too.
- The model's verdict is never edited in place. The human label lives
  in its own columns so agreement can be measured (`output.verdict`
  vs `human_verdict`).
- Nothing about a review changes a live score. Recalibration is a
  deliberate step (see `docs/llm-tuning-log.md`), not a side effect.

## Export

`/admin/decisions/export?reviewed=1` streams JSONL, one decision per
line (`/admin/decisions/export` without the flag exports every row):

```json
{"id":"123","kind":"jd_requirement_verdict","ref":{"kind":"jd_submission","id":"25","key":"r4"},
 "model":"ollama:qwen3:4b-q8_0","prompt":{"id":"requirement_judge","version":2,"num_ctx":8192},
 "input":{...},"output":{"verdict":"unmet","evidence_ids":[],"rationale":"..."},
 "human":{"verdict":"met","note":"Columbia Gas telecom work is exactly this","reviewed_at":"..."},
 "prompt_text":"...","response_text":"...","created_at":"..."}
```

The pair (`prompt_text`, `human`) is directly usable as a supervised
example; (`output`, `human`) is the agreement signal; `input` lets a
future prompt version be re-run offline against the same evidence.

For a `chat_answer` row the `human` block also carries `answer` and
`dimensions`, and `usage` carries `first_token_ms`:

```json
{"id":"981","kind":"chat_answer","ref":{"kind":"chat_message","id":"4412"},
 "model":"ollama:qwen3:4b-q8_0","prompt":{"id":"ask_roger_persona","version":1},
 "input":{"question":"...","path":"model","retrieved":[...],"qa":{"matched":false,"best_similarity":0.61},
          "timings":{"embed_ms":140,"retrieve_ms":22,"first_token_ms":12800,"total_ms":24100}},
 "output":{"text":"...","citations":[...],"markers_offered":3,"markers_written":2,"markers_dropped":0},
 "human":{"verdict":"needs_edit","note":"right facts, too formal",
          "answer":"I ran that programme at Joy Global for eight years.",
          "dimensions":{"grounded":"yes","citations":"yes","voice":"no","scope":"yes","length":"yes"},
          "reviewed_at":"..."},
 "usage":{"prompt_tokens":1019,"completion_tokens":118,"latency_ms":24100,"first_token_ms":12800},
 "prompt_text":"...","response_text":"...","created_at":"..."}
```

Two training sets fall out of the same export. Rows where
`human.answer` is non-empty give supervised pairs
(`prompt_text` -> `human.answer`) and preference pairs
(`human.answer` chosen over `response_text`). Rows where it is empty
and the verdict is `good` confirm the model's own answer as the target.

## Where it lives in the code

- Migration `00019_decision_log.sql`.
- `services/api/internal/users/decision_log.go`: insert, list, review,
  export, plus `ReviewVocabulary`, `ReviewDimensions` and
  `DecisionReview`.
- Migrations `00039_decision_log_failures.sql` (keep the failing
  response) and `00048_decision_log_training_targets.sql`
  (`human_answer`, `human_dimensions`, `first_token_ms`).
- `services/api/internal/users/chat_decision.go`: the `chat_answer`
  input and output shapes, the path constants, and `NewChatDecision`.
- `services/api/internal/jd/assess.go` writes verdict rows after each
  judge call, and `recordFailure` there writes `jd_call_failed`;
  `scorer.go` writes the gate row.
- Migration `00039_decision_log_failures.sql` (the `error` and `ok`
  columns), `00042_jd_run_phases.sql` (`phase_ms`).
- `services/api/internal/jd/phases.go`: the phase timer and the
  stage-to-phase mapping, with `phases_test.go` asserting every stage
  the pipeline reports maps to a real phase.
- `AdminService.ListDecisionLog`, `ReviewDecision`, `ExportDecisionLog`
  in `proto/career/v1/admin.proto`; handlers in
  `services/api/internal/handlers/admin_decisions.go`.
- `apps/web/src/app/admin/(console)/decisions/`: page, server action,
  export route.

## Agreement as a metric

Once a few dozen rows are reviewed, agreement per verdict class is the
first number to watch: it says whether the judge is too strict (many
`unmet` → `met` overrides), too lenient, or wrong on a specific kind of
evidence (soft skills, for example). That, not the match score, is
what decides whether the next step is a prompt revision, a corpus
addition, or adapter training. `/admin/decisions` shows this as an
agreement panel (percent agreed, a judge-versus-owner matrix, and the
split by prompt version and model) computed from the graded rows.

Status (2026-09-22): the log is live on prod with the 4b model, prompts
v2 and score formula v2 (see `docs/llm-tuning-log.md`). Rows produced
by earlier models and prompt versions carry those versions in
`model` / `prompt_version` and stay in the table; filter on them when
measuring agreement. Adapter training has not started.
