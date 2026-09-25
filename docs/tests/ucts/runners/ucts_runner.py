#!/usr/bin/env python3
"""UCTS runner: execute document conversions and check what came out.

The only layer with dependencies, per the UxTS four-layer split. The
schema says what a legal spec looks like, the specs are pure data, this
runs them, and the Makefile gates CI on the result.

Conversions are repeatable transforms, so their correctness is
checkable rather than eyeballed. Every assertion this runner implements
exists because the corresponding failure happened while converting
Roger's résumé archive on 2026-09-25, and in each case the conversion
reported success:

  * The site's own résumé renderer produced a clean PDF with no phone
    number or email, because its JSON schema has no field for a contact
    block. Nothing errored.
  * pandoc faithfully reproduced Word's split bold runs as `****`,
    which Markdown renders as literal asterisks. 57 of them survived
    into the corpus and were being matched against by retrieval.
  * pandoc writes an en dash as `--`, so a correction pattern matching
    a single dash silently changed nothing in 22 files.
  * A template change put three résumés onto a third page their authors
    had fitted onto two.

Usage:
    ucts_runner.py validate      --spec <path>
    ucts_runner.py validate-all  --spec-dir <dir> [--report <path>]
    ucts_runner.py add-hashes    --spec-dir <dir>
    ucts_runner.py verify-hashes --spec-dir <dir>

Run from the repository root.
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
import tempfile
import time
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[2]))
from uxts_report import build_report, build_result, print_summary, save_report  # noqa: E402
from uxts_runner_core import sha256_file, sha256_spec_without_field  # noqa: E402

FRAMEWORK = "ucts"
FRAMEWORK_VERSION = "1.0.0"
SUPPORTED_MAJOR = 1

# docs/tests/ucts/runners/ucts_runner.py -> four levels up is the repo
# root. parents[3] lands on docs/, which resolved to a template path
# that does not exist and surfaced as a bare pandoc exit code 97.
REPO_ROOT = Path(__file__).resolve().parents[4]
RESUME_TEMPLATE = REPO_ROOT / "scripts" / "templates" / "resume-pandoc.typ"

# Parity: allow-sets per nesting level. A spec key this runner does not
# implement is a hard failure, never a warning and never a silent skip.
# The alternative is a spec that asserts something nobody checks, which
# reports green and proves nothing.
_SPEC_KEYS = {"ucts_version", "conversion", "fixture", "expected", "config", "metadata"}
_CONVERSION_KEYS = {"from", "to", "profile", "round_trip"}
_FIXTURE_KEYS = {"path", "sha256"}
_EXPECTED_KEYS = {
    "must_contain",
    "must_not_contain",
    "must_match",
    "max_pages",
    "min_chars",
    "retains_fraction",
}
_CONFIG_KEYS = {"timeout_ms", "sha256"}
_METADATA_KEYS = {"description", "tags", "regression_of"}


def check_parity(spec: dict) -> list[str]:
    errors = []
    for key in spec:
        if key not in _SPEC_KEYS:
            errors.append(f"PARITY FAILURE: unimplemented spec key: {key}")
    for section, allowed in (
        ("conversion", _CONVERSION_KEYS),
        ("fixture", _FIXTURE_KEYS),
        ("expected", _EXPECTED_KEYS),
        ("config", _CONFIG_KEYS),
        ("metadata", _METADATA_KEYS),
    ):
        for key in spec.get(section, {}) or {}:
            if key not in allowed:
                errors.append(f"PARITY FAILURE: unimplemented {section} key: {key}")
    major = int(str(spec.get("ucts_version", "0.0.0")).split(".")[0])
    if major != SUPPORTED_MAJOR:
        errors.append(
            f"PARITY FAILURE: ucts_version major {major} not implemented (runner supports {SUPPORTED_MAJOR})"
        )
    return errors


# --------------------------------------------------------------------
# Conversions. Each returns the text of the output, plus a page count
# where the output is a PDF.
# --------------------------------------------------------------------


def _pandoc(args: list[str], timeout_ms: int) -> None:
    subprocess.run(["pandoc", *args], check=True, capture_output=True, timeout=timeout_ms / 1000)


def _pdf_text_and_pages(path: Path) -> tuple[str, int]:
    from pypdf import PdfReader

    reader = PdfReader(str(path))
    if reader.is_encrypted:
        reader.decrypt("")
    return "\n".join(p.extract_text() or "" for p in reader.pages), len(reader.pages)


def convert(fixture: Path, frm: str, to: str, profile: str, tmp: Path, timeout_ms: int) -> tuple[str, int | None]:
    if frm == "pdf" and to == "md":
        text, _ = _pdf_text_and_pages(fixture)
        return text, None

    if to == "pdf":
        pdf = tmp / "out.pdf"
        if profile == "resume":
            # Call the real renderer rather than reassembling its
            # pandoc invocation here. A runner that reimplements the
            # pipeline tests its own copy of the pipeline, which is
            # how the first version of this file passed a spec the
            # production path would have failed.
            sys.path.insert(0, str(REPO_ROOT / "scripts"))
            from render_resume_pdf import render as render_resume

            render_resume(fixture, pdf, tmp, {})
            return _pdf_text_and_pages(pdf)

        typ = tmp / "out.typ"
        _pandoc([str(fixture), "-t", "typst", "--wrap=none", "-o", str(typ)], timeout_ms)
        import typst

        typst.compile(str(typ), output=str(pdf))
        return _pdf_text_and_pages(pdf)

    out = tmp / f"out.{to}"
    _pandoc([str(fixture), "-t", "markdown" if to == "md" else to, "--wrap=none", "-o", str(out)], timeout_ms)
    if to == "docx":
        # A .docx is not text; read it back through pandoc so the
        # assertions have something to run against. Asserting on the
        # zip's bytes would test the compressor, not the conversion.
        back = tmp / "back.md"
        _pandoc([str(out), "-t", "markdown", "--wrap=none", "-o", str(back)], timeout_ms)
        return back.read_text(encoding="utf-8"), None
    return out.read_text(encoding="utf-8"), None


def round_trip(fixture: Path, frm: str, to: str, profile: str, tmp: Path, timeout_ms: int) -> tuple[str, int | None]:
    """Convert forward and back. Loss that a one-way conversion hides
    shows up here, because the output is comparable with the input."""
    mid = tmp / f"mid.{to}"
    _pandoc([str(fixture), "-t", "markdown" if to == "md" else to, "--wrap=none", "-o", str(mid)], timeout_ms)
    back = tmp / f"back.{frm}"
    _pandoc([str(mid), "-t", "markdown" if frm == "md" else frm, "--wrap=none", "-o", str(back)], timeout_ms)
    return back.read_text(encoding="utf-8"), None


# --------------------------------------------------------------------
# Assertions
# --------------------------------------------------------------------


def evaluate(expected: dict, text: str, pages: int | None, source_chars: int) -> tuple[int, int, list[str]]:
    evaluated = passed = 0
    failures: list[str] = []

    for needle in expected.get("must_contain", []):
        evaluated += 1
        if needle in text:
            passed += 1
        else:
            failures.append(f"must_contain: {needle!r} is not in the output")

    for needle in expected.get("must_not_contain", []):
        evaluated += 1
        if needle not in text:
            passed += 1
        else:
            failures.append(f"must_not_contain: {needle!r} survived the conversion")

    for pattern in expected.get("must_match", []):
        evaluated += 1
        if re.search(pattern, text):
            passed += 1
        else:
            failures.append(f"must_match: /{pattern}/ found nothing")

    if "max_pages" in expected:
        evaluated += 1
        if pages is None:
            failures.append("max_pages: the output is not a PDF, so it has no page count")
        elif pages <= expected["max_pages"]:
            passed += 1
        else:
            failures.append(f"max_pages: {pages} pages, budget is {expected['max_pages']}")

    if "min_chars" in expected:
        evaluated += 1
        if len(text) >= expected["min_chars"]:
            passed += 1
        else:
            failures.append(f"min_chars: {len(text)} characters, floor is {expected['min_chars']}")

    if "retains_fraction" in expected:
        evaluated += 1
        share = (len(text) / source_chars) if source_chars else 0.0
        if share >= expected["retains_fraction"]:
            passed += 1
        else:
            failures.append(
                f"retains_fraction: kept {share:.3f} of the input, floor is {expected['retains_fraction']}"
            )

    return evaluated, passed, failures


# --------------------------------------------------------------------
# Spec execution
# --------------------------------------------------------------------


def run_spec(spec_path: Path) -> dict:
    started = time.perf_counter()
    try:
        spec = json.loads(spec_path.read_text(encoding="utf-8"))
    except Exception as exc:  # noqa: BLE001
        return build_result(spec_path, "error", (time.perf_counter() - started) * 1000, error=str(exc))

    parity = check_parity(spec)
    if parity:
        return build_result(
            spec_path, "fail", (time.perf_counter() - started) * 1000, failures=parity
        )

    # Integrity: the spec's own hash, then the fixture's.
    mismatches = []
    hash_verified: bool | None = None
    declared = (spec.get("config") or {}).get("sha256")
    if declared:
        actual = sha256_spec_without_field(spec, ("config", "sha256"))
        hash_verified = actual == declared
        if not hash_verified:
            mismatches.append(f"spec hash {actual[:12]} does not match declared {declared[:12]}")

    fixture = (spec_path.parent / spec["fixture"]["path"]).resolve()
    if not fixture.exists():
        return build_result(
            spec_path, "error", (time.perf_counter() - started) * 1000,
            error=f"fixture not found: {fixture}",
        )
    declared_fixture = spec["fixture"].get("sha256")
    if declared_fixture:
        actual = sha256_file(fixture)
        if actual != declared_fixture:
            hash_verified = False
            mismatches.append(f"fixture hash {actual[:12]} does not match declared {declared_fixture[:12]}")

    if mismatches:
        return build_result(
            spec_path, "fail", (time.perf_counter() - started) * 1000,
            hash_verified=False, hash_mismatches=mismatches,
            failures=["integrity: the spec or its fixture changed since it was hashed"],
        )

    conv = spec["conversion"]
    timeout_ms = (spec.get("config") or {}).get("timeout_ms", 120000)
    try:
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            fn = round_trip if conv.get("round_trip") else convert
            text, pages = fn(fixture, conv["from"], conv["to"], conv.get("profile", "plain"), tmp, timeout_ms)
    except subprocess.TimeoutExpired:
        return build_result(
            spec_path, "error", (time.perf_counter() - started) * 1000,
            hash_verified=hash_verified, error=f"conversion exceeded {timeout_ms} ms",
        )
    except Exception as exc:  # noqa: BLE001
        return build_result(
            spec_path, "error", (time.perf_counter() - started) * 1000,
            hash_verified=hash_verified, error=f"{type(exc).__name__}: {exc}",
        )

    source_chars = len(fixture.read_bytes()) if conv["from"] != "md" else len(fixture.read_text(encoding="utf-8"))
    evaluated, passed, failures = evaluate(spec["expected"], text, pages, source_chars)

    return build_result(
        spec_path,
        "pass" if not failures else "fail",
        (time.perf_counter() - started) * 1000,
        hash_verified=hash_verified,
        assertions_evaluated=evaluated,
        assertions_passed=passed,
        failures=failures,
    )


def discover(spec_dir: Path) -> list[Path]:
    return sorted(spec_dir.glob("*.ucts.json"))


# --------------------------------------------------------------------
# Commands
# --------------------------------------------------------------------


def cmd_validate(args: argparse.Namespace) -> int:
    result = run_spec(args.spec)
    report = build_report(FRAMEWORK, FRAMEWORK_VERSION, [result])
    print_summary(report)
    for f in result["failures"]:
        print(f"  {f}")
    if result["error"]:
        print(f"  error: {result['error']}")
    return 0 if result["status"] == "pass" else 1


def cmd_validate_all(args: argparse.Namespace) -> int:
    specs = discover(args.spec_dir)
    if not specs:
        print(f"no *.ucts.json under {args.spec_dir}", file=sys.stderr)
        return 2
    results = [run_spec(p) for p in specs]
    report = build_report(FRAMEWORK, FRAMEWORK_VERSION, results)
    print_summary(report)
    for r in results:
        if r["status"] != "pass":
            print(f"\n{Path(r['spec_path']).name}: {r['status']}")
            for f in r["failures"]:
                print(f"    {f}")
            if r["error"]:
                print(f"    error: {r['error']}")
    if args.report:
        save_report(report, args.report)
        print(f"\nreport written to {args.report}")
    return 0 if report["summary"]["failed"] == 0 and report["summary"]["errors"] == 0 else 1


def cmd_add_hashes(args: argparse.Namespace) -> int:
    for path in discover(args.spec_dir):
        spec = json.loads(path.read_text(encoding="utf-8"))
        fixture = (path.parent / spec["fixture"]["path"]).resolve()
        if not fixture.exists():
            print(f"skip {path.name}: fixture missing", file=sys.stderr)
            continue
        spec["fixture"]["sha256"] = sha256_file(fixture)
        spec.setdefault("config", {})
        spec["config"].pop("sha256", None)
        spec["config"]["sha256"] = sha256_spec_without_field(spec, ("config", "sha256"))
        path.write_text(json.dumps(spec, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
        print(f"hashed {path.name}")
    return 0


def cmd_verify_hashes(args: argparse.Namespace) -> int:
    bad = 0
    for path in discover(args.spec_dir):
        spec = json.loads(path.read_text(encoding="utf-8"))
        declared = (spec.get("config") or {}).get("sha256")
        if not declared:
            print(f"UNHASHED {path.name}")
            bad += 1
            continue
        actual = sha256_spec_without_field(spec, ("config", "sha256"))
        if actual != declared:
            print(f"MISMATCH {path.name}: {actual[:12]} != {declared[:12]}")
            bad += 1
            continue
        fixture = (path.parent / spec["fixture"]["path"]).resolve()
        if spec["fixture"].get("sha256") and sha256_file(fixture) != spec["fixture"]["sha256"]:
            print(f"FIXTURE CHANGED {path.name}")
            bad += 1
            continue
        print(f"ok {path.name}")
    return 1 if bad else 0


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = ap.add_subparsers(dest="command", required=True)

    v = sub.add_parser("validate", help="run one spec")
    v.add_argument("--spec", type=Path, required=True)
    v.set_defaults(fn=cmd_validate)

    va = sub.add_parser("validate-all", help="run every spec in a directory")
    va.add_argument("--spec-dir", type=Path, default=Path("docs/tests/ucts/specs"))
    va.add_argument("--report", type=Path, help="write the canonical JSON report here")
    va.set_defaults(fn=cmd_validate_all)

    ah = sub.add_parser("add-hashes", help="write spec and fixture hashes")
    ah.add_argument("--spec-dir", type=Path, default=Path("docs/tests/ucts/specs"))
    ah.set_defaults(fn=cmd_add_hashes)

    vh = sub.add_parser("verify-hashes", help="check spec and fixture hashes")
    vh.add_argument("--spec-dir", type=Path, default=Path("docs/tests/ucts/specs"))
    vh.set_defaults(fn=cmd_verify_hashes)

    args = ap.parse_args()
    return args.fn(args)


if __name__ == "__main__":
    raise SystemExit(main())
