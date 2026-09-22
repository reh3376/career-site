# Decision log: human-in-the-loop labels for the JD reviewer

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

## What is logged

| kind | one row per | input (what it was decided from) | output (what was decided) |
|---|---|---|---|
| `jd_requirement_verdict` | requirement judged | requirement (id, text, category, weight) and the evidence as rendered to the model: the career facts sheet chunks (`source_kind = profile`, first, whole) then the retrieved chunks (chunk id, kind, access, title, similarity, capped text) | `verdict` (validated), `evidence_ids` (validated against what was offered), `rationale`, `raw_verdict` and `raw_evidence_ids` (before validation) |
| `jd_gate` | submission scored | match score, retrieval score, threshold (the live "strong" fit band), requirement count, weight total, verdict counts, whether the assessor ran | `outcome` (`above_threshold` or `below_threshold`) |

Every row also carries: `model` (for example `ollama:qwen3:4b-q8_0`),
`prompt_id`, `prompt_version`, `num_ctx`, `prompt_text` (system + user,
exactly as sent), `response_text` (raw model output), token counts,
latency, and the submission it belongs to (`ref_kind = jd_submission`,
`ref_id`, `key` = requirement id).

Code decisions (`jd_gate`) have `model = code` and empty prompt text.

Since 2026-09-22 every row also carries `run_id`, joining it to the
pipeline run in `jd_runs` that produced it: the build, the judge model
and its context size, the prompt fingerprints, and the corpus
fingerprint. That is what makes two labelled decisions comparable. Two
verdicts on the same requirement mean something different if one was
judged against a corpus the other never saw, and before `jd_runs` there
was no way to tell. Rows written before that date have a null `run_id`.

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

## Where it lives in the code

- Migration `00019_decision_log.sql`.
- `services/api/internal/users/decision_log.go`: insert, list, review,
  export.
- `services/api/internal/jd/assess.go` writes verdict rows after each
  judge call; `scorer.go` writes the gate row.
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
