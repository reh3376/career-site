"""Résumé PDF rendering: Typst for layout, pypdf for the owner-password
lock. No model is involved; the input is the API's verified JSON.

The PDF opens without a password (empty user password) but editing,
form filling and page assembly require the owner password. Printing
and text extraction stay allowed so a recruiter can print or copy.
"""

from __future__ import annotations

import io
import json
import tempfile
from dataclasses import dataclass
from importlib import resources
from pathlib import Path

from pypdf import PdfReader, PdfWriter
from pypdf.constants import UserAccessPermissions

try:
    import typst

    _TYPST_VERSION = getattr(typst, "__version__", "unknown")
except Exception:  # noqa: BLE001 — renderer is optional at import time
    typst = None
    _TYPST_VERSION = ""


def renderer_ready() -> bool:
    return typst is not None


def engine_name() -> str:
    return f"typst:{_TYPST_VERSION}" if typst is not None else "unavailable"


@dataclass(frozen=True)
class RenderResult:
    pdf: bytes
    pages: int
    engine: str


_REQUIRED = ("headline", "summary", "competencies", "experience", "education")


def _normalise(resume: dict) -> dict:
    """Fill defaults so the template never indexes a missing key."""
    out = {k: resume.get(k) for k in _REQUIRED}
    out["headline"] = str(out["headline"] or "")
    out["summary"] = str(out["summary"] or "")
    out["competencies"] = [
        {"text": str(c.get("text", ""))} for c in (out["competencies"] or []) if c.get("text")
    ]
    exps = []
    for e in out["experience"] or []:
        bullets = [{"text": str(b.get("text", ""))} for b in e.get("bullets", []) if b.get("text")]
        if not e.get("role") or not bullets:
            continue
        exps.append(
            {
                "role": str(e.get("role", "")),
                "organisation": str(e.get("organisation", "") or ""),
                "dates": str(e.get("dates", "") or ""),
                "bullets": bullets,
            }
        )
    out["experience"] = exps
    out["education"] = [
        {"text": str(x.get("text", ""))} for x in (out["education"] or []) if x.get("text")
    ]
    return out


def render_resume(resume_json: str, owner_password: str) -> RenderResult:
    if typst is None:
        raise RuntimeError("typst is not available in this sidecar build")
    if not owner_password:
        raise ValueError("owner_password is required")
    resume = _normalise(json.loads(resume_json))
    if not resume["experience"]:
        raise ValueError("resume has no experience entries to render")

    template = resources.files("career_sidecar").joinpath("templates/resume.typ").read_text()
    with tempfile.TemporaryDirectory(prefix="resume-") as tmp:
        root = Path(tmp)
        (root / "data.json").write_text(json.dumps(resume), encoding="utf-8")
        (root / "resume.typ").write_text(template, encoding="utf-8")
        raw = typst.compile(root / "resume.typ", root=root)

    return RenderResult(
        pdf=lock_pdf(raw, owner_password), pages=_page_count(raw), engine=engine_name()
    )


def lock_pdf(pdf: bytes, owner_password: str) -> bytes:
    """Apply AES-256 with an empty user password: opens freely, edits
    need the owner password, printing and copying remain allowed."""
    writer = PdfWriter(clone_from=io.BytesIO(pdf))
    writer.encrypt(
        user_password="",
        owner_password=owner_password,
        algorithm="AES-256",
        permissions_flag=UserAccessPermissions.PRINT | UserAccessPermissions.EXTRACT,
    )
    out = io.BytesIO()
    writer.write(out)
    return out.getvalue()


def _page_count(pdf: bytes) -> int:
    return len(PdfReader(io.BytesIO(pdf)).pages)
