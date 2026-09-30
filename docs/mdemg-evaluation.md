# Should career-site adopt MDEMG?

An investigation, 2026-09-29, prompted by the owner: the codebase is
getting sizable, and MDEMG's `ingest-codebase` plus its "Jiminy"
subsystem would give a coding assistant a set of specified corpora to
check its own output against.

Four parallel investigations: what MDEMG does, what career-site's
complexity actually measures, what this repo already does for the same
goal, and what adoption would cost. Findings below, then a
recommendation.

**Short version.** Two different questions were being asked at once.
MDEMG does not track code complexity and does not claim to. It does do
the guardrail thing, cheaply and with less coupling than expected. The
complexity question has a separate, boring, unrelated answer that is
currently missing.

---

## 1. Is the codebase actually getting sizable?

Not by size. By velocity, yes.

| Area | Files | LOC |
|---|---|---|
| services/api, hand-written | 113 | 22,653 |
| apps/web/src, hand-written | 117 | 18,018 |
| services/sidecar, hand-written | 9 | 957 |
| proto/ | 15 | 5,598 |
| **hand-written total** | **239** | **~41,600** |
| generated (Go, TS, Python) | 76 | ~56,500 |
| docs/ | 167 | 46,559 |

~42k hand-written lines is small-to-medium, not large. The real number
is the clock: **the repo is 11 days old**, 487 commits, and still
adding roughly 10k net lines a week with no sign of slowing. Nothing
here grew organically and settled; it was built fast and is still
accelerating. That is the thing worth managing.

**Generated code now outweighs hand-written code**, driven almost
entirely by one service: `AdminService` holds 56 of the repo's 114
RPCs.

Where complexity is actually concentrating:

- **`internal/handlers` is a god package.** Highest fan-out (imports 12
  other internal packages), highest churn, and it holds the only file
  over 800 lines (`admin.go`, 1,532 lines, touched in 34 commits).
- **`internal/jd`** is second on both counts: 25 files, 5,445 LOC.
- `handlers` + `jd` + `users` are about 68% of hand-written Go.

**The clearest gap has nothing to do with any of the above:**

| Area | Test files | Source files | Ratio |
|---|---|---|---|
| services/api | 25 | 113 | 22% |
| services/sidecar | 5 | 9 | 56% |
| **apps/web** | **0** | **117** | **0%** |

Verified directly: `apps/web/package.json` has no `test` script and no
testing dependency of any kind. 36 pages and the whole admin console
have no automated test.

## 2. What MDEMG actually is

A Neo4j-backed persistent memory graph for AI agents, v0.11.0-beta.1,
already installed here via the owner's Homebrew tap. It stores
observations in a layered graph (L0 observations up to L5 principles)
with hybrid vector, keyword and graph retrieval.

**`ingest-codebase` is real and matches the stated use case.** It walks
a repo with per-language handling, supports incremental re-ingest from
a git diff, extracts tree-sitter symbols with file and line evidence,
and takes a `--space-id`, so **several distinct corpora can be ingested
and kept logically separate**. That is exactly the "set of specified
corpi" the owner described.

**It does not do code-complexity analysis.** Confirmed twice by
independent greps across `cmd`, `internal`, `pkg` and `api`: the only
matches for complexity or cyclomatic are an unrelated Big-O comment and
a UI string. There is also no structured ADR type; "decision" is one of
fifteen generic observation types.

## 3. How Jiminy would actually steer a coding session

It attaches as Claude Code hooks, entirely inside `.claude/`. Nothing
in career-site's build, compose files or CI is touched.

| Hook | Script | Effect |
|---|---|---|
| SessionStart | `session-start.sh` | loads context |
| UserPromptSubmit | `prompt-context.sh` | injects J17 guidance every prompt |
| PreToolUse (Write/Edit) | `pre-write-check.py` | **can deny the write** |
| PreToolUse (Bash) | `pre-bash-check.py` | can deny; plus an always-on destructive-command blocker |
| PostToolUse | `post-tool-observe.py` | observes, never blocks |
| PreCompact | `pre-compact.sh` | saves state before compaction |

Three properties make this much safer than "an unfinished project can
block my edits" suggests:

1. **Blocking is opt-in and off by default.** `pre-write-check.py`
   looks for `~/.mdemg/.jiminy-strict-mode` and exits immediately if it
   is absent, without even contacting the server.
2. **It fails open.** If the MDEMG server is unreachable the action is
   allowed, with a loud stderr warning and a marker file so the next
   run knows enforcement was off.
3. **Denials are recoverable.** The deny message carries the violated
   codes and a copy-pasteable `mdemg jiminy override apply` command.

Two real limitations: the classifier only sees the **first 2,000
characters** of a write, and the guidance channel is advisory only.

### J17, the protocol it talks to the model in

A three-tier adaptive encoding: T1 `C:!|code|ann:val` at ~15 tokens
carries 80% of traffic, T2 telegraphic ~50-100 tokens carries 15%, T3
full prose ~200+ tokens carries 5%. A self-describing bootstrap header
defines the codebook once per session.

A comprehension benchmark in the repo scored a frontier model at
**T1 10.0/10, T2 9.9, T3 9.6**, 19 of 23 perfect. The compressed tier
was understood better than full prose at roughly a thirteenth of the
tokens. Caveat: that document carries a "DESIGN HISTORY, superseded"
banner, is dated 2026-03-22, and ran against a different model, so the
numbers should be re-measured with `cmd/j17-comprehension-test` rather
than quoted.

### How criteria are defined, which is the crux

**There is no rule file or schema.** Constraints are detected by regex
from natural-language sentences in observations and `CLAUDE.md`:
must/always/required, never/forbidden, should/prefer, above a 0.6
confidence floor, then promoted to constraint nodes in the graph. The
documentation's own worked example is the literal sentence "You must
never commit to main".

So "user defined criteria" in practice means: write the rule as a
sentence in a corpus, and a regex promotes it. That is blunter than it
sounds from the outside, and the docs admit it cannot reliably split a
sentence containing several rules.

### Maturity, stated plainly

`docs/JIMINY_CASCADING_FAILURE_FIX_REPORT.md` documents a real outage
in which five compounding failures silenced Jiminy guidance entirely.
It is fixed, and its existence is evidence of genuine operational
history rather than vaporware.

More pointed: Jiminy's own Grafana dashboard tracks guidance follow
rate against a **baseline of 13.25% and a target of 80%**. Outside hard
`/strict` blocks, the system's own instrumentation says its advice is
followed a minority of the time.

The installed binary is also two point releases behind the tap
(beta.1 installed, beta.3 served).

## 4. What this repo already does

Strong, and more than expected:

- `docs/FSD.md` (1,537 lines) with a dated changelog and a D-01..D-27
  decision table, plus 8 ADRs under `docs/adr/`.
- `docs/backlog.md` (1,866 lines) carrying root-cause narratives, often
  deeper than a typical ADR.
- `docs/in-flight.md` for mid-task state.
- CI enforcing proto drift, migration replay against real Postgres,
  design tokens, runtime versions, gitleaks, CodeQL, govulncheck.
- A rich queryable operational history in Postgres: `decision_log`,
  `jd_runs` with `phase_ms`, `eval_runs`, `llm_usage`.

Genuinely missing:

- Automatic capture of an assistant's working state across a
  compaction. `docs/in-flight.md` exists precisely because that gap is
  felt, and it is hand-maintained prose.
- Any enforcement that decisions get written down outside the JD
  pipeline. Nine of the ten UxTS frameworks in FSD §10 are spec-only.
- Aggregation across runs.

**The one thing CI structurally cannot do:** express "does this change
violate ADR-0002". Mechanical rules are well covered; semantic and
architectural ones rely on the assistant having read and remembered
`AGENTS.md` and the ADRs, which is exactly what a compaction destroys.
That is a real gap, and it is the honest case for a corpus-backed hook.

## 5. Recommendation

**Split the two questions.**

**For tracking complexity, MDEMG is the wrong tool and the right one is
cheap.** CI runs only `go vet` today. Missing and worth adding:
`golangci-lint` or `staticcheck` and `gocyclo` for Go, `knip` or
`ts-prune` for the web app. No new service, no daemon, no RAM.

Ahead of all of that, on the measured evidence: **the web app has no
tests at all.** 117 files, 36 pages, zero. That is the largest real
risk this investigation found, and no memory graph addresses it.

**For the guardrail, a bounded experiment is justified, but not
today.** The integration is genuinely cheap and well contained: hooks
live in `.claude/`, blocking is off by default, it fails open, and the
corpus it would need (ADRs, FSD, AGENTS.md) already exists. If tried,
the shape is:

1. Advisory mode only. Do not enable `/strict`, given the documented
   history of pipeline failures and a 13.25% follow rate.
2. Ingest `docs/adr/`, `docs/FSD.md` and `AGENTS.md` as the corpus.
3. Nothing enters `docker-compose.yml`, the Makefile or CI.
4. Upgrade to beta.3 first.

The reason to defer is not technical. There is an eval run to restart,
a deploy queue waiting, and a scheduler half-built. Adopting a second
always-on stateful stack from a pre-1.0 project maintained by the same
person maintaining this one is attention spent on tooling during
delivery.

**Caveats worth carrying forward.** Whether `/v1/jiminy/classify` is
LLM-backed or purely rule-based was not resolved. Two investigators
independently reported that a `jiminy-governance` skill appeared in
their tool lists instructing them to consult it before any action even
if the user never mentioned Jiminy; both declined to invoke it and
treated it as untrusted environment content, which was the right call,
and it is worth noting that it surfaced during an evaluation of MDEMG
itself. That behaviour deserves explicit review before anything is
enabled.
