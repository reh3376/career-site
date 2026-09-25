#!/usr/bin/env python3
"""Extract plain text from the private résumé and cover-letter archive.

The corpus pipeline (deploy/corpus-sync.sh) only ships .md and .txt.
Roger's tailored résumés are .docx and .pdf, so they are invisible to it
and to anything that wants to read them as data: the JD-to-résumé pairs
for the tailoring evaluation, and later the LoRA training set.

Nothing new is installed to do this. A .docx is a zip whose
word/document.xml holds the text, which zipfile and ElementTree handle,
and .pdf is read with pypdf, already a sidecar dependency. Avoiding a
dependency matters here because these files are personal: the fewer
third-party libraries that touch them, the better.

Output goes under docs/personal, which is gitignored in full, so the
extracted text is no more exposed than the originals.

Usage:
    services/sidecar/.venv/bin/python scripts/extract_docs.py
    services/sidecar/.venv/bin/python scripts/extract_docs.py --src docs/personal/resume
"""

from __future__ import annotations

import argparse
import sys
import xml.etree.ElementTree as ET
import zipfile
from pathlib import Path

# WordprocessingML. Paragraphs are w:p, text runs inside them are w:t.
W = "{http://schemas.openxmlformats.org/wordprocessingml/2006/main}"


def from_docx(path: Path) -> str:
    """Text of a .docx, one line per paragraph.

    Runs are concatenated without a separator because Word splits a
    single word across runs whenever formatting changes mid-word; a
    space between them would corrupt the text rather than preserve it.
    """
    with zipfile.ZipFile(path) as z:
        root = ET.fromstring(z.read("word/document.xml"))
    lines = []
    for para in root.iter(W + "p"):
        lines.append("".join(node.text or "" for node in para.iter(W + "t")))
    return "\n".join(lines)


def from_pdf(path: Path) -> str:
    """Text of a .pdf, one blank line between pages.

    Site-generated résumés carry an owner password, which restricts
    editing rather than reading, so an empty user password opens them.
    """
    from pypdf import PdfReader

    reader = PdfReader(str(path))
    if reader.is_encrypted:
        reader.decrypt("")
    return "\n\n".join(page.extract_text() or "" for page in reader.pages)


def from_text(path: Path) -> str:
    return path.read_text(encoding="utf-8", errors="replace")


READERS = {".docx": from_docx, ".pdf": from_pdf, ".md": from_text, ".txt": from_text}


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument(
        "--src",
        type=Path,
        action="append",
        help="directory to read (repeatable). Default: docs/personal/resume and docs/personal/cover",
    )
    ap.add_argument(
        "--out",
        type=Path,
        default=Path("docs/personal/extracted"),
        help="where the .txt files are written (default: docs/personal/extracted)",
    )
    args = ap.parse_args()

    sources = args.src or [Path("docs/personal/resume"), Path("docs/personal/cover")]
    written = skipped = failed = 0

    for src in sources:
        if not src.is_dir():
            print(f"no such directory, skipping: {src}", file=sys.stderr)
            continue
        # Mirror the source directory name so resume/ and cover/ stay
        # apart; they are different kinds of document and a later step
        # reads them differently.
        out_dir = args.out / src.name
        out_dir.mkdir(parents=True, exist_ok=True)

        for path in sorted(src.iterdir()):
            if not path.is_file() or path.name.startswith("."):
                continue
            reader = READERS.get(path.suffix.lower())
            if reader is None:
                print(f"skip  {path.name}  (no reader for {path.suffix})")
                skipped += 1
                continue
            try:
                text = reader(path)
            except Exception as exc:  # noqa: BLE001 - one bad file must not stop the rest
                print(f"FAIL  {path.name}: {exc}", file=sys.stderr)
                failed += 1
                continue
            # Keep the original extension in the stem so
            # reh-resume-Motional.docx and .pdf do not overwrite
            # each other; they are different renderings and the
            # comparison between them is sometimes the point.
            target = out_dir / (path.name + ".txt")
            target.write_text(text, encoding="utf-8")
            print(f"ok    {target.relative_to(args.out)}  ({len(text)} chars)")
            written += 1

    print(f"\n{written} written, {skipped} skipped, {failed} failed -> {args.out}")
    return 1 if failed else 0


if __name__ == "__main__":
    raise SystemExit(main())
