"""Lay a report document out as a PDF.

The document is a list of blocks the api wrote: headings, paragraphs,
tables, charts. This module draws each one and decides nothing about
what the report says. A block marked for one copy is skipped in the
other, which is the whole difference between the participant's copy
and the owner's.
"""

from __future__ import annotations

import io
from functools import lru_cache

from PIL import Image as PILImage
from reportlab.graphics.shapes import Circle, Drawing, Polygon, Rect
from reportlab.lib.colors import HexColor
from reportlab.lib.enums import TA_RIGHT
from reportlab.lib.pagesizes import letter
from reportlab.lib.styles import ParagraphStyle
from reportlab.lib.units import inch
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.pdfgen import canvas as rl_canvas
from reportlab.platypus import (
    BaseDocTemplate,
    Frame,
    Image,
    PageBreak,
    PageTemplate,
    Paragraph,
    Spacer,
    Table,
    TableStyle,
)

from career_sidecar.report import charts, fonts

INK, INK2, MUTED = HexColor("#0b0b0b"), HexColor("#3d3c3a"), HexColor("#6f6d68")
HAIR, WASH, RULE = HexColor("#e1e0d9"), HexColor("#f7f7f4"), HexColor("#c3c2b7")
BLUE, ORANGE, GREY = HexColor("#2a78d6"), HexColor("#eb6834"), HexColor("#898781")

PAGE_W, PAGE_H = letter
MARGIN = 0.75 * inch
WIDTH = PAGE_W - 2 * MARGIN
TOP, BOTTOM = 0.7 * inch, 0.85 * inch

FONT, BOLD = "Report", "Report-Bold"


@lru_cache(maxsize=1)
def _styles() -> dict[str, ParagraphStyle]:
    fam = fonts.family()
    pdfmetrics.registerFont(TTFont(FONT, str(fam.regular)))
    pdfmetrics.registerFont(TTFont(BOLD, str(fam.bold)))
    pdfmetrics.registerFont(TTFont("Report-Italic", str(fam.italic)))
    pdfmetrics.registerFont(TTFont("Report-BoldItalic", str(fam.bold_italic)))
    pdfmetrics.registerFontFamily(
        FONT, normal=FONT, bold=BOLD, italic="Report-Italic", boldItalic="Report-BoldItalic"
    )

    def style(name: str, font: str, size: float, leading: float, colour, **kw) -> ParagraphStyle:
        return ParagraphStyle(
            name, fontName=font, fontSize=size, leading=leading, textColor=colour, **kw
        )

    return {
        "kicker": style("kicker", BOLD, 8, 10, MUTED, spaceAfter=6),
        "title": style("title", BOLD, 23, 27, INK, spaceAfter=6),
        "meta": style("meta", FONT, 8.6, 12, MUTED, spaceAfter=14),
        "h2": style("h2", BOLD, 13.5, 17, INK, spaceBefore=16, spaceAfter=6),
        "h3": style("h3", BOLD, 10, 13, INK, spaceBefore=10, spaceAfter=3),
        "p": style("p", FONT, 9.6, 14, INK2, spaceAfter=7),
        "numbered": style(
            "numbered", FONT, 9.6, 14, INK2, spaceAfter=7, leftIndent=16, bulletIndent=0
        ),
        "bullet": style(
            "bullet", FONT, 9.4, 13.6, INK2, spaceAfter=4.5, leftIndent=11, bulletIndent=0
        ),
        "caption": style("caption", FONT, 8, 11, MUTED, spaceBefore=3, spaceAfter=8),
        "note": style("note", FONT, 8.8, 12.6, INK2),
        "cell": style("cell", FONT, 8.4, 11, INK2),
        "cellb": style("cellb", BOLD, 8.4, 11, INK),
        "cellr": style("cellr", FONT, 8.4, 11, INK2, alignment=TA_RIGHT),
        "head": style("head", BOLD, 7.4, 9.5, MUTED),
        "headr": style("headr", BOLD, 7.4, 9.5, MUTED, alignment=TA_RIGHT),
        "tile_label": style("tile_label", BOLD, 7.2, 9, MUTED),
        "tile_sub": style("tile_sub", FONT, 7.9, 10.6, INK2),
        "tiny": style("tiny", FONT, 7.3, 9.6, INK2),
        "tinyb": style("tinyb", BOLD, 7.3, 9.6, INK),
    }


def _p(text: str, style: str) -> Paragraph:
    return Paragraph(text, _styles()[style])


def _note(text: str) -> Table:
    t = Table([[_p(text, "note")]], colWidths=[WIDTH])
    t.setStyle(
        TableStyle(
            [
                ("BACKGROUND", (0, 0), (-1, -1), WASH),
                ("LINEBEFORE", (0, 0), (0, -1), 2, RULE),
                ("LEFTPADDING", (0, 0), (-1, -1), 11),
                ("RIGHTPADDING", (0, 0), (-1, -1), 11),
                ("TOPPADDING", (0, 0), (-1, -1), 8),
                ("BOTTOMPADDING", (0, 0), (-1, -1), 8),
            ]
        )
    )
    return t


def _tiles(items: list[dict]) -> Table:
    gap = 8
    w = (WIDTH - gap * (len(items) - 1)) / len(items)
    inner_w = w - 20
    cells, widths = [], []
    for i, tile in enumerate(items):
        # The headline figure is set as large as its tile allows, so
        # "5th of 16" and "112th of 240" both sit on one line.
        size = 19.0
        while size > 11 and pdfmetrics.stringWidth(tile["value"], BOLD, size) > inner_w:
            size -= 0.5
        value = ParagraphStyle(
            "tile_value", fontName=BOLD, fontSize=size, leading=23, textColor=INK
        )
        inner = Table(
            [
                [_p(tile["label"].upper(), "tile_label")],
                [Paragraph(tile["value"], value)],
                [_p(tile["detail"], "tile_sub")],
            ],
            colWidths=[inner_w],
        )
        inner.setStyle(
            TableStyle(
                [
                    ("LEFTPADDING", (0, 0), (-1, -1), 0),
                    ("RIGHTPADDING", (0, 0), (-1, -1), 0),
                    ("TOPPADDING", (0, 0), (-1, -1), 0),
                    ("BOTTOMPADDING", (0, 0), (-1, -1), 2),
                    ("VALIGN", (0, 0), (-1, -1), "TOP"),
                ]
            )
        )
        cells.append(inner)
        widths.append(w)
        if i < len(items) - 1:
            cells.append("")
            widths.append(gap)
    t = Table([cells], colWidths=widths)
    style = [("VALIGN", (0, 0), (-1, -1), "TOP")]
    for i in range(len(items)):
        c = i * 2
        style += [
            ("BOX", (c, 0), (c, 0), 0.6, HAIR),
            ("BACKGROUND", (c, 0), (c, 0), WASH),
            ("LEFTPADDING", (c, 0), (c, 0), 10),
            ("RIGHTPADDING", (c, 0), (c, 0), 10),
            ("TOPPADDING", (c, 0), (c, 0), 10),
            ("BOTTOMPADDING", (c, 0), (c, 0), 10),
        ]
    t.setStyle(TableStyle(style))
    return t


def _marker(outcome: str) -> Drawing:
    """The small shape that stands for an outcome, matching the charts:
    hue and shape together, so it reads without colour."""
    d = Drawing(10, 9)
    if outcome == "correct":
        shape, fill, stroke = Circle(5, 4.5, 3.6), BLUE, None
    elif outcome == "lure":
        shape, fill, stroke = Polygon([5, 0.4, 9.1, 4.5, 5, 8.6, 0.9, 4.5]), ORANGE, None
    elif outcome == "other":
        shape, fill, stroke = Rect(1.7, 1.2, 6.6, 6.6), GREY, None
    else:  # ran out of time: an open ring, the absence of an answer
        shape, fill, stroke = Circle(5, 4.5, 3.2), None, INK2
    # Set after construction: reportlab's shapes take their paint as
    # attributes, and the keyword form is typed more narrowly than it
    # behaves.
    shape.fillColor = fill
    shape.strokeColor = stroke
    shape.strokeWidth = 1 if stroke is not None else 0
    d.add(shape)
    return d


def _table(spec: dict) -> Table:
    small = spec.get("style") == "small"
    cols = spec["columns"]
    head = [_p(c["title"], "headr" if c.get("right") else "head") for c in cols]
    body = []
    for r in spec["rows"]:
        line = []
        for i, cell in enumerate(r):
            if cell.get("marker"):
                line.append(_marker(cell["marker"]))
                continue
            if small:
                style = "tinyb" if cell.get("bold") else "tiny"
            elif cols[i].get("right"):
                style = "cellr"
            elif cell.get("bold") or (spec.get("bold_first") and i == 0):
                style = "cellb"
            else:
                style = "cell"
            line.append(_p(cell.get("text", ""), style))
        body.append(line)
    t = Table([head] + body, colWidths=[c["width"] * inch for c in cols], repeatRows=1)
    pad = 4 if small else 4.2
    style = [
        ("LINEBELOW", (0, 0), (-1, 0), 0.8, RULE),
        ("LINEBELOW", (0, 1), (-1, -1), 0.5, HAIR),
        ("VALIGN", (0, 0), (-1, -1), "TOP" if small else "MIDDLE"),
        ("TOPPADDING", (0, 0), (-1, -1), pad),
        ("BOTTOMPADDING", (0, 0), (-1, -1), pad),
        ("LEFTPADDING", (0, 0), (-1, -1), 3 if small else 4),
        ("RIGHTPADDING", (0, 0), (-1, -1), 5 if small else 6),
    ]
    for r in spec.get("breaks") or []:
        style.append(("LINEABOVE", (0, r + 1), (-1, r + 1), 0.9, RULE))
    t.setStyle(TableStyle(style))
    return t


class ChartCache:
    """Draws each chart once. Both copies of a report show mostly the
    same figures, and drawing is the slow part of rendering."""

    def __init__(self) -> None:
        self._png: dict[int, bytes] = {}

    def image(self, spec: dict) -> Image:
        key = id(spec)
        if key not in self._png:
            self._png[key] = charts.draw(spec["kind"], spec["data"])
        png = self._png[key]
        with PILImage.open(io.BytesIO(png)) as im:
            w, h = im.size
        width = (spec.get("width") or WIDTH / inch) * inch
        return Image(io.BytesIO(png), width=width, height=width * h / w)


def _flowables(blocks: list[dict], owner: bool, cache: ChartCache) -> list:
    out: list = []
    for b in blocks:
        only = b.get("only") or ""
        if (only == "owner" and not owner) or (only == "participant" and owner):
            continue
        kind = b["type"]
        if kind in ("kicker", "title", "meta", "h2", "h3", "p", "caption"):
            out.append(_p(b.get("text", ""), kind))
        elif kind == "bullets":
            out += [Paragraph(t, _styles()["bullet"], bulletText="•") for t in b.get("items") or []]
        elif kind == "numbered":
            out += [
                Paragraph(t, _styles()["numbered"], bulletText=f"{i}.")
                for i, t in enumerate(b.get("items") or [], 1)
            ]
        elif kind == "note":
            out.append(_note(b.get("text", "")))
        elif kind == "tiles":
            out.append(_tiles(b["tiles"]))
        elif kind == "table":
            out.append(_table(b["table"]))
        elif kind == "chart":
            out.append(cache.image(b["chart"]))
        elif kind == "columns":
            left_w = b.get("left_width", WIDTH / inch / 2) * inch
            t = Table(
                [
                    [
                        _flowables(b.get("left") or [], owner, cache),
                        _flowables(b.get("right") or [], owner, cache),
                    ]
                ],
                colWidths=[left_w, WIDTH - left_w],
            )
            t.setStyle(
                TableStyle(
                    [
                        ("LEFTPADDING", (0, 0), (-1, -1), 0),
                        ("RIGHTPADDING", (0, 0), (0, 0), 10),
                        ("RIGHTPADDING", (1, 0), (1, 0), 0),
                        ("TOPPADDING", (0, 0), (-1, -1), 0),
                        ("BOTTOMPADDING", (0, 0), (-1, -1), 0),
                        ("VALIGN", (0, 0), (-1, -1), "TOP"),
                    ]
                )
            )
            out.append(t)
        elif kind == "spacer":
            out.append(Spacer(1, b.get("height", 6)))
        elif kind == "page_break":
            out.append(PageBreak())
        else:
            raise ValueError(f"unknown block type {kind!r}")
    # A page break with nothing after it would end the file on an empty
    # page, which is what a copy that skips its last section would get.
    while out and isinstance(out[-1], PageBreak):
        out.pop()
    return out


def _canvas(footer: str, flag: str, pages: list[int]):
    class Numbered(rl_canvas.Canvas):
        """Two passes, so the footer can say 'Page 3 of 9'."""

        def __init__(self, *args, **kwargs):
            super().__init__(*args, **kwargs)
            self._saved: list[dict] = []

        def showPage(self):  # noqa: N802 - reportlab's name
            self._saved.append(dict(self.__dict__))
            self._startPage()  # pyright: ignore[reportAttributeAccessIssue]

        def save(self):
            total = len(self._saved)
            pages.append(total)
            for state in self._saved:
                self.__dict__.update(state)
                self.setStrokeColor(HAIR)
                self.setLineWidth(0.6)
                self.line(MARGIN, 0.62 * inch, PAGE_W - MARGIN, 0.62 * inch)
                self.setFont(FONT, 7.4)
                self.setFillColor(MUTED)
                self.drawString(MARGIN, 0.45 * inch, footer)
                number = self._pageNumber  # pyright: ignore[reportAttributeAccessIssue]
                self.drawRightString(PAGE_W - MARGIN, 0.45 * inch, f"Page {number} of {total}")
                if flag:
                    self.setFont(BOLD, 7.2)
                    self.setFillColor(INK)
                    self.drawRightString(PAGE_W - MARGIN, PAGE_H - 0.47 * inch, flag)
                super().showPage()
            super().save()

    return Numbered


def build_pdf(doc: dict, owner: bool, cache: ChartCache) -> tuple[bytes, int]:
    """One copy of the report, and how many pages it came to."""
    _styles()
    out = io.BytesIO()
    pages: list[int] = []
    template = BaseDocTemplate(
        out,
        pagesize=letter,
        leftMargin=MARGIN,
        rightMargin=MARGIN,
        topMargin=TOP,
        bottomMargin=BOTTOM,
        title=doc.get("title", ""),
        author="Decision test",
        subject=doc.get("subject", ""),
    )
    frame = Frame(
        MARGIN,
        BOTTOM,
        WIDTH,
        PAGE_H - TOP - BOTTOM,
        leftPadding=0,
        rightPadding=0,
        topPadding=0,
        bottomPadding=0,
    )
    template.addPageTemplates([PageTemplate(id="page", frames=[frame])])
    template.build(
        _flowables(doc["blocks"], owner, cache),
        canvasmaker=_canvas(
            doc.get("footer", ""), doc.get("owner_flag", "") if owner else "", pages
        ),
    )
    return out.getvalue(), pages[0] if pages else 0
