import io
import json

import pytest
from pypdf import PdfReader

from career_sidecar import report
from career_sidecar.report import charts

pytestmark = pytest.mark.skipif(not report.renderer_ready(), reason="report renderer not installed")

LEVELS = [
    {
        "level": 10,
        "label": "Guessing",
        "n": 4,
        "right": 1,
        "share": 14.3,
        "acc": 25.0,
        "others_share": 19.6,
        "others_acc": 26.0,
    },
    {
        "level": 30,
        "label": "Doubtful",
        "n": 2,
        "right": 1,
        "share": 7.1,
        "acc": 50.0,
        "others_share": 5.0,
        "others_acc": 35.0,
    },
    {
        "level": 50,
        "label": "Unsure",
        "n": 15,
        "right": 6,
        "share": 53.6,
        "acc": 40.0,
        "others_share": 15.0,
        "others_acc": 23.0,
    },
    {
        "level": 70,
        "label": "Fairly sure",
        "n": 7,
        "right": 4,
        "share": 25.0,
        "acc": 57.1,
        "others_share": 26.0,
        "others_acc": 39.0,
    },
    {
        "level": 90,
        "label": "Certain",
        "n": 0,
        "right": 0,
        "share": 0.0,
        "acc": None,
        "others_share": 34.0,
        "others_acc": 56.0,
    },
]

BLOCKS = [
    {
        "label": f"Block {i}\n3 digits",
        "correct": 2,
        "lure": 2,
        "other": 1,
        "expired": 1,
        "conf": 50.0 + i,
        "others_correct": 2.1,
        "others_conf": 61.0,
    }
    for i in range(1, 6)
]

CHARTS = {
    "standing": {
        "legend": "The 15 other first-time participants",
        "rows": [
            {
                "title": "Score",
                "unit": "questions correct, of 30",
                "line": "You 12  ·  their median 10",
                "placing": "5th highest of 16",
                "mine": 12,
                "others": [3, 4, 7, 8, 9, 10, 10, 10, 10, 11, 13, 13, 14, 24, 9],
                "median": 10,
                "lo": 0,
                "hi": 30,
                "ticks": [0, 10, 20, 30],
            },
            {
                "title": "Confidence minus accuracy",
                "unit": "points",
                "line": "You +8  ·  their median +31",
                "placing": "3rd closest to zero of 16",
                "mine": 7.9,
                "others": [-17, -5, 9, 12, 31, 40, 66],
                "median": 31,
                "lo": -25,
                "hi": 75,
                "ticks": [-25, 0, 25, 50, 75],
            },
        ],
    },
    "blocks": {"compare": True, "blocks": BLOCKS},
    "confidence_use": {"compare": True, "levels": LEVELS},
    "calibration": {"compare": True, "levels": LEVELS},
    "scatter": {
        "you": {"conf": 48, "acc": 40},
        "others": [{"conf": 72, "acc": 33}, {"conf": 14, "acc": 33}],
    },
    "families": {
        "compare": True,
        "rows": [
            {"label": "Syllogism", "correct": 6, "asked": 10, "acc": 60.0, "others_acc": 47.0},
            {"label": "Base rate", "correct": 0, "asked": 5, "acc": 0.0, "others_acc": 23.0},
        ],
    },
    "time_by_outcome": {
        "limit_s": 25,
        "mark_s": 10,
        "rows": [
            {
                "outcome": "correct",
                "label": "Correct",
                "detail": "3 answers, average 12.0 s",
                "times": [7.0, 12.0, 17.0],
            },
            {
                "outcome": "lure",
                "label": "Intuitive trap",
                "detail": "2 answers, average 8.4 s",
                "times": [8.0, 8.7],
            },
            {"outcome": "other", "label": "Other wrong answer", "detail": "none", "times": []},
        ],
    },
    "timeline": {
        "limit_s": 25,
        "others_typical": 14.1,
        "points": [
            {
                "position": i,
                "block_no": (i - 1) // 6 + 1,
                "outcome": ["correct", "lure", "other", "expired"][i % 4],
                "seconds": None if i % 4 == 3 else 6.0 + i % 17,
            }
            for i in range(1, 31)
        ],
    },
}

KEY_TEXT = "QUESTION-TEXT-ONLY-THE-OWNER-SEES"


def chart(kind: str, **extra) -> dict:
    return {"type": "chart", "chart": {"kind": kind, "data": CHARTS[kind]}, **extra}


def document() -> dict:
    half = {"kind": "confidence_use", "width": 3.36, "data": CHARTS["confidence_use"]}
    return {
        "schema": report.SCHEMA,
        "template": "test",
        "title": "Decision test results, run test0001",
        "subject": "Individual results",
        "footer": "Decision test  ·  run test0001  ·  prepared 1 January 2026",
        "owner_flag": "OWNER'S COPY",
        "blocks": [
            {"type": "kicker", "text": "DECISION TEST"},
            {"type": "title", "text": "Your results"},
            {"type": "meta", "text": "Run test0001"},
            {
                "type": "note",
                "only": "owner",
                "text": "<b>Owner's copy.</b> Not for the participant.",
            },
            {
                "type": "tiles",
                "tiles": [
                    {"label": "Score", "value": "12 of 30", "detail": "40% correct."},
                    {
                        "label": "Standing",
                        "value": "112th of 2400",
                        "detail": "A long value that must shrink.",
                    },
                ],
            },
            {"type": "h2", "text": "What stands out"},
            {
                "type": "numbered",
                "items": ["<b>First.</b> One.", "<b>Second.</b> Two &amp; three."],
            },
            {"type": "spacer", "height": 4},
            {"type": "page_break"},
            chart("standing"),
            {"type": "caption", "text": "Each row has its own scale."},
            chart("blocks"),
            {
                "type": "columns",
                "left_width": 3.5,
                "left": [{"type": "chart", "chart": half}, {"type": "caption", "text": "Left."}],
                "right": [
                    {"type": "chart", "chart": {**half, "kind": "calibration"}},
                    {"type": "h3", "text": "Right"},
                    {"type": "bullets", "items": ["One.", "Two."]},
                ],
            },
            {"type": "page_break"},
            chart("scatter"),
            chart("families"),
            chart("time_by_outcome", only="participant"),
            chart("timeline", only="owner"),
            {
                "type": "table",
                "table": {
                    "style": "data",
                    "bold_first": True,
                    "breaks": [1],
                    "columns": [
                        {"title": "Kind", "width": 2.0},
                        {"title": "", "width": 0.3},
                        {"title": "Correct", "width": 1.0, "right": True},
                    ],
                    "rows": [
                        [{"text": "Syllogism"}, {"marker": "correct"}, {"text": "6"}],
                        [{"text": "All thirty"}, {"marker": "lure"}, {"text": "12"}],
                    ],
                },
            },
            {"type": "page_break", "only": "owner"},
            {"type": "h2", "only": "owner", "text": "Appendix"},
            {
                "type": "table",
                "only": "owner",
                "table": {
                    "style": "small",
                    "columns": [
                        {"title": "No.", "width": 0.4},
                        {"title": "Question", "width": 4.0},
                        {"title": "", "width": 0.2},
                        {"title": "Result", "width": 1.0},
                    ],
                    "rows": [
                        [
                            {"text": "1", "bold": True},
                            {"text": KEY_TEXT},
                            {"marker": "expired"},
                            {"text": "Ran out of time"},
                        ]
                    ],
                },
            },
        ],
    }


def text_of(pdf: bytes) -> str:
    return " ".join(page.extract_text() for page in PdfReader(io.BytesIO(pdf)).pages)


def test_renders_two_copies_and_keeps_the_key_out_of_one():
    res = report.render_report(json.dumps(document()))
    assert res.participant_pdf.startswith(b"%PDF") and res.owner_pdf.startswith(b"%PDF")
    assert res.engine.startswith("reportlab:")

    participant, owner = text_of(res.participant_pdf), text_of(res.owner_pdf)
    # The difference between the copies is the blocks marked for one of
    # them, and nothing else.
    assert KEY_TEXT in owner
    assert KEY_TEXT not in participant
    assert "OWNER'S COPY" in owner
    assert "OWNER'S COPY" not in participant
    assert "What stands out" in participant and "What stands out" in owner

    # The owner's copy has the appendix page; the participant's does not,
    # and does not end on an empty page where it would have been.
    assert res.owner_pages == res.participant_pages + 1
    assert len(PdfReader(io.BytesIO(res.participant_pdf)).pages) == res.participant_pages
    assert f"Page {res.participant_pages} of {res.participant_pages}" in participant


def test_every_chart_draws_with_and_without_a_group():
    for kind, data in CHARTS.items():
        assert charts.draw(kind, data).startswith(b"\x89PNG"), kind
    alone_levels = [{**level, "others_share": None, "others_acc": None} for level in LEVELS]
    alone_blocks = [{**b, "others_correct": None, "others_conf": None} for b in BLOCKS]
    alone_blocks[2]["conf"] = None  # a block in which nothing was rated
    assert charts.draw("blocks", {"compare": False, "blocks": alone_blocks}).startswith(b"\x89PNG")
    assert charts.draw("confidence_use", {"compare": False, "levels": alone_levels}).startswith(
        b"\x89PNG"
    )
    assert charts.draw("calibration", {"compare": False, "levels": alone_levels}).startswith(
        b"\x89PNG"
    )
    alone_families = [{**r, "others_acc": None} for r in CHARTS["families"]["rows"]]
    assert charts.draw("families", {"compare": False, "rows": alone_families}).startswith(
        b"\x89PNG"
    )
    assert charts.draw("timeline", {**CHARTS["timeline"], "others_typical": None}).startswith(
        b"\x89PNG"
    )


def test_an_empty_list_sent_as_null_is_drawn_as_empty():
    # What a Go slice that was never appended to looks like on the wire.
    # The first real run with no trap answers stopped the renderer on it.
    rows = [dict(r) for r in CHARTS["time_by_outcome"]["rows"]]
    rows[2]["times"] = None
    drawn = charts.draw("time_by_outcome", {**CHARTS["time_by_outcome"], "rows": rows})
    assert drawn.startswith(b"\x89PNG")
    assert charts.draw("scatter", {"you": {"conf": 48, "acc": 40}, "others": None}).startswith(
        b"\x89PNG"
    )
    strip = {**CHARTS["standing"]["rows"][0], "others": None, "median": None}
    assert charts.draw("standing", {"legend": "Nobody yet", "rows": [strip]}).startswith(b"\x89PNG")
    assert charts.draw(
        "timeline", {"limit_s": 25, "others_typical": None, "points": None}
    ).startswith(b"\x89PNG")


def test_a_large_group_still_fits_its_strip():
    row = dict(CHARTS["standing"]["rows"][0])
    row["others"] = [10] * 40 + list(range(0, 31)) * 6
    assert charts.draw("standing", {"legend": "The 226 others", "rows": [row]}).startswith(
        b"\x89PNG"
    )


def test_refuses_what_it_cannot_draw():
    doc = document()
    doc["schema"] = report.SCHEMA + 1
    with pytest.raises(ValueError, match="schema"):
        report.render_report(json.dumps(doc))

    with pytest.raises(ValueError, match="no blocks"):
        report.render_report(json.dumps({"schema": report.SCHEMA, "blocks": []}))

    doc = document()
    doc["blocks"].append({"type": "chart", "chart": {"kind": "pie", "data": {}}})
    with pytest.raises(ValueError, match="unknown chart kind"):
        report.render_report(json.dumps(doc))

    doc = document()
    doc["blocks"].append({"type": "marquee", "text": "x"})
    with pytest.raises(ValueError, match="unknown block type"):
        report.render_report(json.dumps(doc))
