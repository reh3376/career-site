import io
import json

import pytest
from pypdf import PdfReader

from career_sidecar import render

SAMPLE = {
    "headline": "Principal Engineer, Industrial AI and Controls",
    "summary": "Thirty years across control rooms, plants and decision infrastructure.",
    "competencies": [
        {"text": "Rockwell ControlLogix and Ignition HMI standards", "sources": [1]},
        {"text": "MQTT / Unified Namespace data architecture", "sources": [2]},
        {"text": "*markup* should render literally #not as code", "sources": [3]},
    ],
    "experience": [
        {
            "role": "VP of Engineering & Technology",
            "organisation": "Whiskey House",
            "dates": "2022 to 2026",
            "bullets": [
                {"text": "Built the factory software stack in-house.", "sources": [4]},
                {"text": "Directed a greenfield 24/7 facility build.", "sources": [5]},
            ],
        }
    ],
    "education": [{"text": "B.S. Applied Mathematics", "sources": [6]}],
}


@pytest.mark.skipif(not render.renderer_ready(), reason="typst not installed")
def test_render_resume_produces_locked_pdf():
    res = render.render_resume(json.dumps(SAMPLE), "owner-secret")
    assert res.pdf.startswith(b"%PDF")
    assert res.pages >= 1
    assert res.engine.startswith("typst:")

    reader = PdfReader(io.BytesIO(res.pdf))
    assert reader.is_encrypted
    # Opens with the empty user password ...
    assert reader.decrypt("") > 0
    text = "".join(p.extract_text() for p in reader.pages)
    assert "Roger E. Henley II" in text
    assert "Whiskey House" in text
    # ... and editing is not among the granted permissions.
    perms = reader.user_access_permissions
    assert perms is not None
    from pypdf.constants import UserAccessPermissions as P

    assert P.PRINT in perms
    assert P.MODIFY not in perms


def test_render_requires_owner_password():
    if not render.renderer_ready():
        pytest.skip("typst not installed")
    with pytest.raises(ValueError):
        render.render_resume(json.dumps(SAMPLE), "")


def test_render_rejects_resume_without_experience():
    if not render.renderer_ready():
        pytest.skip("typst not installed")
    bad = dict(SAMPLE, experience=[])
    with pytest.raises(ValueError):
        render.render_resume(json.dumps(bad), "x")
