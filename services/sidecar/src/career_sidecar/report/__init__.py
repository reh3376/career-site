"""The decision test's individual results report, rendered.

The api writes the report as a document: every figure worked out and
every sentence chosen (services/api/internal/dtreport). This package
draws it, twice: a participant's copy and an owner's copy, which adds
the blocks the document marks for the owner. No model is involved and
nothing is read from outside the request.
"""

from __future__ import annotations

import json
import threading
from dataclasses import dataclass
from functools import lru_cache

# The document contract this renderer draws. The api sends the version
# it wrote; a mismatch is refused rather than drawn wrongly.
SCHEMA = 1

# matplotlib's font cache and reportlab's font registry are process
# globals, and neither is written for concurrent first use. Reports are
# rare and take a second or two, so they are drawn one at a time.
_LOCK = threading.Lock()


@lru_cache(maxsize=1)
def _probe() -> tuple[bool, str]:
    """Whether the renderer can run here, and the engine string if so.

    Imported on first use rather than at module load, so a sidecar that
    never draws a report does not pay for matplotlib, and one that
    cannot import it still serves everything else.
    """
    try:
        import matplotlib
        import reportlab

        from career_sidecar.report import fonts, layout  # noqa: F401

        face = fonts.family().name
        return True, f"reportlab:{reportlab.Version}+matplotlib:{matplotlib.__version__}+{face}"
    except Exception as e:  # noqa: BLE001 - optional at import time, like typst
        return False, str(e)


def renderer_ready() -> bool:
    return _probe()[0]


def engine_name() -> str:
    ready, engine = _probe()
    return engine if ready else "unavailable"


@dataclass(frozen=True)
class ReportResult:
    participant_pdf: bytes
    owner_pdf: bytes
    participant_pages: int
    owner_pages: int
    engine: str


def render_report(document_json: str) -> ReportResult:
    ready, detail = _probe()
    if not ready:
        raise RuntimeError(f"report renderer is not available: {detail}")
    from career_sidecar.report import layout

    doc = json.loads(document_json)
    if not isinstance(doc, dict) or not isinstance(doc.get("blocks"), list) or not doc["blocks"]:
        raise ValueError("document has no blocks")
    if doc.get("schema") != SCHEMA:
        raise ValueError(
            f"document schema {doc.get('schema')!r} is not the one drawn here ({SCHEMA})"
        )
    with _LOCK:
        cache = layout.ChartCache()
        participant, p_pages = layout.build_pdf(doc, owner=False, cache=cache)
        owner, o_pages = layout.build_pdf(doc, owner=True, cache=cache)
    return ReportResult(
        participant_pdf=participant,
        owner_pdf=owner,
        participant_pages=p_pages,
        owner_pages=o_pages,
        engine=detail,
    )
