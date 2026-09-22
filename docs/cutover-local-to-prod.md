# Cutover: local LLM testing to production

This is the runbook and the engineering checklist for the Ask Roger model
path: how it went from "proven on the owner's Mac" to "serving on the
Hetzner box" (done 2026-09-22), and how any later prompt, model or setting
change is proven locally and rolled out. It covers what is different
between the two environments today, the exact order of operations, and
the code changes behind each step. Update it whenever a step changes; it
is the source of truth for the LLM rollout.

## 1. Where things stand

| Layer | Local (owner's Mac) | Production (CPX31, 4 vCPU / 7.7 GB, since 2026-09-21) |
|---|---|---|
| Embeddings | Ollama on the host, `nomic-embed-text`, sidecar at `host.docker.internal:11434` | `ollama` container, `SIDECAR_EMBED_PROVIDER=ollama`, corpus fully on `ollama:nomic-embed-text#p1` (Phase A done 2026-09-21) |
| LLM gateway | Ollama on the host, `qwen3:4b-q8_0`, `SIDECAR_LLM_NUM_CTX=8192` | **live since 2026-09-22**: the box's `ollama`, `qwen3:4b-q8_0`, 8k context, `OLLAMA_MAX_LOADED_MODELS=1`, flash attention, q8 KV cache, `OLLAMA_MEM_LIMIT=7g` |
| Corpus | public mount + `./.corpus-private` staged by `make stage-corpus` | public mount + `/opt/career-site-private/corpus` synced by `make sync-corpus`; includes the career facts sheet (`profile`) |
| JD scoring | requirements v2 → per-requirement retrieval + facts sheet → one judge call per requirement → score formula v2 in code | same code; the gate is the "strong" fit band edited on `/admin/jd` (0.70 on prod; `JD_MATCH_THRESHOLD` only seeds it on first read); one pipeline at a time; 15 to 30 minutes per JD on the CPU |
| Résumé | JSON with source ids, code-side verification, markdown render, Typst PDF locked with `RESUME_PDF_OWNER_PASSWORD` | same; `RESUME_PDF_OWNER_PASSWORD` is set on prod; the résumé call is ~13 minutes of generation on the box |
| Decision log | `/admin/decisions` collects every verdict and gate call with the evidence shown | same; the reviewed rows are exported as JSONL (`docs/decision-log.md`) |
| Adapter | not started on either side; the reviewed decision-log export is the labelled set it will train and evaluate on | not started |

The api decides at boot whether to run the structured assessor: it calls the
sidecar's `Health` and enables the assessor only when `llm_provider` is not
`stub` (or `LLM_ALLOW_STUB=1`); the log line is `jd assessor + résumé writer
enabled`. Flipping `SIDECAR_LLM_PROVIDER` on the sidecar and restarting both
containers is therefore the whole switch on the application side, and it has
been flipped on prod. With the assessor wired, an assessment failure marks
the submission `failed` (Re-score on `/admin/jd` reruns it); the retrieval
pre-score is diagnostics only and is never used as a gate. The dated record
of every measurement and decision behind the current settings is
`docs/llm-tuning-log.md`.

## 2. Principles the rollout must keep

From the owner's design notes (see `project_llm_design_principles` memory):

1. Retrieve, don't memorize: facts live in pgvector, the model handles format
   and judgment.
2. The model never emits the score. Requirements and verdicts are
   schema-constrained JSON; the weighted score is computed and stored with
   its derivation (`jd_submissions.assessment`).
3. The résumé is grounded: every bullet carries source chunk ids and code
   drops anything unsourced.
4. PDF is rendered outside the model (sidecar, Typst).

## 3. Sizing: the CPX31 is the ceiling

**Constraint (owner, 2026-09-21): no further server spend.** The CPX31
(4 vCPU / 7.7 GB) is the box; the design has to fit it. Two levers make
that work, and both are in place:

1. **Context budgeting.** `SIDECAR_LLM_NUM_CTX` (sidecar) and `LLM_NUM_CTX`
   (api) carry one number. The sidecar requests exactly that window from
   Ollama and refuses any call the server evidently truncated; the api
   estimates tokens at 4 bytes per token, keeps each judge call inside
   the window and trims résumé evidence to it in priority order (master
   résumé, cited chunks, JD retrieval). At 8192 with the q8 KV cache,
   `qwen3:4b-q8_0` peaks at about 5.5 GB against the 7 GB
   `OLLAMA_MEM_LIMIT`. Judgments are one requirement per call
   (`judgeBatchMax = 1` in `services/api/internal/jd/assess.go`), which
   costs nothing in total tokens and makes the verdicts independent of
   the context size; the reasoning and the measurements are in
   `docs/llm-tuning-log.md`. Expect 15 to 30 minutes per JD on this CPU.
2. **The model is `qwen3:4b-q8_0` (owner's decision, 2026-09-22).** The
   8b OOM-killed the box at 8k context; the 14b never fit and is dropped
   from all measurement. With the career facts sheet in every judge call
   the 4b reads the owner's record closer to his own assessment than the
   14b did. `OLLAMA_LLM_URL` still allows a different Ollama host for the
   LLM than for the embedder if that is ever wanted at no cost
   (`OLLAMA_API_KEY` for a hosted endpoint).

| Workload | Model | Where | Memory | Notes |
|---|---|---|---|---|
| Embeddings | `nomic-embed-text` | CPX31 `ollama` | ~0.6 GB loaded | done (Phase A); unloads while the LLM runs (`OLLAMA_MAX_LOADED_MODELS=1`) |
| Assess + résumé | `qwen3:4b-q8_0` at 8k ctx | CPX31 `ollama` | ~5.5 GB peak | live; judge calls ~100 to 135 s each, résumé ~13 min |

### Historical sizing note (superseded; kept for the record)

These were the numbers behind the earlier plan to run a 14b or 8b model.
Neither is an option now: the 14b never fit, the 8b OOM-killed the box, and
a bigger box is off the table.

| Workload | Model | Disk | RAM while loaded | Fits |
|---|---|---|---|---|
| Embeddings | `nomic-embed-text` | ~275 MB | ~600 MB | CPX11 with `OLLAMA_MEM_LIMIT=900m` and swap |
| Assess + résumé | `qwen3:14b` (Q4_K_M) | ~9.3 GB | ~10 to 11 GB | would need a 16 GB box; dropped |
| Assess + résumé, smaller | `qwen3:8b` (Q4_K_M) | ~5 GB | ~6 GB | OOM-killed the CPX31 at 8k context; dropped |
| Adapter-merged model | as published to Ollama | same as base | same as base | same as base |

CPU inference on 4 shared vCPUs runs at a few tokens per second. The JD
pipeline is asynchronous and bounded by `JD_PIPELINE_TIMEOUT_SECONDS`
(3600 s in prod, counted from the moment a submission holds its pipeline
slot, not from submit; `SIDECAR_LLM_TIMEOUT_SECONDS=3000` bounds one model
call), so a slow answer is acceptable; a timed-out one is recorded as
`failed` with the error visible in `/admin/jd`. Budget roughly: one
requirements call, one judge call per requirement (6 to 14, ~2.4k prompt
tokens each with the facts sheet, of which ~1.1k are served from Ollama's
prompt cache) and one résumé call (~1.6k output tokens) per above-gate JD.

## 4. Order of operations

### Phase A: embeddings on prod (done 2026-09-21)

1. `.env.prod`: `SIDECAR_EMBED_PROVIDER=ollama`, `OLLAMA_MEM_LIMIT=2g`,
   `OLLAMA_KEEP_ALIVE=10m` (the CPX31 has the headroom; raised to `30m`
   once the LLM went live, which is the value on the box today). The limit was
   later raised to `7g` for the LLM (Phase C).
2. `cs up -d ollama sidecar api`; wait for the model pull in `cs logs -f ollama`.
3. `/admin/corpus`: **Reindex public content**, **Reindex private corpus**
   (after `make sync-corpus`), then **Embed sweep** until `remaining` is 0.
   Reindex and sweep run as api-side jobs since PR 88 (`RunJob` /
   `GetJob`; the corpus page starts one and polls it), so a 150 s
   private reindex or a full sweep no longer outruns the proxy: click
   once, watch the progress line, leave the page if you like.
4. `deploy/live-check.sh --submit` confirms a real `retrieval_score`
   (0.701 for the labelled strong JD on the first run).

### Phase B: prove a change locally (done for the current settings; repeat for every prompt, model or formula change)

1. `SIDECAR_EMBED_PROVIDER=ollama SIDECAR_LLM_PROVIDER=ollama docker compose up -d --build`.
2. `make stage-corpus`, reindex both scopes, embed sweep.
3. Submit the calibration set (`docs/personal/calibration-jds.json`, keys
   `strong`, `mid`, `weak`, `unrelated`, `bosch_lead`) and review
   `/admin/jd/[id]`: requirements extracted, verdicts cite evidence, score
   separates the set. Adjust prompts by bumping their `Version` in
   `services/api/internal/prompts`; log the run in `docs/llm-tuning-log.md`.
4. When an adapter is ready, publish it to local Ollama
   (`ollama create <name> -f Modelfile` with `FROM qwen3:4b-q8_0` and
   `ADAPTER ./adapter`), set `OLLAMA_LLM_MODEL=<name>`, rerun step 3 and
   compare `llm_usage` latency and the assessment quality side by side.
5. Exit criteria: the calibration set orders correctly with a gap of at
   least 0.15 between "mid" and "strong" (current: strong 0.857, mid 0.591;
   Bosch 0.917; weak 0.100; unrelated 0.000), no unsourced résumé bullets,
   and p95 pipeline time under the prod timeout at prod-equivalent CPU
   speed (measure with `OLLAMA_NUM_THREAD=4` locally to approximate the
   box).

### Phase C: flip the LLM on prod (done 2026-09-22)

The box is not resized for this; the CPX31 is the ceiling (section 3).

1. `.env.prod`: `OLLAMA_MEM_LIMIT=7g`, `OLLAMA_MAX_LOADED_MODELS=1`,
   `OLLAMA_FLASH_ATTENTION=1`, `OLLAMA_KV_CACHE_TYPE=q8_0`,
   `SIDECAR_LLM_NUM_CTX=8192` and `LLM_NUM_CTX=8192`,
   `JD_PIPELINE_TIMEOUT_SECONDS=3600`, `SIDECAR_LLM_TIMEOUT_SECONDS=3000`,
   `RESUME_PDF_OWNER_PASSWORD` set. Set `OLLAMA_LLM_MODEL` to the proven
   model name (`qwen3:4b-q8_0`).
2. Get the model onto the box. For a stock tag, `cs exec ollama ollama pull
   <tag>`. For an adapter-merged model, two options:
   - registry: `ollama push reh3376/<name>` from the Mac, then on the box
     `docker compose exec ollama ollama pull reh3376/<name>`;
   - file copy: `rsync` the `~/.ollama/models` blobs for that model into the
     `ollama_models` volume.
3. `.env.prod`: `SIDECAR_LLM_PROVIDER=ollama`. `cs up -d sidecar api` (the
   api re-reads the sidecar `Health` at boot and enables the assessor; the
   log line is `jd assessor + résumé writer enabled`, with `pdf=true` when
   the owner password is set).
4. Re-submit the calibration set on prod and compare scores with the local
   run. Existing rows are not rescored automatically; use **Re-score** on
   `/admin/jd/<id>`.
5. Watch `llm_usage` latency; if p95 approaches the pipeline timeout, the
   levers are the résumé's target length, the number of evidence chunks per
   requirement and the context window (see the tuning log), not the box.

### Phase D: résumé PDFs (shipped with PR 3b/4)

The sidecar renders the verified JSON with Typst (the `typst` wheel bundles
the compiler; no binary download) and `pypdf` applies AES-256 with an empty
user password and the owner password from `RESUME_PDF_OWNER_PASSWORD`, so the
file opens without a prompt but cannot be edited. The api stores the PDF on
the submission row and serves it at `/api/jd/resume/<id>.pdf?t=<result_token>`;
`generated_resume_url` carries that path. **Prod step (done):**
`RESUME_PDF_OWNER_PASSWORD` is set in `.env.prod`; with it empty the api
skips the PDF and logs why, and the markdown remains the deliverable. The
value is never committed.

## 5. Programmatic changes, by step

| Step | Change | Status |
|---|---|---|
| A | `ollama` service, `embedder_model` tracking, embed sweep | shipped (PR 2) |
| A | purpose-aware prefixes for nomic | shipped (PR 64) |
| A | reindex and embed sweep as api-side jobs with progress (`RunJob` / `GetJob`) | shipped (PR 88) |
| B/C | sidecar `Generate` RPC with JSON-schema constrained output, `SIDECAR_LLM_PROVIDER` | shipped (PR 3a) |
| B/C | api `llm` gateway, versioned prompt registry, `llm_usage` ledger, monthly call cap | shipped (PR 3a) |
| B/C | requirements → retrieval → judgment assessor, `assessment` jsonb, `retrieval_score` | shipped (PR 3a); prompts and formula now v2 |
| B/C | api enables the assessor from sidecar `Health.llm_provider` | shipped (PR 3a) |
| B/C | result token on submissions; public poll releases the résumé only with it | shipped (PR 3a); owning member's session also releases it (PR 87) |
| B | grounded résumé JSON with source ids, verification, markdown render | shipped (PR 3b, with PR 4) |
| B | career facts sheet (`profile` chunks) first in every judge prompt; never truncated since PR 84 | shipped |
| B/C | decision log: every verdict and gate call with the evidence shown, human review, JSONL export (migration 00019) | shipped |
| C | admin Re-score action; one automatic retry on transient LLM errors; assessor failure marks the row `failed` | shipped |
| C | one pipeline at a time (`PipelineConcurrency = 1`); deadline counted from slot acquisition (PR 84) | shipped |
| C | `OLLAMA_MEM_LIMIT=7g`, memory levers, model publish path | done on prod 2026-09-22 |
| C | `RESUME_PDF_OWNER_PASSWORD` set in `.env.prod` | done on prod |
| C | fit bands editable on `/admin/jd` (`app_settings`, migration 00022); "strong" is the gate | shipped (PR 89) |
| C | submitter progress (`progress_pct` / `progress_stage`, migration 00021), reopenable reviews, `jd_result` email by category with the PDF attached | shipped (PRs 87, 89) |
| D | Typst + pypdf render in sidecar, PDF on the row, token-gated download route | shipped (PR 4, with PR 3b) |
| later | Ask Roger chat on the same gateway (streaming `Generate`, `chat_usage` kind) | not built (Phase 4 PR 5) |
| later | adapter Modelfile + publish step in `deploy/` | not started; waits on reviewed decision-log labels |

## 6. Rollback

- LLM: set `SIDECAR_LLM_PROVIDER=stub`, `cs up -d sidecar api`. The api
  boots with the assessor disabled (log line `jd assessor disabled;
  retrieval score is the gate`), so new submissions are scored by the
  retrieval pre-score alone. The tuning log shows that score cannot
  separate JDs (0.58 to 0.71 across the calibration set), so treat this
  as "stop reviewing", not as a degraded mode: Re-score anything that ran
  in the window once the LLM is back.
- Embeddings: set `SIDECAR_EMBED_PROVIDER=stub` and run an embed sweep; the
  recorded `embedder_model` makes the switch reversible without deleting
  anything.
- Model: `OLLAMA_LLM_MODEL` back to the previous tag and `cs up -d sidecar
  api`; the previous blobs stay in the `ollama_models` volume until pruned.
- Box: there is no box-level rollback; the CPX31 is fixed.
