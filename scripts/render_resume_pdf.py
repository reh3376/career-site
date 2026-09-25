#!/usr/bin/env python3
"""Render the canonical Markdown résumés to PDF.

Markdown is the source, PDF is the artifact. pandoc converts to Typst
through scripts/templates/resume-pandoc.typ, which carries the same
typography as the site's own renderer, and the `typst` Python package
compiles it. Neither is a new dependency: pandoc was already installed
and typst is a sidecar requirement.

Output goes to a separate directory rather than over the originals.
The hand-made .docx and .pdf files are the only record of the layout
their author chose, and overwriting them to find out whether the
render is good enough would destroy the thing the comparison is
against.

The page count of each render is reported beside its original, because
that is the number that matters: a résumé that runs to three pages
when its author wrote two has been changed, not converted.

Optional edit lock: set RESUME_PDF_OWNER_PASSWORD and every PDF is
written with an owner password, as the site's generated résumés are.
Off by default, because a password the author does not have to hand
later is a trap on files he sends himself.

Usage:
    services/sidecar/.venv/bin/python scripts/render_resume_pdf.py
    services/sidecar/.venv/bin/python scripts/render_resume_pdf.py --only reh-resume-Cognite
"""

from __future__ import annotations

import argparse
import os
import subprocess
import sys
import tempfile
from pathlib import Path

TEMPLATE = Path("scripts/templates/resume-pandoc.typ")


def page_count(path: Path) -> int | None:
    try:
        from pypdf import PdfReader

        reader = PdfReader(str(path))
        if reader.is_encrypted:
            reader.decrypt("")
        return len(reader.pages)
    except Exception:  # noqa: BLE001 - a missing or unreadable original is not fatal
        return None


def lock(path: Path, owner_password: str) -> None:
    """Apply an owner password: editing restricted, reading is not.

    Printing and text extraction stay allowed, so a recruiter can still
    print the file and an applicant tracking system can still parse it.
    A résumé nobody's parser can read is worse than one that can be
    edited.
    """
    from pypdf import PdfReader, PdfWriter
    from pypdf.constants import UserAccessPermissions

    reader = PdfReader(str(path))
    writer = PdfWriter()
    for page in reader.pages:
        writer.add_page(page)
    writer.encrypt(
        user_password="",
        owner_password=owner_password,
        permissions_flag=(
            UserAccessPermissions.PRINT
            | UserAccessPermissions.PRINT_TO_REPRESENTATION
            | UserAccessPermissions.EXTRACT_TEXT_AND_GRAPHICS
        ),
    )
    with path.open("wb") as fh:
        writer.write(fh)


# Progressively tighter typography. The first entry is the template's
# own default. A résumé that will not fit two pages at one level is
# retried at the next, so a long document is set tighter rather than
# every document being set tight enough for the longest one.
#
# The ladder stops at 9pt. Below that a résumé is not too loosely set,
# it is too long, and the answer is to cut it rather than to shrink it
# until a recruiter cannot read it.
DENSITIES: list[tuple[str, dict[str, str]]] = [
    ("default", {}),
    ("tight", {"fontsize": "9.5pt", "leading": "0.46em", "listspacing": "0.3em",
               "marginx": "0.65in", "marginy": "0.5in"}),
    ("tighter", {"fontsize": "9pt", "leading": "0.44em", "listspacing": "0.28em",
                 "marginx": "0.6in", "marginy": "0.45in"}),
]


def sanitise(md: Path, tmp: Path) -> Path:
    """Repair conversion artifacts before rendering.

    The bold-run repair used to live only in the .docx to .md converter,
    on the assumption that Markdown in hand is already clean. The master
    résumé disproved that: hand-written .md, exported from Word at some
    point in its history, carrying 57 `****` sequences that rendered as
    literal asterisks. Anything that reaches the renderer gets the same
    treatment, whatever produced it.
    """
    import sys as _sys

    _sys.path.insert(0, str(Path(__file__).resolve().parent))
    from resume_to_md import merge_bold_runs

    text, merged = merge_bold_runs(md.read_text(encoding="utf-8"))
    if not merged:
        return md
    clean = tmp / (md.stem + ".clean.md")
    clean.write_text(text, encoding="utf-8")
    return clean


def render(md: Path, out_pdf: Path, tmp: Path, density: dict[str, str]) -> None:
    md = sanitise(md, tmp)
    typ = tmp / (md.stem + ".typ")
    cmd = ["pandoc", str(md), "-t", "typst", f"--template={TEMPLATE}", "--wrap=none"]
    for key, value in density.items():
        cmd += ["-V", f"{key}={value}"]
    cmd += ["-o", str(typ)]
    subprocess.run(cmd, check=True, capture_output=True)

    import typst

    typst.compile(str(typ), output=str(out_pdf))


def render_to_fit(md: Path, out_pdf: Path, tmp: Path, max_pages: int) -> tuple[int, str]:
    """Render at the loosest density that still fits. Returns pages and level."""
    pages, level = 0, DENSITIES[-1][0]
    for name, density in DENSITIES:
        render(md, out_pdf, tmp, density)
        pages = page_count(out_pdf) or 0
        level = name
        if pages <= max_pages:
            break
    return pages, level


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--src", type=Path, default=Path("docs/personal/resume"))
    ap.add_argument("--out", type=Path, default=Path("docs/personal/resume-pdf"))
    ap.add_argument("--only", help="render just this stem, for iterating on the template")
    ap.add_argument(
        "--max-pages",
        type=int,
        default=2,
        help="the page budget each résumé must fit. Two is the standing rule; the master and the long-form variants are expected to exceed it.",
    )
    args = ap.parse_args()

    if not TEMPLATE.exists():
        print(f"template missing: {TEMPLATE}", file=sys.stderr)
        return 2

    owner_password = os.environ.get("RESUME_PDF_OWNER_PASSWORD", "")
    args.out.mkdir(parents=True, exist_ok=True)

    sources = sorted(args.src.glob("*.md"))
    if args.only:
        sources = [p for p in sources if p.stem == args.only]
        if not sources:
            print(f"no such résumé: {args.only}", file=sys.stderr)
            return 2

    ok = failed = grew = over = 0
    rows: list[tuple[str, int | None, int | None, str]] = []

    with tempfile.TemporaryDirectory() as td:
        tmp = Path(td)
        for md in sources:
            out_pdf = args.out / (md.stem + ".pdf")
            try:
                new, level = render_to_fit(md, out_pdf, tmp, args.max_pages)
            except subprocess.CalledProcessError as exc:
                print(f"FAIL  {md.name}: pandoc: {exc.stderr.decode()[:160]}", file=sys.stderr)
                failed += 1
                continue
            except Exception as exc:  # noqa: BLE001
                print(f"FAIL  {md.name}: {exc}", file=sys.stderr)
                failed += 1
                continue

            if owner_password:
                lock(out_pdf, owner_password)

            old = page_count(args.src / (md.stem + ".pdf"))
            rows.append((md.stem, old, new, level))
            if old is not None and new > old:
                grew += 1
            if new > args.max_pages:
                over += 1
            ok += 1

    width = max((len(r[0]) for r in rows), default=20)
    print(f"{'résumé':<{width}} {'was':>5} {'now':>5}  density")
    for stem, old, new, level in rows:
        flags = []
        if old is not None and new > old:
            flags.append("longer than original")
        if new > args.max_pages:
            flags.append(f"OVER {args.max_pages}-PAGE RULE")
        note = ("  " + ", ".join(flags)) if flags else ""
        shown = level if level != "default" else ""
        print(f"{stem:<{width}} {str(old if old is not None else '-'):>5} {new:>5}  {shown:<8}{note}")

    print(f"\n{ok} rendered, {failed} failed, {grew} longer than the original, "
          f"{over} still over {args.max_pages} pages")
    print(f"written to {args.out}")
    if owner_password:
        print("edit-locked with RESUME_PDF_OWNER_PASSWORD")
    else:
        print("not edit-locked (set RESUME_PDF_OWNER_PASSWORD to lock)")
    return 1 if failed else 0


if __name__ == "__main__":
    raise SystemExit(main())
