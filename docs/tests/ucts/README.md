# UCTS, Universal Conversion Test Specification

Document conversions, checked rather than eyeballed.

`.md` to `.pdf`, `.docx` to `.md`, `.pdf` to `.md`, `.md` to `.docx`:
repeatable transforms whose correctness is testable. UCTS is the
career-site instance of Roger's UxTS pattern (see
`docs/personal/dev-docs/UXTS_DEVELOPER_GUIDE.md`, §13.3), the letter C
being free in the framework matrix.

## Why this exists

Converting the résumé archive on 2026-09-25 produced four failures that
every tool involved reported as success:

| What happened | How it looked at the time |
|---|---|
| The site's résumé renderer dropped the contact block, having no field for one | A clean PDF with no phone number or email |
| pandoc reproduced Word's split bold runs as `****` | 57 literal asterisks reached the corpus and retrieval matched against them for four days |
| pandoc writes an en dash as `--`, so a single-dash pattern matched nothing | "4 files corrected" when the true number was 32 |
| A template change added a page to three résumés | Three-page résumés their authors had fitted onto two |

None threw. None warned. Each was found by a person reading output.
Every spec in `specs/` names the incident it exists to prevent, in its
`metadata.regression_of` field; a spec that cannot name one is usually
testing that the code does what the code does.

## Layout

```
docs/tests/ucts/
├── schema/ucts.schema.json     what a legal spec looks like
├── specs/*.ucts.json           canonical, CI-gated, hashed
├── drafts/*.ucts.json          in progress, excluded from runs
├── fixtures/                   inputs, hashed by the specs that use them
├── runners/ucts_runner.py      the only layer with dependencies
└── README.md
```

Shared infrastructure sits one level up at `docs/tests/uxts_report.py`
and `uxts_runner_core.py`, vendored from `~/mdemg/docs/tests/` so the
report format and the hashing are identical across frameworks.

## Running it

```bash
make ucts                                  # what CI runs
services/sidecar/.venv/bin/python docs/tests/ucts/runners/ucts_runner.py validate-all
services/sidecar/.venv/bin/python docs/tests/ucts/runners/ucts_runner.py validate --spec docs/tests/ucts/specs/<name>.ucts.json
services/sidecar/.venv/bin/python docs/tests/ucts/runners/ucts_runner.py verify-hashes
services/sidecar/.venv/bin/python docs/tests/ucts/runners/ucts_runner.py add-hashes
```

No new dependencies: pandoc was already installed, `typst` and `pypdf`
are sidecar requirements.

## Writing a spec

```json
{
  "ucts_version": "1.0.0",
  "conversion": { "from": "md", "to": "pdf", "profile": "resume" },
  "fixture":    { "path": "../fixtures/your-fixture.md" },
  "expected":   { "must_contain": ["..."], "max_pages": 2 },
  "config":     { "timeout_ms": 120000 },
  "metadata":   { "description": "...", "regression_of": "..." }
}
```

Then run `add-hashes`, which fills in the fixture's SHA-256 and the
spec's own. Commit both.

`profile: "resume"` calls `scripts/render_resume_pdf.py` directly
rather than reassembling its pandoc invocation. That is deliberate: the
first version of this runner rebuilt the pipeline inline and passed a
spec that the production path failed, which is the exact failure mode a
test suite is supposed to make impossible.

## The guarantees, and the proof they hold

Four properties, each verified by deliberately breaking it:

**An unimplemented field is a hard failure, never a silent skip.** The
runner carries an allow-set per nesting level. A spec asserting
something nobody checks would report green and prove nothing, so an
unknown key fails with `PARITY FAILURE`.

**An unreviewed edit is detectable.** Each spec carries a SHA-256 of
itself with that field removed, and of its fixture's bytes. Change
either and the run fails on integrity before it converts anything.

**Zero assertions is a failure, not a pass.** `build_result` rewrites a
pass with no assertions evaluated into a fail with `0/0 false pass`. An
empty `expected` block cannot go green.

**`verify-hashes` reports independently of `validate-all`,** so drift is
visible without running conversions.

## Adding a conversion

The runner implements `md`, `docx` and `pdf` as inputs and `md`, `pdf`
and `docx` as outputs, plus `round_trip` for converting forward and
back. A new format needs a branch in `convert()`, its enum value in the
schema, and its key added to the relevant allow-set in the runner, or
the parity check will reject every spec that uses it. That coupling is
intentional: the schema and the runner cannot drift apart quietly.
