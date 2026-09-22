# Cutover: local LLM testing to production

This is the runbook and the engineering checklist for taking the Ask Roger
model path from "proven on the owner's Mac" to "serving on the Hetzner box".
It covers what is different between the two environments today, the exact
order of operations, and the code changes that have to land before each
step is possible. Update it whenever a step changes; it is the source of
truth for the LLM rollout.

## 1. Where things stand

| Layer | Local (owner's Mac) | Production (CPX31, 4 vCPU / 8 GB, since 2026-09-21) |
|---|---|---|
| Embeddings | Ollama on the host, `nomic-embed-text`, sidecar at `host.docker.internal:11434` | `ollama` container, `SIDECAR_EMBED_PROVIDER=ollama`, corpus fully on `ollama:nomic-embed-text#p1` (Phase A done 2026-09-21) |
| LLM gateway | Ollama on the host, `qwen3:4b-q8_0`, `SIDECAR_LLM_NUM_CTX=8192` | **live since 2026-09-22**: the box's `ollama`, `qwen3:4b-q8_0`, 8k context, `OLLAMA_MAX_LOADED_MODELS=1`, flash attention, q8 KV cache, `OLLAMA_MEM_LIMIT=7g` |
| Corpus | public mount + `./.corpus-private` staged by `make stage-corpus` | public mount + `/opt/career-site-private/corpus` synced by `make sync-corpus`; includes the career facts sheet (`profile`) |
| JD scoring | requirements v2 → per-requirement retrieval + facts sheet → one judge call per requirement → score formula v2 in code | same code; `JD_MATCH_THRESHOLD=0.70`; one pipeline at a time; 15 to 30 minutes per JD on the CPU |
| Résumé | JSON with source ids, code-side verification, markdown render, Typst PDF locked with `RESUME_PDF_OWNER_PASSWORD` | same; the résumé call is ~13 minutes of generation on the box |
| Adapter | not started; the decision log (`/admin/decisions`) is collecting the labelled set | none |

The api decides at boot whether to run the structured assessor: it calls the
sidecar's `Health` and enables the assessor only when `llm_provider` is not
`stub` (or `LLM_ALLOW_STUB=1`). Flipping `SIDECAR_LLM_PROVIDER` on the
sidecar and restarting both containers is therefore the whole switch on the
application side. With the assessor wired, an assessment failure marks the
submission `failed` (Re-score reruns it); the retrieval pre-score is never
used as a gate. The dated record of every measurement and decision behind
the current settings is `docs/llm-tuning-log.md`.

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
(4 vCPU / 8 GB) is the box; the design has to fit it. Two levers make
that work, and both are in place:

1. **Context budgeting.** `SIDECAR_LLM_NUM_CTX` (sidecar) and `LLM_NUM_CTX`
   (api) carry one number. The sidecar requests exactly that window from
   Ollama and refuses any call the server evidently truncated; the api
   batches the judgment prompt into calls that fit it and trims résumé
   evidence to it in priority order (master résumé, cited chunks, JD
   retrieval). At 8192 the KV cache for `qwen3:8b` is ~1.2 GB, so the
   model (5.2 GB) plus cache stays under a 7 GB `OLLAMA_MEM_LIMIT`.
   Judgments are one requirement per call (`judgeBatchMax`), which
   costs nothing in total tokens and makes the verdicts independent of
   the context size; the reasoning and the measurements are in
   `docs/llm-tuning-log.md`. Expect ~10 minutes per JD on this CPU.
2. **The model is `qwen3:4b-q8_0` (owner's decision, 2026-09-22).** The
   8b OOM-killed the box at 8k context; the 14b never fit and is dropped
   from all measurement. With the career facts sheet in every judge call
   the 4b reads the owner's record closer to his own assessment than the
   14b did. `OLLAMA_LLM_URL` still allows a different Ollama host for the
   LLM than for the embedder if that is ever wanted at no cost.

| Workload | Model | Where | Memory | Notes |
|---|---|---|---|---|
| Embeddings | `nomic-embed-text` | CPX31 `ollama` | ~0.6 GB loaded | done (Phase A); unloads while the LLM runs (`OLLAMA_MAX_LOADED_MODELS=1`) |
| Assess + résumé | `qwen3:4b-q8_0` at 8k ctx | CPX31 `ollama` | ~5.5 GB peak | live; judge calls ~100 to 135 s each, résumé ~13 min |

### Historical sizing note

| Workload | Model | Disk | RAM while loaded | Fits |
|---|---|---|---|---|
| Embeddings | `nomic-embed-text` | ~275 MB | ~600 MB | CPX11 with `OLLAMA_MEM_LIMIT=900m` and swap |
| Assess + résumé | `qwen3:14b` (Q4_K_M) | ~9.3 GB | ~10 to 11 GB | **CPX41 (16 GB)** or better; not CPX31 |
| Assess + résumé, smaller | `qwen3:8b` (Q4_K_M) | ~5 GB | ~6 GB | CPX31 (8 GB) |
| Adapter-merged model | as published to Ollama | same as base | same as base | same as base |

CPU inference of a 14B model on 4 or 8 shared vCPUs runs at a few tokens per
second. The JD pipeline is asynchronous and bounded by
`JD_PIPELINE_TIMEOUT_SECONDS` (3600 s in prod, counted from the moment a
submission holds its pipeline slot, not from submit), so a slow answer is
acceptable; a timed-out one is recorded as `failed` with the error visible in
`/admin/jd`. Budget roughly: one requirements call, one judge call per
requirement (6 to 14, ~2.4k prompt tokens each with the facts sheet, of
which ~1.1k are served from Ollama's prompt cache) and one résumé call
(~1.6k output tokens) per above-threshold JD.

## 4. Order of operations

### Phase A: embeddings on prod (done 2026-09-21)

1. `.env.prod`: `SIDECAR_EMBED_PROVIDER=ollama`, `OLLAMA_MEM_LIMIT=2g`,
   `OLLAMA_KEEP_ALIVE=10m` (the CPX31 has the headroom).
2. `cs up -d ollama sidecar api`; wait for the model pull in `cs logs -f ollama`.
3. `/admin/corpus`: **Reindex public content**, **Reindex private corpus**
   (after `make sync-corpus`), then **Embed sweep** until `remaining` is 0.
   Reindex and sweep run as api-side jobs since PR 88 (`RunJob` /
   `GetJob`; the corpus page starts one and polls it), so a 150 s
   private reindex or a full sweep no longer outruns the proxy: click
   once, watch the progress line, leave the page if you like.
4. `deploy/live-check.sh --submit` confirms a real `retrieval_score`
   (0.701 for the labelled strong JD on the first run).

### Phase B: prove the model locally (current work)

1. `SIDECAR_EMBED_PROVIDER=ollama SIDECAR_LLM_PROVIDER=ollama docker compose up -d --build`.
2. `make stage-corpus`, reindex both scopes, embed sweep.
3. Submit the calibration set (strong / mid / weak / unrelated JDs) and
   review `/admin/jd/[id]`: requirements extracted, verdicts cite evidence,
   score separates the set. Adjust prompts by bumping their `Version` in
   `services/api/internal/prompts`.
4. When an adapter is ready, publish it to local Ollama
   (`ollama create <name> -f Modelfile` with `FROM qwen3:14b` and
   `ADAPTER ./adapter`), set `OLLAMA_LLM_MODEL=<name>`, rerun step 3 and
   compare `llm_usage` latency and the assessment quality side by side.
5. Exit criteria: the calibration set orders correctly with a gap of at
   least 0.15 between "mid" and "strong", no unsourced résumé bullets, and
   p95 pipeline time under the prod timeout at prod-equivalent CPU speed
   (measure with `OLLAMA_NUM_THREAD=4` locally to approximate the box).

### Phase C: resize and flip the LLM on prod

1. Hetzner console: power off (`sudo poweroff`, never `compose stop`),
   rescale (CPX31 done 2026-09-21; CPX41 if `qwen3:14b` is the target),
   keep disk, power on. `deploy/setup-server.sh` is not rerun; the stack
   comes back on Docker's restart policy. Verify with `deploy/live-check.sh`.
2. Raise `OLLAMA_MEM_LIMIT` (e.g. `12g` on CPX41) and `OLLAMA_KEEP_ALIVE`
   (`30m`) in `.env.prod`. Set `OLLAMA_LLM_MODEL` to the proven model name.
3. Publish the model to the box. Two options:
   - registry: `ollama push reh3376/<name>` from the Mac, then on the box
     `docker compose exec ollama ollama pull reh3376/<name>`;
   - file copy: `rsync` the `~/.ollama/models` blobs for that model into the
     `ollama_models` volume.
4. `.env.prod`: `SIDECAR_LLM_PROVIDER=ollama`. `cs up -d sidecar api` (the
   api re-reads the sidecar `Health` at boot and enables the assessor; the
   log line is `jd assessor enabled`).
5. Re-submit the calibration set on prod and compare scores with the local
   run. Existing rows are not rescored automatically; use the admin
   re-score action once it lands (backlog).
6. Watch `llm_usage` latency for a day; if p95 exceeds the pipeline timeout,
   drop to the smaller model or raise the tier.

### Phase D: résumé PDFs (shipped with PR 3b/4)

The sidecar renders the verified JSON with Typst (the `typst` wheel bundles
the compiler; no binary download) and `pypdf` applies AES-256 with an empty
user password and the owner password from `RESUME_PDF_OWNER_PASSWORD`, so the
file opens without a prompt but cannot be edited. The api stores the PDF on
the submission row and serves it at `/api/jd/resume/<id>.pdf?t=<result_token>`;
`generated_resume_url` carries that path. **Prod step:** set
`RESUME_PDF_OWNER_PASSWORD` in `.env.prod` before the LLM flip; with it empty
the api skips the PDF and logs why, and the markdown remains the deliverable.

## 5. Programmatic changes required, by step

| Step | Change | Status |
|---|---|---|
| A | `ollama` service, `embedder_model` tracking, embed sweep | shipped (PR 2) |
| A | purpose-aware prefixes for nomic | shipped (PR 64) |
| B/C | sidecar `Generate` RPC with JSON-schema constrained output, `SIDECAR_LLM_PROVIDER` | PR 3a |
| B/C | api `llm` gateway, versioned prompt registry, `llm_usage` ledger, monthly call cap | PR 3a |
| B/C | requirements → retrieval → judgment assessor, `assessment` jsonb, `retrieval_score` | PR 3a |
| B/C | api enables the assessor from sidecar `Health.llm_provider` | PR 3a |
| B/C | result token on submissions; public poll releases the résumé only with it | PR 3a |
| B | grounded résumé JSON with source ids, verification, markdown render | PR 3b (with PR 4) |
| C | admin "re-score" action + one automatic retry on transient LLM errors | backlog |
| C | `OLLAMA_MEM_LIMIT` / `KEEP_ALIVE` raise, model publish path | runbook only |
| C | `RESUME_PDF_OWNER_PASSWORD` set in `.env.prod` | runbook only |
| D | Typst + pypdf render in sidecar, PDF on the row, token-gated download route | PR 4 (with PR 3b) |
| later | Ask Roger chat on the same gateway (streaming `Generate`, `chat_usage` kind) | PR 5 |
| later | adapter Modelfile + publish step in `deploy/` | when the local evaluation passes |

## 6. Rollback

- LLM: set `SIDECAR_LLM_PROVIDER=stub`, `cs up -d sidecar api`. The api falls
  back to the retrieval pre-score; nothing else changes.
- Embeddings: set `SIDECAR_EMBED_PROVIDER=stub` and run an embed sweep; the
  recorded `embedder_model` makes the switch reversible without deleting
  anything.
- Box: Hetzner rescale down is the same console operation in reverse; the
  compose stack does not care.
