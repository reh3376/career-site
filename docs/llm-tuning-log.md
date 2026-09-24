# LLM tuning log: JD assessment and résumé pipeline

A dated engineering log of every experiment, setting and decision on the
Ask Roger model path. It exists so the owner can review the work at his
own pace; the pace of the sessions is faster than a person can follow
live. Newest entries at the bottom. Each entry says what was changed,
what was measured, and what was decided. The companion runbook is
`docs/cutover-local-to-prod.md`; the design constraints are in section 2
of that file.

Vocabulary used below:

- **Calibration set**: five labelled JDs kept at
  `docs/personal/calibration-jds.json` on the owner's Mac (gitignored;
  keys `strong`, `mid`, `weak`, `unrelated`, `bosch_lead`): `strong` (a
  distillery controls / data-platform lead role that matches the owner
  closely), `mid` (adjacent), `weak` (loosely related), `unrelated`,
  and `bosch_lead` (a real Bosch "Manufacturing Automation Engineering
  Lead" posting the owner expects above 0.75, added 2026-09-22).
  Expected order: bosch_lead ≈ strong > mid > weak ≈ unrelated. Older entries below call the keys `medium` and `off`; they
  are the same JDs.
- **Retrieval score**: mean cosine similarity of the JD against the top 8
  corpus chunks. Cheap, no LLM, diagnostics only (never a gate). Stored
  as `jd_submissions.retrieval_score`.
- **Match score**: the weighted requirement score computed in code from
  the model's verdicts (`met` = 1, `partial` = 0.5, `unmet` = 0, weights
  1 to 3). Stored as `jd_submissions.match_score`; the derivation is in
  `jd_submissions.assessment` (jsonb).
- **Gate**: the lower edge of the "strong" fit band, editable by the
  owner on `/admin/jd` (`app_settings` key `jd_fit_bands`; 0.70 on prod;
  `JD_MATCH_THRESHOLD` only seeds it). Above the gate the pipeline
  writes the grounded résumé and the locked PDF. Entries below record
  the gate's history: a 0.65 constant, then 0.55, then 0.70 as
  configuration.
- **num_ctx**: the context window requested from Ollama per call
  (`SIDECAR_LLM_NUM_CTX` on the sidecar, `LLM_NUM_CTX` on the api; the
  two must match). Ollama silently truncates any prompt longer than this.

Where to look at a run: `/admin/jd/<id>` shows requirements, verdicts,
cited chunk ids and the score arithmetic. The `llm_usage` table has one
row per model call (prompt id, tokens, latency, error).

---

## 2026-09-21: baseline on the owner's Mac (qwen3:14b, 16k context)

**Setup.** Everything local: Ollama on the Mac, `qwen3:14b`,
`SIDECAR_LLM_NUM_CTX=16384`, one judge call for all requirements,
4 evidence chunks per requirement, chunk text capped at 1200 runes in
the judge prompt.

**Result (submissions 10 to 16).**

| JD | Retrieval | Match | Status |
|---|---|---|---|
| strong | 0.712 | 0.792 | ready, résumé + PDF produced |
| medium | | 0.361 | below threshold |
| weak | | 0.000 | below threshold |
| off | | 0.000 | below threshold |

The set orders correctly with a 0.43 gap between strong and medium. The
strong JD extracted 12 requirements; 9 met, 2 partial (OT network
design; leading a small team), 1 unmet ("15+ years in process
manufacturing automation": the corpus states the experience but no
single chunk says "15 years", so the judge correctly refused it).

**Decision.** Prompts `jd_requirements` v1, `requirement_judge` v1 and
`resume_tailor` v3 are the baseline. No prompt tuning to chase the
number; the verdicts read as fair.

## 2026-09-21: first prod attempt (qwen3:8b on the CPX31) and why it failed

**What happened.** Flipped prod to `SIDECAR_LLM_PROVIDER=ollama` with
`qwen3:8b`. The strong JD scored 0.048. `llm_usage` showed the judge
call had `prompt_tokens = 2050` for a prompt that should have been
about 10k tokens: Ollama had silently cut the prompt to its default 2048
context, so the model saw the first requirement and a fragment of its
evidence and judged everything else `unmet`.

**Fixes (this branch).**

1. The sidecar sends `num_ctx` explicitly on every call
   (`services/sidecar/src/career_sidecar/llm.py`).
2. The sidecar estimates prompt tokens before the call and refuses the
   result when Ollama reports fewer than 60% of the estimate
   (`check_truncation`), so a truncated call is an error in
   `llm_usage`, never a silent bad score.
3. The api budgets its prompts to the same number: the judge prompt is
   split into batches that fit (`Assessor.batchRequirements`) and the
   résumé prompt drops evidence in priority order (master résumé first,
   then cited chunks, then JD retrieval hits) until it fits.

**Prod state after this.** Reverted to `SIDECAR_LLM_PROVIDER=stub`
(retrieval score only; above-threshold rows park at `generating`).

## 2026-09-21: owner constraint, no server spend beyond the CPX31

The owner will not upgrade to a CPX41 (about $150/month more). The
CPX31 (4 vCPU / 7.7 GB) is the ceiling. Options recorded:

| Option | Cost | Quality | Notes |
|---|---|---|---|
| `qwen3:8b` on the CPX31 at 8k context | $0 extra | to be measured | ~10 min per JD on CPU; `OLLAMA_MEM_LIMIT=7g` |
| `qwen3:14b` on the owner's Mac over Tailscale (`OLLAMA_LLM_URL`) | $0 | baseline quality | Mac must be on; Starlink CGNAT is irrelevant because tailnet links are outbound from both ends |
| Ollama Cloud catalog models (`OLLAMA_LLM_URL=https://ollama.com`, `OLLAMA_API_KEY`) | free tier, rate-limited | catalog models only | cannot host a custom adapter; useful as a burst/backup backend |
| Self-host at home with a public address | $0 to a small Tailscale Funnel / VPS tunnel | baseline | Starlink has no static or port-forwardable IP; Tailscale is the practical route |

Code added so all four are one-variable choices: `OLLAMA_LLM_URL`
(LLM host separate from the embedder), `OLLAMA_API_KEY` (bearer
header for a hosted endpoint), `SIDECAR_LLM_NUM_CTX` / `LLM_NUM_CTX`.

## 2026-09-21: does the 8k context budget change the score?

**Question.** With batching in place, does the same model at 8k reach
the same verdicts as at 16k?

**Run 19** (`qwen3:14b`, 8k, 3 chunks per requirement, 1000-rune cap,
3 judge batches): strong JD **0.625**, below the gate. No truncation
(prompt tokens 4355 / 4353 / 1942).

**Run 20** (restored 4 chunks and 1200-rune cap, 4 judge batches):
strong JD **0.625** again.

**Per-requirement comparison (run 18 at 16k vs run 20 at 8k).** The
retrieved chunk ids are identical in both runs (retrieval is
deterministic), so the only difference is how many requirements share
one judge call.

| Requirement | 16k, one call of 12 | 8k, calls of 3 |
|---|---|---|
| r2 own the control-system and data architecture (w3) | met | met (run 19: partial) |
| r5 historian and MQTT / Unified Namespace (w2) | met | partial |
| r9 run post-mortems and improve processes (w2) | met | unmet |
| r12 applying LLMs to decision support (w1) | met | partial |
| all others | same | same |

Rationales in the 8k run are stricter readings of the same evidence
("references MQTT and Unified Namespace concepts but lacks direct
experience with historian systems"). Nothing in the evidence changed.

**Conclusion.** The evidence count was not the cause; the grouping was.
`qwen3:14b` at temperature 0 gives different verdicts for the same
requirement and evidence depending on which other requirements are in
the prompt. That is a ±0.17 swing in the match score from prompt
layout alone, which makes a hard 0.65 gate fragile whatever box we
run on.

**Decision.** Judge one requirement per call (`judgeBatchMax = 1` in
`services/api/internal/jd/assess.go`). Every verdict then depends only
on its own requirement and evidence, so an 8k box and a 16k
workstation must produce the same score, and a verdict can be
re-judged on its own later. Total prompt tokens are about the same
(the ~300-token system prompt is repeated; evidence is not). The
number of model calls per JD rises from 2-5 to 1 + N (N ≈ 6 to 14), so
`LLM_MONTHLY_CALL_CAP` (2000) now covers roughly 130 JDs a month
rather than 600; still far above real traffic.

## 2026-09-21: one requirement per judge call (run 21 onward)

**Run 21** (`qwen3:14b`, 8k, `judgeBatchMax = 1`, strong JD): match
**0.604**, 12 judge calls of ~1.5k prompt tokens, 1 to 3 s each on the
Mac GPU (the whole assessment took about 30 s, versus 7 to 8 minutes
for the batched prompts, because prompt evaluation dominates and the
small prompts are cheap).

Verdicts on the strong JD by grouping, same evidence every time:

| Requirement (weight) | 12 per call | 3-4 per call | 1 per call |
|---|---|---|---|
| r1 15+ years process manufacturing automation (3) | unmet | unmet | partial |
| r2 own control-system and data architecture (3) | met | met / partial | partial |
| r3 PLC/HMI standards, ControlLogix, Ignition (2) | met | met | met |
| r4 OT network design (2) | partial | partial | unmet |
| r5 historian and MQTT / UNS (2) | met | partial | partial |
| r6 hub-and-spoke data platform (2) | met | met | met |
| r7 lead a small team (2) | partial | partial | partial |
| r8 delivery pipelines with release gates (2) | met | met | partial |
| r9 post-mortems, process improvement (2) | met | unmet | partial |
| r10 multi-year digital strategy with ops leadership (2) | met | met | met |
| r11 ontologies and knowledge graphs (1) | met | met | met |
| r12 LLMs for decision support (1) | met | partial | partial |
| **score** | **0.792** | **0.625** | **0.604** |

The trend is monotonic: the fewer requirements the judge sees at once,
the stricter it reads each one. The per-requirement rationales are
defensible ("25 years in manufacturing, but not specifically in process
automation"; "involvement in defining pipeline gates, but does not
explicitly confirm..."). The single-call 0.79 was the lenient end of the
same model, not a better measurement.

**Corpus-coverage note for the owner.** r4 "OT network design" is
judged unmet under the strict reading, although the Columbia Gas
telecom / SCADA work and the net-topo-toolkit project are exactly that.
The retrieved chunks do not state it plainly. Adding a short
public-or-private corpus document that names the network design work
(what, where, scale) would fix this at the evidence layer, which is the
right layer; see `docs/personal/corpus-manifest.txt`.

**Decision.** Keep `judgeBatchMax = 1`. It is the only grouping whose
result does not depend on the context budget, it is the fastest on a
GPU, and each verdict is individually re-judgeable. The gate must be
recalibrated against this regime (next entry).

## 2026-09-21: gate recalibrated to 0.55

Full calibration set, `qwen3:14b`, 8k context, one requirement per
judge call (runs 21 to 24):

| JD | Requirements | Retrieval | Match | Verdict mix |
|---|---|---|---|---|
| strong | 12 | 0.712 | **0.604** | 5 met, 6 partial, 1 unmet |
| mid | 10 | 0.692 | **0.333** | 1 met, 6 partial, 3 unmet |
| weak | 13 | 0.643 | **0.020** | 0 met, 1 partial, 12 unmet |
| unrelated | 11 | 0.579 | **0.000** | all unmet |

Ordering is correct and the gaps are 0.27 (strong to mid) and 0.31
(mid to weak). Note how flat the retrieval score is (0.58 to 0.71
across the whole set): it is a fine pre-filter but cannot be a gate on
its own, which is why prod on `stub` parks above-threshold rows rather
than generating anything.

**Decision.** `MatchThreshold` moved from 0.65 to **0.55**
(`services/api/internal/jd/scorer.go`; the JD-upload page and result
copy quote the same number). 0.55 leaves a 0.05 margin under the
strong JD and 0.22 above the mid one. The exit criterion in the cutover
runbook (gap ≥ 0.15 between mid and strong) is met with 0.27.

**Owner's call.** The gate is a product decision. If 0.604 for a role
that fits closely feels low, the two honest levers are (a) corpus
coverage (see the r4 note above; every `partial` rationale names what
the evidence did not say) and (b) the gate itself. Loosening the judge
prompt is not on the list: the strict reading is the trustworthy one.

**Cost of the new grouping.** Calls per JD: 1 + N (N = requirements,
6 to 14) plus 1 for the résumé. `LLM_MONTHLY_CALL_CAP=2000` covers
about 130 JDs a month. On the CPX31 CPU each 1.5k-token judge call is
roughly 30 to 45 s with `qwen3:8b`, so an assessment is 6 to 10 minutes
there; on the Mac GPU it is about 30 s.

## 2026-09-21: résumé at 8k, and the token estimator was 48% pessimistic

**Run 25** (strong JD, gate 0.55, 8k): `READY`, match 0.604, résumé
JSON with 0 unsourced items dropped, 2-page locked PDF (43 KB),
résumé call 3,594 prompt tokens / 1,901 output tokens, 55 s on the
Mac GPU. First complete end-to-end run at the production context size.

**What the PDF showed.** Credible and grounded, with two losses:

1. The education line was a fragment ("Electrical Engineering,
   Infrastructure"). The writer had kept only 4 of the 6 master-résumé
   chunks; the education chunk was one of the two dropped.
2. No judge-cited evidence reached the writer at all (`kept=4,
   dropped=31`), so the résumé was written from the master résumé
   alone, not tailored by the chunks that earned the verdicts.

Yet the prompt used 3,594 of a 5,062-token budget. Measured: the
prompt was ~15.9k bytes for 3,594 tokens, **4.4 bytes per token**; the
judge prompts measured 4.3. `EstimateTokens` assumed 3, so it
over-counted by ~48% and trimmed evidence that would have fit.

Two smaller observations, logged, not acted on (a prompt change would
invalidate today's calibration):

- Competencies 1 and 2 are the two example phrases from rule 7 of the
  `resume_tailor` prompt, copied verbatim. They happen to be true of
  the owner and match the posting, so the result is fine; the leak is
  worth removing when the prompt is next revised (v4).
- The summary mentions "OT network design" and "run post-mortems",
  which the judge marked unmet / partial; rule 4 asks the writer not to
  claim unmet requirements. The master résumé chunks do state network
  infrastructure work, so the sentence is sourced; the rule is about
  emphasis and the model leaned on it anyway.

**Changes.**

- `prompts.EstimateTokens` is now `len/4 + 1` (about 10% pessimistic
  against the measured 4.3 to 4.4). The sidecar's truncation guard
  (`prompt_tokens < 0.6 x len/3.6`) only fires below ~6 bytes per
  token, so the two estimates cannot disagree on a prompt that fit.
- Non-résumé chunks in the writer prompt are capped at 1,200 runes
  (was 2,000), the same as the judge, so more cited evidence fits.
- The master résumé outranks the posting body: if the six résumé
  chunks and the full JD cannot share the window, the JD is cut to its
  first 1,500 runes with a note (`jdHeadRunes` in `resume.go`). The
  writer already has every requirement with its verdict, so nothing it
  needs is lost; roles, dates and education are never dropped.

**Run 26** (same JD, calibrated estimator, 8k): `READY`, 0.604,
2-page PDF (42 KB). The writer kept 7 chunks: all six master-résumé
chunks and one cited chunk; the résumé prompt was 5,036 tokens
(budget 5,114, so the estimate is now within 2% of the real count)
with 1,656 output tokens, 6.7k of the 8,192 window used. Both
education lines came out whole. The competencies still open with the
two rule-7 example phrases (noted above).

**Remaining headroom.** At 8k only one cited chunk fits beside the
full master résumé. Two cheap ways to admit more, when wanted: lower
`resumeReserve` from 2,600 toward the observed 1,650 to 1,900 output
tokens, or raise `SIDECAR_LLM_NUM_CTX` to 10240 (KV cache for
`qwen3:8b` grows by ~0.3 GB, still inside a 7 GB `OLLAMA_MEM_LIMIT`).
On the Mac backend (16k) the same code admits the whole cited set with
no change. Not done today; the PDF is already correct and grounded.

## 2026-09-21: prod flip and the first CPX31 numbers

Deployed `06f7067cd5da` (PR 76) and set `SIDECAR_LLM_PROVIDER=ollama`,
`SIDECAR_LLM_NUM_CTX=8192`, `qwen3:8b`, `JD_PIPELINE_TIMEOUT_SECONDS=2400`
on the box. Live check (`deploy/live-check.sh --submit`, now updated for
the members-only policy: it submits as the admin member from the
server):

- Prod submission 5 (the script's terse synthetic JD, 9 requirements):
  **0.444**, below the gate. Judge calls 66 to 91 s each on the CPX31
  CPU; the whole assessment about 12 minutes. `qwen3:8b` marked
  "Rockwell ControlLogix and Ignition standards" unmet where `qwen3:14b`
  marked it met with the same corpus.
- Prod submission 6 (the calibration strong JD, like-for-like with the
  Mac's 0.604): result recorded in the next entry when it lands.

Budget note: at ~90 s per judge call a 14-requirement posting plus the
résumé needs ~25 minutes, which is why the pipeline timeout went to
2400 s. The Mac backend over Tailscale does the same work in under a
minute; that comparison is what decides the default backend.

## 2026-09-21: decision log for human-in-the-loop labels

The owner: "collect logs on decisions, then I will review the decision
logs to create human-in-the-loop training data." Design and schema in
`docs/decision-log.md`: every verdict and every gate outcome is logged
with the evidence it was made from and the raw prompt/response; the
owner labels rows in `/admin/decisions`; reviewed rows export as JSONL.
Agreement between `output.verdict` and `human_verdict` becomes the
metric that decides between prompt revision, corpus additions and
adapter training.

## 2026-09-22: prod OOM at judge call 11, and a wrong fallback

**Prod submission 6** (calibration strong JD, `qwen3:8b`, 8k): the
kernel OOM-killed `llama-server` inside the ollama container at judge
call 11 of 12 (`anon-rss: 7,073,268 kB` against the 7 GB cgroup cap).
Ollama had both models resident (`loaded runners count=2`: the
embedder plus the 8b) and the KV cache at full precision. Ollama
restarted the runner within 10 s.

The pipeline then did the wrong thing: the assessor error fell back to
the retrieval score (0.712), which passed the gate, and a résumé was
generated with no verdicts behind it. The retrieval score is flat
across the calibration set (0.58 to 0.71) and cannot gate anything.

**Fixes (PR 78).**

- Assessor failure with the assessor wired now marks the submission
  `failed` with the error ("assessment failed: ..."), keeps the
  retrieval score on the row, and leaves Re-score to run it again.
- Judge calls retry once on a transient error at the call level, so a
  runner restart costs one call, not a whole 12-call pass. "model
  runner has unexpectedly stopped" and HTTP 500 count as transient.
- Ollama memory levers on the box: `OLLAMA_MAX_LOADED_MODELS=1` (the
  embedder unloads while the LLM works; it reloads in seconds),
  `OLLAMA_FLASH_ATTENTION=1`, `OLLAMA_KV_CACHE_TYPE=q8_0` (halves the
  KV cache). Together about 1 GB off the peak.

**Owner's direction:** "we need a smaller model, maybe a Q8 4b?"
Candidate `qwen3:4b-q8_0` (~4.4 GB on disk, near-lossless 8-bit
weights, about twice the tokens per second of the 8b on this CPU).
Measured on the calibration set in the next entry; the decision log
now lets the owner grade its verdicts directly.

## 2026-09-22: qwen3:4b-q8_0 on the calibration set (owner's Mac, 8k)

| JD | 14b (1 req/call) | 4b-q8_0 (1 req/call) | 4b requirements extracted | 4b judge call |
|---|---|---|---|---|
| strong | 0.604 | **0.842** | 7 (14b: 12) | 1.3 s |
| mid | 0.333 | **0.625** | 10 | 1.4 s |
| weak | 0.020 | **0.231** | 12 | 1.3 s |
| unrelated | 0.000 | **0.000** | 7 | 1.2 s |

Ordering correct, gaps 0.22 (strong to mid) and 0.39 (mid to weak).
Two differences from the 14b:

1. The 4b reads evidence more leniently: everything shifts up by
   0.2 to 0.3. Under the 0.55 gate the mid JD would get a résumé.
2. The 4b extracts fewer, broader requirements from the same posting
   (7 vs 12 for the strong JD), so each verdict carries more weight.

**Decision: the gate is configuration, not a constant.** It is
model-dependent, so `JD_MATCH_THRESHOLD` now lives in `.env.prod`
next to `OLLAMA_LLM_MODEL`; the api enforces it and the web quotes
the same variable on the JD pages. Calibrated values:

| Judge model | Gate | Basis |
|---|---|---|
| `qwen3:14b` | 0.55 | strong 0.604 / mid 0.333 |
| `qwen3:4b-q8_0` | 0.72 | strong 0.842 / mid 0.625 (margins 0.12 / 0.10) |

**Recommendation for the CPX31:** `qwen3:4b-q8_0` at 8k with the
0.72 gate. Peak memory about 5.5 GB against the 7 GB cap (4.4 GB
weights, 0.6 GB q8 KV cache, buffers), judge calls in the 20 to 40 s
range on this CPU instead of 85 s, and the decision log lets the owner
grade its verdicts directly; if the agreement rate is poor, the Mac
backend (`OLLAMA_LLM_URL`, 14b, gate 0.55) is the fallback at no cost.

**Owner's real-world JD.** A Bosch "Manufacturing Automation
Engineering Lead" posting the owner expects above 0.75 was added to
the calibration set as `bosch_lead`; results on both models in the
next entry.

## 2026-09-22: the Bosch posting, and why embedding retrieval fails on facts

The owner's real posting (Bosch, Manufacturing Automation Engineering
Lead; he expects above 0.75) first scored **0.712** on `qwen3:4b-q8_0`
and **0.426** on `qwen3:14b`. The decision log showed the cause for
the worst rows:

- "2+ years of leadership or supervisory experience" → unmet. The
  four retrieved chunks were two dev-doc speaker notes about *session*
  "resume" scoring, an interview-prep story and the application
  playbook. None of the six master-résumé chunks (VP of Engineering
  2022–2026, Director of Engineering 2017–2022) were offered. The
  judge answered the evidence it had.
- "OSHA, environmental and safety compliance" → unmet, same pattern
  (worksheet, readme, article; no NFPA 70E chunk).
- "Bachelor's degree in engineering or related field" → unmet with the
  education chunk present: the extraction dropped the posting's own
  "or combination education and experience", and the judge read the
  remainder literally against a B.S. Applied Mathematics plus an A.S.
  Electrical Engineering Technology.

**Tried first: a résumé anchor** (closest master-résumé chunk by
cosine added to every requirement). No change: for the leadership
requirement the closest résumé chunk was the publications section
(similarity 0.52). Cosine similarity is the wrong tool for tenure,
title, degree and certification facts.

**Fix: an owner-maintained career facts sheet** (`source_kind`
`profile`, `docs/personal/career-facts.md`, ~2.4k chars, one chunk)
offered to every requirement the judge sees (`ProfileKind` in
`assess.go`; `profile` added to `ingest.KnownKinds` and the manifest).
Roles with dates, degrees, credentials, safety and standards
ownership, recognition. The owner edits it directly; `make
sync-corpus` plus a private reindex puts it on the box.

| JD | 4b before | 4b with profile | 14b before | 14b with profile |
|---|---|---|---|---|
| bosch_lead | 0.712 | **0.833** | 0.426 | 0.574 |
| strong | 0.842 | **0.974** | 0.604 | (not rerun) |
| mid | 0.625 | **0.667** | 0.333 | (not rerun) |

With the profile, the 4b marks leadership, degree and 7+ years met on
the Bosch posting; the remaining unmets are safety compliance (read
strictly despite NFPA 70E ownership in the sheet) and the Master's
degree, which the posting lists as preferred. The 14b still marks
"industrial networking and communication protocols" and "machine
safety standards" unmet, which the record does not support; it reads
fine-grained "must" lines more literally than a human would.

**Decisions.**
- The owner (2026-09-22): "The 14b model is not viable for our resource
  constraints so there is no reason to waste time with it." The 14b
  is dropped from all further measurement; `qwen3:4b-q8_0` is the
  model, locally and on the box, and the defaults now say so.
- Gate `JD_MATCH_THRESHOLD=0.70` (Bosch 0.833, strong 0.974 above;
  mid 0.667 below, narrowly).
- Judge calls with the profile cost ~+600 prompt tokens (3.6 s per
  call on the Mac; expect ~50 s on the CPX31).

**Owner's framing (2026-09-22):** "This is a major issue with ATS
systems that HR departments use. They rank on musts and misses that
set aside viable candidates that never make it to a human to provide
the full context." Two consequences for this system: the gate only
decides whether a résumé is generated automatically, never whether
the owner sees the submission (he sees all of them); and the result
page should show the requirement-by-requirement verdicts with their
rationales to the hiring manager, so a below-threshold result reads
as "9 of 14 met, 2 not evidenced: X, Y" instead of a rejection.
The breakdown shipped in PR 81: `GetJdResult` returns the
per-requirement verdicts with rationales (released with the result
token, whatever the outcome) and the result panel renders "N of M
evidenced, not evidenced: X, Y" above or below the gate.
`jd_requirements` v2 (keep the posting's alternative-qualification
clauses in the extracted requirement) is next.

## 2026-09-22: prod on the 4b with the facts sheet

Deployed PR 80 (`c0498e034f75`), reindexed the facts sheet on the box
(`profile`, 1 chunk), and re-scored the two prod submissions so the
decision review queue has rows.

- Submission 6 (calibration strong JD): **ready, 0.974**, 7
  requirements (6 met, 1 partial), résumé and PDF produced. Identical
  to the Mac run on the same model, as the one-requirement-per-call
  design predicts. 8 decision rows landed in `/admin/decisions`.
- Submission 5 (the live-check JD): **failed**, "embed requirements:
  embed failed: timed out". Both re-scores ran concurrently; with
  `OLLAMA_MAX_LOADED_MODELS=1` the embedder and the LLM swapped in and
  out on every alternate call until an embed exceeded the sidecar's
  timeout.

**Fix (PR 82).** JD pipelines run one at a time (`PipelineConcurrency
= 1`, a slot the pipeline acquires before its first model call;
queued submissions sit in `scoring` until their turn) and "timed out"
counts as transient, so a swap that overruns once is retried rather
than failed. One JD at a time is also faster on four vCPUs than two
interleaved.

Deployed PR 81 (`64aafef17421`): the result panel now shows the
requirement-by-requirement verdicts to the submitter.

Submission 5 re-scored alone after the PR 81 deploy: **0.611**, below
the 0.70 gate (it is the live-check script's terse synthetic posting,
not a real one). 9 judge calls at **106 s each** on the CPX31 with the
facts sheet in the prompt, about 17 minutes end to end. 18 decision
rows now in `/admin/decisions` (submissions 5 and 6).

**Speed note.** The judge prompt renders the requirement before the
evidence, so the ~900 tokens of system prompt plus facts sheet that
are identical across every call sit behind a varying prefix and
Ollama's prompt cache cannot reuse them. Rendering the profile first
(`<candidate_profile>` before `<requirement>`) makes those tokens a
shared prefix; on a CPU box where prompt evaluation dominates, that
should cut a judge call by roughly a third. Prompt layout is part of
the prompt version, so this ships as `requirement_judge` v2 together
with `jd_requirements` v2 and a fresh calibration table.

## 2026-09-22: prompts v2 and the v2 score formula

**Prompt changes.** `jd_requirements` v2 keeps the posting's own
alternatives ("or related field", "or equivalent experience", "or a
combination of education and experience") inside the requirement
text, and files anything under "preferred" / "nice to have" as
`nice`. `requirement_judge` v2 renders the career facts sheet once,
first, in `<candidate_profile>`, then the requirement and its
retrieved evidence: the system prompt plus profile become a shared
prefix Ollama's prompt cache can reuse across the one-per-call judge
pass. Verdict rules unchanged except that an alternative the
requirement itself offers counts as met.

**First v2 pass (4b, formula v1):** Bosch **0.704**, strong 0.864,
mid 0.458, weak 0.091, unrelated 0.000. Cleaner separation, but Bosch
fell from 0.833 for two reasons: the 4b still dropped "or combination
of education and experience" from the degree line despite the rule,
and the extraction now lists the posting's preferred certifications
(Master's, Six Sigma belt, PMP) as separate `nice` items that count
against the denominator when unmet. That is the ATS pattern in
miniature: a missing PMP was subtracting from a plant automation
lead's score.

**Score formula v2 (`ScoreFormula = "v2-nice-bonus"`).** Every `must`
requirement is in the denominator; a `nice` requirement joins it only
when it earned something. Preferred items can raise a score, never
sink it. Verdict values unchanged. Stored assessments carry
`score_formula` so old rows are read correctly.

**Facts sheet.** Added a plain statement under Education that the
B.S. Applied Mathematics plus A.S. Electrical Engineering Technology
satisfy "engineering or related field", "or equivalent experience"
and "combination of education and experience" requirements. The judge
reads the sheet; it does not infer equivalence on its own.

| JD | v1 prompts + profile | v2 prompts, formula v1 | **v2 prompts, formula v2, sheet** |
|---|---|---|---|
| bosch_lead | 0.833 | 0.704 | **0.917** |
| strong | 0.974 | 0.864 | **0.857** |
| mid | 0.667 | 0.458 | **0.500** |
| weak | 0.231 | 0.091 | **0.100** |
| unrelated | 0.000 | 0.000 | 0.000 |

Bosch verdicts now: 12 met, 1 unmet must ("robotics and vision
systems", which the corpus does not state; the owner can add a line
to the facts sheet if the experience exists), 3 unmet preferred
certifications that no longer count against him.

**Decision.** Gate stays `JD_MATCH_THRESHOLD=0.70`: strong 0.857 and
Bosch 0.917 clear it with margin; mid 0.500 sits 0.20 below. Every
verdict on these runs is in the decision log for the owner to grade.

## 2026-09-22: prod on v2 (PR 82, `e346ee994bb5`)

Re-scored submissions 6 and 5 after the deploy, one at a time (the
slot logic held: submission 5 waited 1,670 s for its turn).

- Submission 6 (calibration strong JD): **ready, 0.857**, 8
  requirements, identical to the Mac run. Judge calls 8 × 102 s
  (1,991 prompt tokens each); requirements call 79 s; **résumé call
  772 s** (5,021 prompt tokens, about 1,600 output tokens at roughly
  3 tokens/s). Whole pipeline about 28 minutes.
- Submission 5: judging at 95 s per call at the time of writing.
- 27 decision rows in `/admin/decisions`.

**Two findings.**

1. The profile-first judge layout gave no measurable speedup on the
   box (102 s vs 106 s before). The ollama log explains it: every
   judge call reports `cached n_tokens = 715` and then evaluates
   ~1,160 new tokens (72 s) and generates ~90 (18 s). So the prompt
   cache works and the shared prefix is simply short, because of a
   bug: `writeJudgeChunk` capped every chunk at 1,200 runes, the facts
   sheet included. The judge had been seeing only the first 1,200
   characters of the sheet (the roles); the education, safety and
   standards lines never reached it (the "Degree requirements" line
   is absent from every logged v2 prompt; the degree passed at run 53
   only because the résumé's education chunk was retrieved). Fixed:
   profile chunks are never capped, and the decision log records what
   the judge saw. Consequence for speed: the per-call cost is the four
   retrieved chunks (~1,150 tokens), not the sheet; the honest lever
   is fewer or shorter retrieved chunks, measured against verdict
   quality.
2. The résumé call is now the long pole: 13 minutes of generation.
   Levers, cheapest first: a shorter target length in `resume_tailor`
   (550–700 words is ~1,600 output tokens; 450–550 would save ~4
   minutes), or `MaxTokens` below 2,200. Not changed yet; the PDF
   quality at the current length is what the owner has approved.

**Operational change.** `JD_PIPELINE_TIMEOUT_SECONDS=3600` and
`SIDECAR_LLM_TIMEOUT_SECONDS=3000` in `.env.prod` (takes effect on the
next api/sidecar restart): a 14-requirement posting plus the résumé
is ~37 minutes at these speeds, over the old 2,400 s budget.

**Submission 5 stalled, and why (PR 84).** It waited 1,670 s for the
slot behind submission 6, then failed at judge call 7 of 9 with
`DeadlineExceeded`: the 2,400 s pipeline deadline had started at
submit time, so the queue wait consumed most of it. The failure path
then tried to write `failed` with the same expired context and could
not, leaving the row at `scoring`. Fixed: the pipeline timeout starts
when the slot is acquired (queue wait has its own 3-hour cap), and
every status write uses a context that survives the deadline. The row
was marked failed by hand with the reason.

**With the whole sheet in the judge prompt (local, 4b):** Bosch
0.917, strong 0.857, mid 0.591. Mid rose from 0.500 because the sheet
now states standards and methodologies the mid posting asks about;
the 0.70 gate keeps a 0.11 margin below and 0.16 above.

**Prod on PR 84 (`2fabaa16c6d7`), submission 5 re-scored alone:**
below threshold at **0.667** (the live-check script's synthetic
posting), 9 requirements, 9 judge calls at 116 s each with ~2,380
prompt tokens, no stall, finished in about 18 minutes. Ollama now
reports `cached n_tokens = 1130` (system prompt plus the whole facts
sheet) on every judge call, so the shared prefix is served from the
cache as intended; the remaining ~1,250 tokens per call are the four
retrieved chunks. 43 decision rows in `/admin/decisions`.

**Owner email (PR 85).** Every finished submission now mails the
owner: score against the gate, the verdict table with rationales,
the application link, and links to the admin detail, the decision
review and the PDF. Verified locally against Mailpit; audited as
`jd_outcome` in `notification_deliveries`.

## 2026-09-22: fit categories, progress, and the review email by category

Owner's spec: a modal on submit with 0 to 100% progress the submitter
can keep open or close; a finished-review email that classifies the
fit as very strong / strong / possible / weak / very weak, attaches
the two-page PDF for very strong and strong, says "I will review and
get back to you" for possible and weak, and needs no further action
for very weak. And: "add a section in the admin console that will
allow me to change the confidence numbers to classify the review."

**Implemented (PR 89).** `jd_submissions.progress_pct` /
`progress_stage`, written by the pipeline (reading the posting 5%,
gathering evidence 12%, judging requirement i of N from 15% to 75%,
computing the score 76%, writing the résumé 80%, rendering the PDF
94%, finished 100%); `GetJdResult` returns them plus `fit_category`.
Bands live in `app_settings` (`jd_fit_bands`), edited on `/admin/jd`,
cached 15 s in the api; "strong" is the résumé gate, so
`JD_MATCH_THRESHOLD` now only seeds the setting on first use. The
member RPC `GetJdReviewConfig` lets the JD pages quote the live gate.
Email providers gained attachments (Resend base64, SMTP
multipart/mixed).

**Seed bands** (from the 4b calibration with the facts sheet: strong
0.857, mid 0.591, weak 0.100, Bosch 0.917): very strong 0.85, strong
0.70, possible 0.55, weak 0.35. Under these, Bosch and the strong JD
are very strong, mid is possible, weak is very weak. The owner tunes
them from the console; changes never rewrite a stored score.

**State at the end of the day.** Everything above is in the branch
`claude_dev01` as one PR. Production remains on `SIDECAR_LLM_PROVIDER=stub`
until that PR is deployed; the flip is then the runbook's Phase C with
`SIDECAR_LLM_NUM_CTX=8192` and either `qwen3:8b` on the box or
`qwen3:14b` on the Mac over `OLLAMA_LLM_URL`.


## 2026-09-22: data layer D1, the product event stream

Not a tuning change; recorded here because the owner reads this log
for every decision. The data strategy agreed after the site review is
D1 events, D2 `jd_runs` with a `run_id` on `llm_usage` and
`decision_log`, D3 feedback and outcomes, D4 a golden set with
`eval_runs`, D5 views and `/admin/analytics`. This entry is D1.

**What shipped.** One append-only `events` table (migration 00023),
one registry of names in `internal/events`, one writer used by the
browser beacon and the api's own emit points. The web proxy sets a
first-party `career_anon` cookie (HttpOnly, 400 days) so anonymous
landing visits and later member actions line up under one visitor id.
A public `EventService.Record` RPC takes beacon batches (50 per call,
120 per minute per address); identity, device class and the salted
address hash are attached by the api, never trusted from the client.
Server-side emits sit at the success point of register, verify,
approval (all three channels), login, logout, expiry, contact, JD
submit, JD finish (with score, fit and threshold), PDF download, and
the admin decision review, rescore and fit-band edits. A daily job
blanks identity columns after `EVENT_IDENTITY_RETENTION_DAYS` (400).
The 42 `activity_events` rows were copied in. `docs/events/README.md`
is the registry and holds the funnel query; the privacy page now says
all of this in plain words.

**Why this shape.** The questions worth answering cross the anonymous
and member boundary (landing to request to approval to JD to result),
so one table with one visitor id beats the member-only activity table
plus joins. The registry in code keeps the table from filling with
ad-hoc names; a new event is a one-line addition in two places.

**Verified locally.** Proxy sets the cookie; a beacon batch with a
duplicate id, an unregistered name and a server-only name stored one
row; utm survived and a stray query parameter did not; the referrer
was reduced to its host; a real browser load wrote page.view, then
page.leave with dwell on navigation, then the next page.view.

**Prod step.** Set `EVENT_IP_SALT` in `.env.prod` (`openssl rand -hex
16`) before the deploy, or the api logs a warning and hashes with the
development salt.

**Left for later.** Reserved browser names (`article.view`,
`gallery.*`, `repo.click`, `jd.poll_abandoned`) are in the registry but
not yet emitted; `ActivityService.RecordEvents` still writes the old
table and should be retired; an admin analytics surface is D5.

## 2026-09-22: run telemetry (data layer D2)

The reviewer could not answer its most important question. A re-score
overwrites `jd_submissions`: the score, the assessment, the status and
the résumé are all replaced. So after changing a prompt, a model, or the
corpus, there was no way to say whether the change helped, because the
thing it would be compared against had been destroyed by the comparison
itself.

**What shipped.** `jd_runs`, one immutable row per pipeline run
(migration 00025). It records what produced the verdict and what the
verdict was:

- build, host, judge model, context window, embedder model
- prompt fingerprints, which are the version plus a short hash of the
  system text and schema. The version alone is a promise; an edit
  without a version bump is an easy mistake and an invisible one, and
  it would make two runs look comparable when they are not
- corpus fingerprint, a hash over the documents' content hashes, with
  document and chunk counts
- the score formula, retrieval score, match score, threshold, fit band,
  requirement count and the met / partial / unmet split
- queue time and work time separately, because one says the box is busy
  and the other says the pipeline is slow

`run_id` rides the request context, the way the tenant does, so every
`llm_usage` and `decision_log` row written anywhere in the fan-out is
tied back to its run without threading a parameter through a dozen
signatures. Token counts and cost are deliberately not copied onto the
run; they live in `llm_usage` and are summed by `run_id`, because a
second copy is a second source of truth that drifts.

A re-score now opens attempt two and records the admin who asked for
it. `/admin/jd/[id]` lists the history, so the superseded verdict is
readable beside the one that replaced it.

**Verified locally.** Two attempts on one submission, both preserved
with their own provenance; the gate decision row carried the run id;
the run closed with its outcome and timings. A run left open by a
process that died is closed at boot, beside the existing stranded
submission reconciliation.

**What this unlocks.** D4, the golden set, becomes possible: re-score a
fixed set of postings, compare runs by fingerprint, and state whether a
prompt change moved the score because of the prompt or because
something else moved underneath it.

## 2026-09-22: ground truth and judgment (data layer D3)

Everything measured up to here is the system's account of itself: the
model's verdicts, the owner's labels on those verdicts, the timings and
the provenance. All of it answers "was the reasoning sound". None of it
answers "was the answer right", and those are different questions. A
posting can score 0.92 with every requirement properly evidenced and
still be a job that goes nowhere.

**What shipped** (migration 00026), two tables because they are two
different kinds of claim:

- `jd_outcomes`: what happened in the world. One revisable row per
  posting, from a fixed vocabulary ordered the way a posting actually
  moves: not pursued, applied, screening, interview, offer, rejected,
  no response, withdrew. It carries the day the person knows, which is
  not the day they typed it in.
- `jd_feedback`: what a person thought of a run's output. Attached to
  the run rather than the submission, so a re-score does not inherit an
  opinion of the thing it replaced. Owner and submitter judgments are
  kept apart and must never be averaged together.

**Why not thumbs.** The score ratings are accurate, too generous, too
harsh, unusable. A thumb down cannot distinguish the second from the
third, and those point at opposite fixes: one says tighten the judge,
the other says the evidence is not reaching it. The résumé ratings are
would send, needs edits, wrong. A unique index keeps one current
opinion per person per run per target, so counting ratings counts
people rather than clicks.

**Verified locally.** An outcome stored with its date and note; an
invalid status rejected with the vocabulary listed back; feedback tied
to the run and the tenant; an invalid target and rating pair rejected;
and a second rating replacing the first rather than stacking.

**What this unlocks.** The calibration question stops being internal.
Once a handful of postings carry outcomes, the useful query is whether
the fit bands separate the postings that went somewhere from the ones
that did not, which is the first honest test of whether the gate is set
in the right place. Until then the bands rest on four calibration
postings and the owner's judgment.

## 2026-09-22: the golden set (data layer D4)

The gate was justified by four postings scored by hand and written into
this log. That was enough to pick a number once. It could not answer the
question that arrives with every prompt or model change: did this make
the reviewer better, worse, or neither?

**What a golden posting asserts.** Only which side of the gate it
belongs on. Not a score. A score is model-dependent, so an expected
score would have to be rewritten every time the model changed, and that
rewriting is precisely how a regression hides: the number moves, the
expectation moves with it, and nothing looks wrong. "This posting is a
real fit" is a claim about the world and stays true when the model does
not.

**Three measurements, because one is not enough** (`internal/jd/evaluate.go`):

- **Gate accuracy.** How many landed on the expected side. Coarse: a
  set can read 100 % while every score creeps toward the threshold.
- **Inversions.** Pairs where a posting expected below outscored one
  expected above. Worse than a gate miss, because a gate can be moved
  and an inversion cannot be fixed by moving it. There is no threshold
  that separates an inverted pair.
- **Margin.** The smallest gap between the two groups. This is the one
  that gives warning: it shrinks for several changes before any posting
  actually crosses, so a collapse announces itself instead of arriving
  as a sudden failure.

Failed postings are excluded from all three rather than counted as
zero. Scoring a failure as 0.0 would push the margin negative and report
a regression that did not happen.

**Same pipeline, flagged.** An evaluation creates real submissions and
runs them through the ordinary path, because a test that takes a
different path tests a different thing. They carry `is_eval`, which
keeps them out of the submission list and stops the owner being emailed
once per posting. Their runs, decisions and model calls are recorded
like any other, so an evaluation result opens into the same derivation
view as a real submission, and its decisions are extra labelled data.

**Verified locally** on the stub provider, which scores by retrieval
only: two postings, the fit one at 0.048 and the unrelated one at 0.038.
One of two on the expected side, zero inversions, margin +0.010. That
is the correct reading: the ordering is right, both sit far below the
0.55 gate because retrieval alone cannot separate them, and the three
numbers say exactly that rather than collapsing it into a pass or fail.

**Next.** Add the real calibration postings on production, run one
evaluation to set the baseline, and from then on run one before and one
after every prompt or model change. That pair is the evidence.

## 2026-09-22: every metric defined once (data layer D5)

The data layer is complete. D1 records what people do, D2 what produced
a verdict, D3 whether the answer was right, D4 whether a change helped.
D5 is the part that makes them readable, and it carries one rule worth
stating plainly because it is the rule that usually gets broken first:

**A metric is defined once, in SQL, and read.** Never recomputed in Go,
never recomputed in a page. Two definitions of the same number is how a
dashboard starts disagreeing with itself, and after that nobody can say
which one is lying. Ten views in migration 00028, catalogued in
`docs/metrics.md`.

Three of the definitions carry a judgment worth recording:

- **Evaluation runs are excluded from `v_jd_runs`.** A golden-set run
  is a test of the reviewer, not use of it. Counting them would flatter
  both the volume and the latency, and the flattery would grow every
  time the set was exercised.
- **Agreement is split into soft and hard.** A disagreement where one
  side said "partial" is the judge being unsure; a disagreement between
  "met" and "unmet" is the judge being wrong. They call for different
  fixes, so a single agreement percentage hides the one thing the
  number was supposed to tell you.
- **Queue time is never folded into duration.** One says the box is
  busy, the other says the pipeline is slow. The sum says neither.

`/admin/analytics` is arranged by the owner's own criteria for "runs
very well" rather than by what is easy to chart. Where a criterion has
no data it says so, instead of showing a zero that reads like a failure:
"no evaluation yet" and "0 percent calibrated" are very different
statements and only one of them is true today.

**The view that matters most is the emptiest.** `v_outcome_by_fit` puts
the fit band a run assigned against what actually happened afterwards.
It is the question the whole data layer was built to answer, and it will
say nothing useful until outcomes accumulate. That is the honest state
of it, and the page says so rather than pretending otherwise.

## 2026-09-22: the first human review found a systematic bias

The owner graded seven requirement verdicts on one posting. Four
agreed. Three disagreed, and all three the same way: the model said
"unmet" where the owner said "met". Nothing went the other direction.
Three harsh and zero generous is not noise.

The three were Machine Learning, Data Science and Principal Software
Engineer, all musts. Every rationale said a version of "the candidate's
profile does not mention" that skill. Meanwhile the evidence actually
retrieved for the machine-learning requirement included the owner's own
MDEMG framework: a graph memory system with vector similarity search,
Hebbian reinforcement and model-based re-ranking. The judge was handed
that and concluded he has no machine-learning experience.

**A wrong first reading, recorded because it was wrong.** The empty
`evidence_ids` on those three looked like the judge ignoring the
documents. It was not: rule 3 of the v2 prompt requires evidence_ids to
be empty for "unmet". The citation pattern was mandated, not diagnostic.
The real tell was the rationale wording, which points at framing rather
than retrieval.

**The cause.** The v2 prompt introduced the profile chunk first and
uncapped, to fix a leadership requirement that retrieval kept missing
(2026-09-22, earlier). It fixed that and created this: the judge began
treating the profile as the definitive account of the candidate, and
the retrieved documents as commentary on it. The profile is a timeline
of roles and degrees. It says nothing about machine learning, so the
judge concluded there is none.

**requirement_judge v3** (`v2:e9c650d9` → `v3:5475a41f`) says plainly
that the two kinds of evidence carry equal weight; that the summary is a
timeline and not an inventory, so its silence about a skill is not
evidence of absence; that a document showing he built a system is
evidence of the skills that system required, named or not; and that an
"unmet" rationale must say what was looked for in the documents and may
not cite the profile's silence.

**Unverified.** The local provider is a stub, so this cannot be tested
here. The test is a re-score of that posting on production and, properly,
an evaluation before and after. That is what the golden set is for and
it is the first real use for it.

**A second, separate gap that is the owner's to close.** The facts sheet
genuinely does not mention software engineering, data engineering or
machine learning. His review notes name the work: the three-layer
control system with model-predictive control, the MES, warehouse and
historian software behind Whiskey House, and MDEMG. A better prompt
cannot supply facts that are not written down. Fix the sheet and the
prompt separately, with an evaluation between them, or neither effect
will be attributable.

## 2026-09-22: "I could not judge this" is a third answer

The same review exposed a defect in the grading tool. On the gate row
the owner had two options, agree or disagree, and neither was true: the
evidence in front of him was not enough to make a credible call. He
picked one deliberately to raise the issue.

Forcing a verdict there manufactures a wrong label, and a wrong label is
worse than no label, because the agreement rate then counts it as
signal. `insufficient_evidence` is now a third option on both the
requirement verdicts and the gate, excluded from the agreement rate
rather than counted against it, and surfaced on its own.

It is the most actionable grade in the set. Disagreement says the judge
reasoned badly, which a prompt can address. Ungradeable says the right
documents never reached the judge at all, which no prompt will fix.
Those need opposite responses and used to be indistinguishable.

The agreement summary now also splits disagreements by direction, too
harsh against too generous, for the same reason: this review is three
harsh and zero generous, which is a bias with a fix, and a single
percentage would have hidden it.

## 2026-09-23: the first question is whether it is a job description

Submission 9, the one whose grading exposed the judge bias, was not a
job description. The whole input was 365 characters of a job-search
worksheet: two company names and the search terms to use on each. The
extractor had not flattened a posting into keywords, as first read. The
keywords were the input.

That reframes the earlier finding. The requirement extraction was not
degenerate; it faithfully structured what it was given. But the system
then judged seven "must" requirements, scored 0.571, and presented it as
a verdict about fit, with nothing anywhere saying "this is not a job
description". A confident number computed from the wrong kind of input
looks exactly like a real one, and the reader has no way to tell.

The owner's grade on that gate, "not enough information for me to make a
credible review", was the correct answer, and the tool had no way to
record it. Both defects came from the same submission.

**posting_check v1** runs before anything expensive. It answers one
question, is this a job posting, and it:

- costs one small call, before retrieval and judging
- **fails open**. A classifier that is down, slow or confused must never
  stop a real posting being assessed. Wrongly refusing a hiring
  manager's posting costs far more than scoring something odd, so every
  error path returns "carry on"
- logs its verdict like every other decision, so it can be reviewed and
  graded and its agreement measured. A gatekeeper nobody can audit is
  how a system quietly starts refusing real work
- has a length guard in code for the obvious cases (an empty box, a
  pasted link) purely to save the call. The guard sits well below any
  plausible posting: the 365-character worksheet passes it and is
  judged on meaning, because the difference between that and a terse
  posting is meaning, not length

The refusal is a status of its own, `not_a_posting`, not a failure. The
submitter is told what the text looked like and what to do, in the
server's words, and no email goes to the owner: a mis-paste is not news.

**What the prompt refuses to do.** It does not judge tidiness. A posting
pasted with navigation junk around it, badly formatted, brief, or
missing a salary is still a posting. What disqualifies text is
describing no role: a list of search terms, a résumé, a board's search
results, a fragment.

**Unverified against the real model.** The local provider is a stub, so
the assessor is disabled and this path does not run here. The
deterministic half is covered by tests. The real test is production,
re-submitting that same worksheet and seeing it refused.

## 2026-09-23: the fix over-corrected, and the golden set caught it

First real before-and-after on a genuine posting (Blue Origin, Senior
Software Development Manager, AI Manufacturing Execution Systems).

| | run 1 | run 2 |
|---|---|---|
| judge prompt | v3:5475a41f | v3:5475a41f |
| corpus | 853404a1b788 | a3ec0c636121 |
| score | 0.800 (strong) | 1.000 (very strong) |
| met / partial / unmet | 11 / 0 / 3 | 14 / 0 / 0 |

Between the runs, three spans were added to the career facts sheet: the
MPC/APC work (15+ years), MDEMG (2024 to 2026, two years, sole
developer) and the in-house Whiskey House manufacturing software. The
documents already described what these systems do; nothing stated how
long they took, which is what a "N+ years of X" requirement needs.

**A perfect score is not a win.** Two of the three that flipped were
wrong, and the owner had already said so about one of them.

- The requirement to collaborate with Anthropic, AWS and OpenAI as
  external partners flipped to met, reasoning that MDEMG "integrates
  with LLMs and supports collaboration with external AI providers
  through gRPC, DevSpace hub, and inter-agent messaging". Calling an
  API is not a partnership with the company behind it.
- "5+ years in building products that apply or incorporate Machine
  Learning Models" flipped to met citing MDEMG, which the facts sheet
  now states is two years. The judge never consulted a span, and never
  touched the fifteen years of MPC/APC added for exactly this purpose.

**The cause was v3's rule 3**, written that morning to fix the opposite
failure: "a document showing the candidate built a system is evidence of
the skills that system required, named or not". True, and necessary, but
it said nothing about what "evidence of" may establish. So the judge
swung from reasoning only about the profile summary to inferring
durations and business relationships from a repository existing.

**v4** (`v3:5475a41f` → `v4:52e4ba2e`) keeps rule 3 and bounds it:
inference establishes capability and nothing else. It may never
establish a length of time, a relationship with a named company, a
credential, or an employer; those must be stated in the evidence. A
requirement asking for a minimum number of years must find a stated
span covering that much of that work, met when one does, partial when
the work is evidenced but no span reaches the length, and never
inferred from a system existing or from total career years in another
field.

**Prediction recorded before running it**, so the next entry is a test
rather than a rationalisation: vendor requirement back to unmet, the
two-year LLM requirement stays met because the span now says two years,
the five-year ML requirement lands on partial. Roughly twelve met, one
partial, one unmet, high eighties. If it returns 1.0 again, the prompt
is not the lever and retrieval is.

**What this says about the method.** Two changes were made at once by
accident of sequencing (prompt v3 shipped, then the corpus changed), and
the run records still separated them: same prompt fingerprint, different
corpus fingerprint, so the movement is attributable to the corpus. That
is the entire reason D2 records provenance, and it paid for itself on
the first real comparison.

## 2026-09-23: v4 changed nothing, so the judgment moved into code

The prediction in the previous entry was wrong, and usefully so.

| run | judge | corpus | score | met / partial / unmet |
|---|---|---|---|---|
| 1 | v3:5475a41f | 853404a1b788 | 0.800 | 11 / 0 / 3 |
| 2 | v3:5475a41f | a3ec0c636121 | 1.000 | 14 / 0 / 0 |
| 3 | v4:52e4ba2e | a3ec0c636121 | 1.000 | 14 / 0 / 0 |

v4 bounded the inference rule in prose: capability may be inferred,
durations and named relationships may not. It produced an identical
score, identical verdicts, and for the five-year machine-learning
requirement a rationale **word for word identical** to the previous run.

Both excuses were checked and neither held. The v4 text was genuinely
sent (its new wording is in the stored `prompt_text`), and both spans
were in the evidence for that requirement: the fifteen years of MPC/APC
and the two years of MDEMG. The model had the rule and the facts, and
still answered a five-year requirement by pointing at a two-year
project.

**The conclusion is about the method, not the wording.** Three prompt
versions tried to teach a 4B model to police its own inference. Prose
rules asking a small model to reason about its own reasoning do not
work, and the third attempt produced byte-identical output, which is as
clear a null result as this setup can give.

**So the judgment moved where the score already lives.** The project
already holds that the model never emits a number, because a number is
arithmetic and arithmetic belongs in code. A duration check is a
comparison between two numbers, and it belongs there too.

- `requirement_judge` v5 adds one required field, `stated_span_years`:
  the number of years the evidence explicitly states for the work in
  question, 0 when it states none. The prompt is explicit that this is
  a fact being reported, not a decision being made.
- `internal/jd/duration.go` reads how many years the requirement demands
  from the requirement's own text, and compares. A "met" whose stated
  span falls short becomes "partial", with a note saying why.
- It only ever weakens a verdict. An "unmet" judge knows something the
  rule does not, and the rule must never argue a candidate upward.
- It downgrades to "partial" rather than "unmet" because the work was
  evidenced; what is missing is proof of how long, which is a lesser
  claim rather than no claim.
- Compound requirements take the larger span: "10+ years of software
  engineering with 3+ years leading teams" asks for ten, and reading the
  three would let the easier half satisfy the whole.
- The adjustment is stored on the judgment and rendered on the admin
  derivation. An adjustment nobody can see is indistinguishable from the
  model having said so itself.

Tested in code rather than by rescoring: travel percentages, team sizes,
standard numbers like NFPA 70E, and a founding year must not read as
durations, and none of them do.

**Still outstanding, deliberately.** The vendor requirement ("collaborate
with partners like Anthropic, AWS, OpenAI") is still wrongly met, and
this change does not touch it. Relationships have no deterministic
signal in the requirement text the way "N+ years" does, so it needs its
own approach and its own measurement. One change, one comparison.

## 2026-09-23: the duration rule worked, so relationships got the same treatment

| run | judge | score | met / partial / unmet |
|---|---|---|---|
| 1 | v3:5475a41f | 0.800 | 11 / 0 / 3 |
| 2 | v3:5475a41f | 1.000 | 14 / 0 / 0 |
| 3 | v4:52e4ba2e | 1.000 | 14 / 0 / 0 |
| 4 | v5:6f1e98c4 + code | 0.971 | 13 / 1 / 0 |

Run 4 is the first change that did what it was meant to. The five-year
machine-learning requirement came back partial, carrying its own
explanation: "asks for 5 years; the evidence states 2". The reported
spans show the rule discriminating rather than blunting: 30 against the
ten-year engineering ask, 2 against the two-year LLM ask (enough, so
met), and nothing at all on the nine requirements that ask for no
duration, which the rule never touched.

**What it did not fix, by design.** The vendor requirement was still
met, so 0.971 still overstates a real application. Partial credit on one
requirement barely moves a weighted score when the other thirteen are
met.

**Relationships now work the same way**, because the same division of
labour is what worked:

- `jd_requirements` v3 (`v2:5a8616c7` → `v3:357938f1`) extracts
  `named_parties` per requirement: organisations the requirement names
  and expects a working relationship with. Companies, clients,
  institutions, employers. Explicitly not technologies, languages,
  standards or frameworks, because "experience with Kubernetes" names no
  party and treating it as one would downgrade half the posting.
  Extracted once per posting, since which companies a requirement names
  is a fact about the posting rather than a judgment about the
  candidate.
- `requirement_judge` v6 (`v5:6f1e98c4` → `v6:409a63a4`) adds
  `parties_evidenced`: which of those the evidence names as
  organisations the candidate worked with or for. The prompt says
  plainly that using a company's product, calling its API or
  integrating with its service is not working with that company.
- `internal/jd/relationship.go` compares the two sets. A "met" on a
  requirement naming organisations, where the evidence names none of
  them, becomes "partial".

Same constraints as the duration rule: it only weakens, never
strengthens; it weakens to partial rather than unmet, because the
capability may be real and only the named relationship is missing; and
the adjustment is stored and shown, since one that cannot be read is
indistinguishable from the model having said so itself.

**Matching is deliberately generous**, because a false downgrade
understates a real candidate and costs more than a loose match. Case and
punctuation are ignored, corporate suffixes stripped, containment works
either way, and acronyms match their expansion: a posting saying "AWS"
is satisfied by a document saying "Amazon Web Services". That last case
failed on the first run of the tests, which is why it is now handled;
containment alone never matches an acronym against its expansion.
Acronym matching is capped at five words, beyond which shared initials
are coincidence rather than a name anyone uses.

## 2026-09-23: the golden set can now surprise us

Both postings in the set were chosen by the owner, and one of them was
a job he had applied for, so its expected outcome was his application
decision restated. A set like that can confirm what he already believes
and cannot produce an unwelcome result, which means it was not
measuring anything.

**What shipped** (migration 00030):

- `selection` on each posting: `chosen` (the owner picked it) or
  `random` (drawn from a search without picking for a known answer),
  plus `source` so a row can be traced back when a link dies.
- `expected_gate` may now be empty, which is the unlabelled state. A
  random posting arrives that way on purpose. Evaluations skip
  unlabelled postings rather than guessing, and say so rather than
  reporting an empty set.
- `LabelGoldenPosting`, separate from the upsert, so recording a
  judgment does not mean resending the posting text.
- The console shows each posting's **full text** behind a disclosure,
  with a label control on the unlabelled ones. This was the missing
  piece: the owner asked for it directly, and he was right, because
  labelling a posting you cannot read is guessing with extra steps.

**Three random postings loaded**, from web searches in adjacent fields,
taken in the order the results came back rather than picked:

| posting | why it is interesting |
|---|---|
| Process Control Engineer, Dover Chemical | his hands-on wheelhouse, but well below his level |
| Automation Engineer (GMP), CAI, Limerick | automation, wrong industry and wrong continent |
| General Manager, commercial manufacturing, Orca Bio | senior site leadership, but 15 years of it in biologics |

None of the three has an obvious answer, which is the point. Two are
rendered from the page rather than byte-verbatim, which the `source`
field records; the Dover one is extracted from the original PDF and is
verbatim.

**Still to do before this pays off.** The owner labels them, the set
grows toward ten, and calibration is reported separately for chosen and
random postings. A gate that is clean on chosen postings and noisy on
random ones is the most useful thing the set can tell us, and averaging
the two together would hide exactly that.

## 2026-09-23: three labels that never left the browser

The owner labelled the three random postings and asked for an
evaluation. Production had none of them. Worth recording, because the
first instinct was to doubt the report rather than the plumbing, and
the plumbing was at fault.

**What the evidence said.** The production `golden_postings` table had
all three randoms still at `expected_gate = ''`. The API log for the
preceding six hours held exactly one `LabelGoldenPosting` call, a
`{"id":"999999"}` probe sent to test routing, which answered `404
posting not found`: the endpoint is wired and reachable. There was also
no page render from the owner's browser in that window, no
`ListEvalRuns`, nothing. The local database held one label,
`random-cai-automation-engineer = below`, timestamped during an earlier
test of the label path and therefore not the owner's.

So the clicks never reached any server. The deployed web image was
checked directly and does contain the label form and the corrected
capability wording, which rules out a stale image.

**Most likely cause.** A deploy finished at 18:47:38. A tab opened
before that holds server action IDs from the previous build, and after
the containers restart those submissions are discarded rather than run.
The click looks accepted and nothing happens. The absent page render
fits: navigating inside the admin console can be served from the client
router cache without touching the server, so a tab can sit on a build
that no longer exists. An expired session is the other candidate, since
`labelGoldenAction` returns "Not signed in." without calling the API,
and that message renders small enough to miss.

**What this argues for.** Any console control whose failure is silent
is a control that cannot be trusted to record a judgment. This is the
second time in this project a review action has been lost quietly; the
first was the decision vocabulary mismatch that `useActionState` now
surfaces. The pattern worth keeping: a write that does not land must
say so where the person is looking, and a deploy that invalidates open
tabs should not be able to swallow one.

**What shipped.** `StaleBuild` in the admin layout. The layout reads
the version the API reports at render time and hands it to a client
component, which re-reads `/api/readyz` on mount, whenever the tab
becomes visible again, and every two minutes. Web and api roll out
under one image tag, so a disagreement between the two means the page
belongs to a deploy that no longer exists. When they disagree a bar
appears across the top of the console saying so, naming both versions,
with a reload button. It cannot make a stale submission succeed. It can
stop one being mistaken for a success, which is the part that cost real
work here.

## 2026-09-23: the first five-posting run, and the ceiling it hit

The run died at exactly two hours, three postings in, with `context
deadline exceeded`. Two separate faults, one visible and one not.

**The ceiling.** The job runner cancels anything past two hours. That
suits work bounded by the size of the corpus. An evaluation is bounded
by the language model: every posting goes through the real pipeline
with one judge call per requirement, and on this box a single posting
takes the better part of an hour. Five postings was already a
four-hour job. The kind now carries its own deadline, eight hours,
loose enough that reaching it means something is wrong rather than
merely slow.

**The silent one.** When the deadline fired, the evaluator closed the
run using the context that had just expired, so the write did nothing.
The run sat at `running` for ever, with no summary and no record of how
far it got. Bookkeeping now uses `context.WithoutCancel`, and the note
records where it stopped. A run that died should say so in the table,
because that table is the only place anyone looks afterwards.

**What the two completed postings said**, gate 0.700:

| posting | expected | scored | |
|---|---|---|---|
| random-orca-general-manager | above | 0.786 | strong |
| ati-director-process-control-automation | above | 1.000 | very strong |

**A label was corrected mid-run.** Orca was first labelled `below` with
the note "not the type of position that translates well to my core
skillset". The owner then restated it: "I do have the experience and
skill set to do this job", and the earlier note was a personal
preference, which is a parameter filter and not a capability judgment.
The reviewer's 0.786 was right and the expectation was wrong.

Two things follow. First, the guidance on the label form was not enough
on its own: the note field invited a reason, and the reason that came
naturally was about fit. A field that only accepts a capability
sentence, or a separate place to record whether he would take the job,
would have kept them apart. Second, and worse for measurement, the set
now has no posting expected `below` at all. `separation` needs both
groups, so it reports no margin, and gate accuracy degenerates into
"did everything score above 0.700", which a reviewer that says yes to
everything would pass perfectly. The set cannot currently detect the
failure that costs the owner something: a generous reviewer tailoring a
résumé for a job he cannot do. The next postings drawn must include
ones that are a genuine no on capability.

## 2026-09-24: the first run that could have failed, and did not

Eight postings, five expected above the gate and three below, with all
three of the low side drawn at random from adjacent fields rather than
picked. 8 of 8 on the expected side, 0 ordering violations, margin
0.143, 6h13m45s. Model `qwen3:4b-q8_0`, build `3e6107d71ebf`, corpus
`a3ec0c636121`, gate 0.700.

| | posting | expected | score | fit |
|---|---|---|---|---|
| chosen | ati-director-process-control-automation | above | 1.000 | very strong |
| random | random-dover-process-control-engineer | above | 0.956 | very strong |
| random | random-cai-automation-engineer | above | 0.865 | very strong |
| chosen | blue-origin-mes-ai-manager | above | 0.857 | very strong |
| random | random-orca-general-manager | above | 0.786 | strong |
| random | random-nexus-power-system-studies | below | 0.643 | possible |
| random | random-xai-structural-data-centers | below | 0.357 | weak |
| random | random-profluent-ml-pretraining | below | 0.308 | very weak |

**The groups do not interleave at all.** Every posting expected above
scored above every posting expected below. That is what makes 8 of 8
worth anything: gate accuracy alone can be bought by a reviewer that
says yes to everything, and until this run the set had no low side to
catch one.

**Random calibration matches chosen calibration.** The two chosen
postings scored 1.000 and 0.857; the three random postings expected
above scored 0.956, 0.865 and 0.786. If the reviewer were flattering
the owner's own picks, the chosen rows would sit high and the random
rows would scatter. They do not. This was the split that was worth
building the `selection` column for, and it reports no difference.

**The ordering inside the low side is defensible on its own terms.**
Power system studies at 0.643 is genuinely adjacent work gated on ETAP
depth and a BSEE, and the reviewer treated it as a near miss. Structural
engineering at 0.357 is gated on a PE licence. Protein-design
pretraining at 0.308 is gated on a PhD and top-venue publications. The
system is grading distance, not merely failing to match words.

**The gate is well placed and should not move.** The margin runs from
0.643 to 0.786 and its midpoint is 0.7145, against a threshold of
0.700. There is no evidence here for changing it.

**One reproducibility result, unplanned.** Orca scored 0.786 in the run
that was cancelled at the two-hour ceiling and 0.786 again here, on the
same input under the same configuration. An earlier note recorded that
the model's reported `stated_span_years` was not stable between runs on
identical input; this says the arithmetic downstream of it can still
land on the same number. One matched pair is not a stability claim, but
it is the first evidence either way.

**What this baseline is, and what it is not.** It is a floor: any later
change to a prompt, a model or the corpus that drops gate accuracy below
8 of 8, introduces an inversion, or shrinks the margin below 0.143 is a
regression, and the numbers now exist to say so. It is not a claim that
the reviewer is right in general. Eight postings is a small set, the
low side is three rows, and the tightest pair, Orca at 0.786 against
Nexus at 0.643, is the one that will flip first.

## 2026-09-24: the golden set measures the owner's path, not a visitor's

Retrieval is now scoped by visibility (`internal/corpusscope`), so a
member's submitted posting retrieves from public documents only while
the owner's own submissions and evaluation runs retrieve from the whole
corpus. In production that is 52 chunks against 239.

This matters for how the numbers here should be read. Every measurement
in this log, including the 8 of 8 with margin 0.143 recorded today, was
taken with the full corpus in reach, because an evaluation exists to
measure what the owner gets when he reviews a posting for himself. None
of it describes what a recruiter submitting a posting will see. Their
reviewer draws on five public documents and will be thinner and less
evidenced, and we have not measured it at all.

Two things follow. A golden-set run cannot be cited as evidence about
the member-facing product. And if the member path is worth measuring,
it needs its own runs under the public scope, which would double the
cost of an evaluation on a box where one already takes six hours. The
cheaper answer is to publish more of the corpus, which is a content
decision rather than a code one.

Why the control is in SQL rather than the prompt: the judge prompt
already asked the model not to quote private chunks, and a submitted
posting is text the submitter wrote being fed to that model. Asking the
model to keep a secret it has been handed is not a control, it is a
hope. The same reasoning as duration and relationship: the model
reports, the code decides.

## 2026-09-24: a citation is not evidence

`jd/resume.go` called a line verified when its cited chunk id was among
the ids offered to the model. That catches an invented citation and
nothing else. A line could cite a real document that says nothing like
it and still be printed on a résumé sent to an employer, with a
citation attached, which is precisely what stops a reader checking.

**What now has to hold.** Every line is compared against the text of
the chunks it cites, and dropped when it asserts something they do not
carry:

- a quantity in the line that appears in none of its sources;
- a named organisation in the line that appears in none of its sources.

Nothing else. "Led the team through a difficult migration" is not
checkable this way and passes untouched. Pretending otherwise would
drop true lines to look rigorous.

**Why this is code and not a model call.** One judging call per line
would add roughly twenty minutes to a review that already takes fifty.
The duration and relationship rules established the cheaper lesson
earlier this week: prose instruction to the model changed nothing
measurable, structure in code changed the score. Fabrication shows up
first in the specifics, and specifics are what code can check.

**Tuned to miss rather than over-report.** A missed check costs a check
that would have passed; a false one costs a true line. Three rules
follow from that, and each came from a test that failed first:

- A token carrying letters is hardware, not a quantity. `S7-1500` read
  as a claim of 1500 on a line that was entirely true.
- A token carrying digits is hardware, not an employer, for the same
  reason on the same line.
- The first word of a line is capitalised because it is first. A
  capitalised run that starts the line drops its first word, so
  "Commissioned Acme Steel controls" is checked on "Acme Steel". A
  company that opens a sentence goes unchecked, which is the cheaper
  error.

Acronym matching is shared with the relationship rule, so AWS and
Amazon Web Services are one employer here too, in both directions.

**What is recorded.** `unsupported` counts the lines removed and
`unsupported_claims` says what each asserted, both in the stored résumé
JSON. They are deliberately separate from `dropped`, which counts lines
with no usable citation at all: a model citing nothing and a model
citing something that does not say what it claims argue for different
fixes.

**Not yet measured.** No generated résumé has been through this. The
next real submission is the first evidence of whether it drops nothing,
something, or too much, and the count is the thing to look at.

## 2026-09-24: an outage caused by testing in an order that cannot happen

Migration 00032 narrowed the agreement metric to submissions the
posting-check gatekeeper accepted. It dropped `v_judge_agreement` and
its summary to redefine them. goose applies 00031 first, which creates
`v_gate_agreement` on top of that summary, so by the time 00032 ran the
drop was refused. The api crash-looped, `readyz` returned 502, and
production was rolled back to the previous tag to restore service.

It passed locally because the two migrations were applied by hand in
the opposite order, which is an order goose can never produce. The test
was not weaker than the real thing, it was a different thing.

The second attempt replaced the view in place rather than dropping it,
which removes the dependency problem entirely since the column list was
meant to be unchanged. It was written from the definition in 00028.
Migration 00029 had since added `gradeable` and `human_note`, so the
replacement dropped two columns and Postgres refused that too. A view
replacement has to be written against the view as it now stands, which
means reading `pg_get_viewdef` rather than the migration that first
created it.

**What now catches this.** CI applies every migration to an empty
database in version order and then selects from every view. Both
failures take seconds to reproduce there and neither is visible in a
diff. The rule worth keeping: a migration is only correct in the order
goose runs it, against the schema the migrations before it left behind,
and the only honest way to know that is to replay them.

## 2026-09-24: thirty-four minutes of the fifty were a cache that never hit

Time to a result fails its criterion: median 47.9 minutes against a
target of 20. A run breaks down as

| prompt | calls | avg | total | tokens in/out |
|---|---|---|---|---|
| requirement_judge | 14 | 146.6 s | 34.2 min | 3108 / 131 |
| resume_tailor | 1 | 763.1 s | 12.7 min | 4845 / 882 |
| jd_requirements | 1 | 201.8 s | 3.4 min | 1177 / 797 |
| posting_check | 1 | 34.4 s | 0.6 min | 751 / 33 |

131 output tokens against 3,108 input says the judge is bound by
prompt evaluation, not generation. The prompt was built to exploit
that: the career facts sheet is rendered first so that it and the
system prompt form a prefix Ollama can reuse across the fourteen calls.
A direct measurement on the box:

```
call 1: prompt_eval_count=2515  prompt_eval_s=152.1
call 2: prompt_eval_count=2513  prompt_eval_s=2.0
call 3: prompt_eval_count=2513  prompt_eval_s=1.7
```

Seventy-five times faster when the prefix is shared, and the box is
configured for it: `OLLAMA_NUM_PARALLEL=1`, one loaded model, a thirty
minute keep-alive.

**It was never being shared.** The profile block was assembled from the
retrieved evidence, and retrieval runs per requirement, so a different
requirement pulled a different subset of the profile. The prefix
diverged on the second call and nothing was ever reused. The comment
above the code described the intent accurately and the code did not
implement it, which is the kind of thing that survives review precisely
because the comment reads as a justification.

**The fix** passes the profile in explicitly, fetched once per run and
sorted by chunk id, so the block cannot depend on what was retrieved.
Four tests pin the property: two different requirements must render an
identical prefix, the order the rows arrive in must not matter, a
profile chunk that is also retrieved must not print twice, and an
absent profile must still render.

**What to expect, stated before measuring so it can be wrong.** Call
one still pays for the full prompt. Calls two to fourteen should pay
only for the requirement and its evidence, roughly 1,200 tokens rather
than 3,100. If prompt evaluation dominates as the numbers suggest, the
judge pass should fall from about thirty-four minutes to somewhere near
ten, and a run from 50.9 to around 28. That is a large improvement and
still short of the twenty-minute target, which would then need
`resume_tailor` addressed next: 12.7 minutes, and unlike the judge it
is generation-bound at 882 output tokens, so caching will not touch it.

The prompt is version 7. Nothing about what the judge is asked changed,
but what it is shown did: every requirement now sees the whole facts
sheet rather than the part retrieval happened to surface. The golden
set is the check on whether that helps, hurts or does nothing, against
a baseline of 8 of 8 with a margin of 0.143.

## 2026-09-24: the correction, an hour later

The entry above predicted that making the judge's prefix genuinely
shared would take a run from 50.9 minutes to about 28. It did not, and
the prediction was wrong for an instructive reason.

Measured on the box, in the sidecar's own request shape (`/api/chat`,
system message, schema-constrained decoding), with a realistic
per-requirement tail:

```
call 1: prompt=3565 tok in 266.0s | output=73 tok in 20.4s | total 286.6s
call 2: prompt=3565 tok in 127.9s | output=69 tok in 19.5s | total 147.4s
call 3: prompt=3565 tok in 128.0s | output=70 tok in 19.2s | total 147.3s
call 4: prompt=3565 tok in 128.9s | output=69 tok in 19.6s | total 148.5s
```

Cold 266 s, cached 128 s. The cache works, and it was already working:
production under v6 showed the same shape, one slow call then thirteen
at about 139 s. The 146.6 s average was the cached case all along.

**Where the earlier reasoning went wrong.** The first measurement used
`/api/generate` with a one-line tail and showed 152 s falling to 2 s. I
read that as "the cache is not being used in production" when it
actually showed "a prompt that is almost entirely prefix is almost
entirely free". Production prompts are not almost entirely prefix. Two
thousand of the 3,565 tokens are the cached system prompt and facts
sheet; the other 1,500 are the requirement and its four evidence
chunks, which differ every call by design and cost about 120 s at the
12 to 13 tokens a second this box evaluates at.

The fix still stands on its own: the prefix was assembled from
retrieved evidence and genuinely did diverge, and four tests now pin
it. It was worth making for reproducibility. It was not worth making
for speed, and saying otherwise before measuring was the error.

**What would actually make it faster**, and what it costs: the judge
pass is 34 of the 50.9 minutes and is dominated by evidence tokens, so
only sending less evidence moves it. Four chunks at 1,200 runes down to
two at 900 would take the tail from about 1,500 tokens to about 600,
and the judge pass from 34 minutes to roughly 14. That is a quality
trade, not a free win: the chunk that carries a verdict may be the one
dropped. The golden set is the instrument, against 8 of 8 and a margin
of 0.143, and no such change should ship without a run through it.

## 2026-09-24: leaving the pipeline alone, on purpose

Decision by the owner, after the measurements above: stop optimising
time to a result. A review takes about fifty minutes because the box
evaluates prompts at 12 to 13 tokens a second and the judge must read
fresh evidence for every requirement. Caching is already working and
already counted. What remains is cutting evidence per requirement,
which buys about twenty minutes and risks the verdicts.

His reasoning, and it is the right one: this is a resource constraint,
not a defect, and continued attempts risk reducing the effectiveness of
a pipeline that currently works. The golden set says it works, 8 of 8
with a margin of 0.143, and that is the thing worth protecting.

Recorded here because the next person to look at a 50-minute run will
reach for the same optimisations, and should know they were measured,
costed and declined rather than missed.
