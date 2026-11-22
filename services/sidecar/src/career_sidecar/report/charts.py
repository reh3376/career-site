"""Charts for the decision test report. Static, light surface, print.

Form before colour: the participant is the point and everyone else is
context, so every comparison is an emphasis chart, one hue and grey.
Outcome charts use two hues (correct, intuitive trap) and two greys.
Blue and orange pass the palette checks against a white surface; the
greys are neutral on purpose.

Each function takes the payload the api built for it and returns a PNG.
Nothing here computes a figure about a person: the numbers arrive
worked out, and the only arithmetic below is where to put a mark.
"""

from __future__ import annotations

import io
import math

import matplotlib

matplotlib.use("Agg")

from matplotlib import font_manager  # noqa: E402
from matplotlib.backends.backend_agg import FigureCanvasAgg  # noqa: E402
from matplotlib.figure import Figure  # noqa: E402
from matplotlib.lines import Line2D  # noqa: E402
from matplotlib.patches import PathPatch  # noqa: E402
from matplotlib.path import Path  # noqa: E402
from matplotlib.transforms import IdentityTransform  # noqa: E402

from career_sidecar.report import fonts  # noqa: E402

INK, INK2, MUTED = "#0b0b0b", "#52514e", "#898781"
GRID, AXIS, SURFACE = "#e1e0d9", "#c3c2b7", "#ffffff"
BLUE, ORANGE = "#2a78d6", "#eb6834"
GREY, LIGHT, OTHERS = "#898781", "#d3d2cb", "#a9a79f"

# Print resolution for a chart seven inches wide. Higher buys nothing
# on paper and doubles the size of a file that travels by email.
DPI = 220
_PX = DPI / 300  # radii below were drawn at 300 and scale with the dpi

_family_name: str | None = None


def _rc() -> dict:
    """The matplotlib settings every chart is drawn under."""
    global _family_name
    if _family_name is None:
        fam = fonts.family()
        for path in (fam.regular, fam.bold, fam.italic, fam.bold_italic):
            font_manager.fontManager.addfont(str(path))
        _family_name = font_manager.FontProperties(fname=str(fam.regular)).get_name()
    return {
        "font.family": _family_name,
        "font.size": 8,
        "text.color": INK,
        "axes.edgecolor": AXIS,
        "axes.labelcolor": INK2,
        "axes.linewidth": 0.6,
        "xtick.color": MUTED,
        "ytick.color": MUTED,
        "xtick.labelcolor": INK2,
        "ytick.labelcolor": INK2,
        "xtick.major.size": 0,
        "ytick.major.size": 0,
        "xtick.major.pad": 5,
        "ytick.major.pad": 5,
        "axes.grid": False,
        "figure.facecolor": SURFACE,
        "axes.facecolor": SURFACE,
        "savefig.facecolor": SURFACE,
        "figure.dpi": DPI,
        "savefig.dpi": DPI,
        "legend.frameon": False,
        "legend.fontsize": 7.5,
        "axes.unicode_minus": True,
    }


def _figure(width: float, height: float) -> Figure:
    fig = Figure(figsize=(width, height), dpi=DPI)
    FigureCanvasAgg(fig)
    return fig


def _png(fig: Figure) -> bytes:
    out = io.BytesIO()
    fig.savefig(out, format="png", dpi=DPI)
    return out.getvalue()


def _clean(ax, left: bool = False, bottom: bool = True) -> None:
    for side in ("top", "right", "left", "bottom"):
        ax.spines[side].set_visible(False)
    ax.spines["bottom"].set_visible(bottom)
    ax.spines["left"].set_visible(left)


def _hgrid(ax, ys) -> None:
    for y in ys:
        ax.axhline(y, color=GRID, lw=0.6, zorder=0)


def _tick(v: float) -> str:
    s = f"{v:g}"
    return s.replace("-", "−")


def _rounded(ax, x0, x1, y0, y1, color, end="top", r_px=7.0, zorder=2) -> None:
    """A bar with its data end rounded and its baseline end square.

    Drawn in pixel space so the corner is a true circle whatever the
    axes' aspect. The figure is created and saved at one dpi, and the
    axes are placed by hand, so the pixels do not move afterwards.
    """
    (px0, py0), (px1, py1) = ax.transData.transform([(x0, y0), (x1, y1)])
    left, right = min(px0, px1), max(px0, px1)
    bottom, top = min(py0, py1), max(py0, py1)
    r = min(r_px * _PX, (right - left) / 2, (top - bottom) / 2)
    if r <= 0.5:
        verts = [(left, bottom), (right, bottom), (right, top), (left, top), (left, bottom)]
        codes = [Path.MOVETO, Path.LINETO, Path.LINETO, Path.LINETO, Path.CLOSEPOLY]
    elif end == "top":
        verts = [
            (left, bottom),
            (right, bottom),
            (right, top - r),
            (right, top),
            (right - r, top),
            (left + r, top),
            (left, top),
            (left, top - r),
            (left, bottom),
        ]
        codes = [
            Path.MOVETO,
            Path.LINETO,
            Path.LINETO,
            Path.CURVE3,
            Path.CURVE3,
            Path.LINETO,
            Path.CURVE3,
            Path.CURVE3,
            Path.CLOSEPOLY,
        ]
    else:  # the data end is on the right
        verts = [
            (left, bottom),
            (right - r, bottom),
            (right, bottom),
            (right, bottom + r),
            (right, top - r),
            (right, top),
            (right - r, top),
            (left, top),
            (left, bottom),
        ]
        codes = [
            Path.MOVETO,
            Path.LINETO,
            Path.CURVE3,
            Path.CURVE3,
            Path.LINETO,
            Path.CURVE3,
            Path.CURVE3,
            Path.LINETO,
            Path.CLOSEPOLY,
        ]
    ax.add_patch(
        PathPatch(
            Path(verts, codes),
            facecolor=color,
            edgecolor="none",
            transform=IdentityTransform(),
            zorder=zorder,
            clip_on=False,
        )
    )


def standing(d: dict) -> bytes:
    """One strip per measure: every first-time participant a dot, the
    participant in blue. The numbers sit in the text beside each strip,
    so no label has to fight a dot for room."""
    rows = d["rows"]
    row_in, head_in, foot_in = 0.73, 0.375, 0.245
    height = head_in + row_in * len(rows) + foot_in
    fig = _figure(7.0, height)

    def fy(inches_from_top: float) -> float:
        return 1 - inches_from_top / height

    fig.legend(
        handles=[
            Line2D(
                [],
                [],
                marker="o",
                ls="",
                color=BLUE,
                markersize=7,
                markeredgecolor=SURFACE,
                markeredgewidth=1.2,
                label="You",
            ),
            Line2D(
                [],
                [],
                marker="o",
                ls="",
                color=OTHERS,
                markersize=5.2,
                markeredgecolor=SURFACE,
                markeredgewidth=0.8,
                label=d["legend"],
            ),
            Line2D([], [], color=INK2, lw=1.0, label="Their median"),
        ],
        loc="upper left",
        bbox_to_anchor=(0.395, 0.995),
        ncol=3,
        columnspacing=1.6,
        handletextpad=0.5,
    )
    for i, r in enumerate(rows):
        top = head_in + i * row_in
        fig.text(
            0.0, fy(top + 0.15), r["title"], fontsize=9, fontweight="bold", color=INK, va="center"
        )
        fig.text(0.0, fy(top + 0.29), r["unit"], fontsize=7.2, color=MUTED, va="center")
        fig.text(0.0, fy(top + 0.45), r["line"], fontsize=7.8, color=INK2, va="center")
        fig.text(0.0, fy(top + 0.58), r["placing"], fontsize=7.8, color=INK2, va="center")
        ax = fig.add_axes((0.40, fy(top + 0.61), 0.585, 0.525 / height))
        _clean(ax)
        lo, hi = r["lo"], r["hi"]
        ax.set_xlim(lo, hi)
        ax.set_ylim(-0.6, 5.2)
        ax.set_yticks([])
        ax.set_xticks(r["ticks"])
        ax.set_xticklabels([_tick(t) for t in r["ticks"]], fontsize=7)
        step = (hi - lo) / 46  # dots closer than this stack

        def slot(v: float, lo=lo, step=step) -> int:
            return round((v - lo) / step)

        others = sorted(r.get("others") or [])
        counts: dict[int, int] = {}
        for v in others + [r["mine"]]:
            counts[slot(v)] = counts.get(slot(v), 0) + 1
        tallest = max(counts.values())
        # Six dots fit a row at full spacing. A larger group packs them
        # closer, so the strip stays a strip rather than leaving the row.
        rise = 0.80 if tallest <= 6 else 4.0 / (tallest - 1)
        size = 30 if tallest <= 8 else 16
        if r.get("median") is not None:
            ax.plot(
                [r["median"]] * 2, [-0.6, 4.6], color=INK2, lw=1.0, zorder=1, solid_capstyle="butt"
            )
        seen: dict[int, int] = {}
        for v in others:
            k = seen.get(slot(v), 0)
            seen[slot(v)] = k + 1
            ax.scatter(
                [lo + slot(v) * step],
                [0.45 + k * rise],
                s=size,
                color=OTHERS,
                edgecolor=SURFACE,
                linewidth=0.8,
                zorder=3,
                clip_on=False,
            )
        k = seen.get(slot(r["mine"]), 0)
        ax.scatter(
            [lo + slot(r["mine"]) * step],
            [0.45 + k * rise],
            s=62,
            color=BLUE,
            edgecolor=SURFACE,
            linewidth=1.3,
            zorder=4,
            clip_on=False,
        )
    return _png(fig)


CONF_TICKS = [10, 30, 50, 70, 90]
CONF_WORDS = ["Guessing", "Doubtful", "Unsure", "Fairly sure", "Certain"]


def blocks(d: dict) -> bytes:
    """Two panels on one block axis: what happened to the questions in
    each block, and the confidence given. Two measures with two scales,
    so two panels, never two axes on one plot."""
    rows, compare = d["blocks"], d["compare"]
    n = len(rows)
    xs = list(range(1, n + 1))
    per_block = max(max(r["correct"] + r["lure"] + r["other"] + r["expired"] for r in rows), 1)
    fig = _figure(7.0, 4.0)
    ax = fig.add_axes((0.135, 0.545, 0.775, 0.335))
    _clean(ax)
    ax.set_xlim(0.45, n + 0.55)
    ax.set_ylim(0, per_block * 1.042)
    ax.set_yticks(range(0, per_block + 1))
    _hgrid(ax, range(1, per_block + 1))
    ax.set_xticks(xs)
    ax.set_xticklabels([])
    ax.set_ylabel(f"Questions ({per_block} per block)", fontsize=7.5)
    fig.canvas.draw()
    w = 0.30
    parts = [
        ("correct", BLUE, "#ffffff"),
        ("lure", ORANGE, "#ffffff"),
        ("other", GREY, "#ffffff"),
        ("expired", LIGHT, INK),
    ]
    gap = 0.0075 * per_block  # the surface gap between segments
    for x, r in zip(xs, rows, strict=True):
        y = 0.0
        present = [(k, c, t) for k, c, t in parts if r[k] > 0]
        for j, (k, c, t) in enumerate(present):
            h = float(r[k])
            lo = y + (gap / 2 if j > 0 else 0)
            hi = y + h - (gap / 2 if j < len(present) - 1 else 0)
            _rounded(
                ax,
                x - w / 2,
                x + w / 2,
                lo,
                hi,
                c,
                end="top",
                r_px=7 if j == len(present) - 1 else 0,
            )
            ax.text(
                x,
                y + h / 2,
                f"{int(h)}",
                ha="center",
                va="center",
                fontsize=8,
                color=t,
                fontweight="bold",
                zorder=5,
            )
            y += h
    handles = [
        Line2D([], [], marker="s", ls="", color=BLUE, markersize=7, label="Correct"),
        Line2D([], [], marker="s", ls="", color=ORANGE, markersize=7, label="Intuitive trap"),
        Line2D([], [], marker="s", ls="", color=GREY, markersize=7, label="Other wrong answer"),
        Line2D([], [], marker="s", ls="", color=LIGHT, markersize=7, label="Ran out of time"),
    ]
    if compare:
        for x, r in zip(xs, rows, strict=True):
            v = r["others_correct"]
            if v is None:
                continue
            ax.scatter(
                [x + 0.30],
                [v],
                s=38,
                marker="D",
                color=INK2,
                edgecolor=SURFACE,
                linewidth=1.0,
                zorder=4,
            )
            ax.annotate(
                f"{v:.1f}",
                (x + 0.30, v),
                xytext=(7, 0),
                textcoords="offset points",
                fontsize=7,
                color=INK2,
                va="center",
            )
        handles.append(
            Line2D(
                [],
                [],
                marker="D",
                ls="",
                color=INK2,
                markersize=4.8,
                label="Others' average correct",
            )
        )
    fig.legend(
        handles=handles,
        loc="upper left",
        bbox_to_anchor=(0.125, 0.995),
        ncol=len(handles),
        columnspacing=1.2,
        handletextpad=0.3,
    )
    fig.text(
        0.135,
        0.905,
        f"Your answers in each block, of {per_block}",
        fontsize=8.5,
        fontweight="bold",
        color=INK,
    )

    ax2 = fig.add_axes((0.135, 0.14, 0.775, 0.265))
    _clean(ax2)
    ax2.set_xlim(0.45, n + 0.55)
    ax2.set_ylim(0, 100)
    ax2.set_yticks(CONF_TICKS)
    ax2.set_yticklabels(
        [f"{w_}  {t}" for t, w_ in zip(CONF_TICKS, CONF_WORDS, strict=True)], fontsize=7
    )
    _hgrid(ax2, CONF_TICKS)
    ax2.set_xticks(xs)
    ax2.set_xticklabels([r["label"] for r in rows], fontsize=7.5)
    mine = [(x, r["conf"]) for x, r in zip(xs, rows, strict=True) if r["conf"] is not None]
    theirs = [
        (x, r["others_conf"]) for x, r in zip(xs, rows, strict=True) if r["others_conf"] is not None
    ]
    if compare and theirs:
        ax2.plot(*zip(*theirs, strict=True), color=OTHERS, lw=2, solid_capstyle="round", zorder=2)
        ax2.scatter(
            *zip(*theirs, strict=True),
            s=40,
            color=OTHERS,
            edgecolor=SURFACE,
            linewidth=1.2,
            zorder=3,
        )
    if mine:
        ax2.plot(*zip(*mine, strict=True), color=BLUE, lw=2, solid_capstyle="round", zorder=4)
        ax2.scatter(
            *zip(*mine, strict=True), s=46, color=BLUE, edgecolor=SURFACE, linewidth=1.3, zorder=5
        )
    if compare and theirs and mine and theirs[-1][0] == n and mine[-1][0] == n:
        # The two end labels share a margin; when the lines finish close
        # together they are moved apart rather than printed on each other.
        a, b = mine[-1][1], theirs[-1][1]
        nudge = 5 if abs(a - b) < 9 else 0
        up = 1 if a >= b else -1
        ax2.annotate(
            "You",
            (n, a),
            xytext=(9, up * nudge),
            textcoords="offset points",
            fontsize=7.6,
            color=INK,
            fontweight="bold",
            va="center",
            annotation_clip=False,
        )
        ax2.annotate(
            "Others' average",
            (n, b),
            xytext=(9, -up * nudge),
            textcoords="offset points",
            fontsize=7.2,
            color=INK2,
            va="center",
            annotation_clip=False,
        )
    if mine:
        lowest = min(mine, key=lambda p: p[1])
        others_at = dict(theirs) if compare else {}
        for x, v in {mine[0], lowest}:
            # The figure goes under its point, unless the others' line
            # passes just beneath it or the point sits on the floor, in
            # which case it goes above and stays clear of both.
            theirs_here = others_at.get(x)
            crowded = theirs_here is not None and 0 <= v - theirs_here < 18
            below = v > 14 and not crowded
            ax2.annotate(
                f"{v:.0f}",
                (x, v),
                xytext=(0, -11 if below else 11),
                textcoords="offset points",
                fontsize=7.4,
                color=INK,
                ha="center",
                va="center",
            )
    fig.text(
        0.135, 0.43, "Average confidence in each block", fontsize=8.5, fontweight="bold", color=INK
    )
    return _png(fig)


def confidence_use(d: dict) -> bytes:
    """How often each of the five ratings was chosen."""
    levels, compare = d["levels"], d["compare"]
    shares = [r["share"] for r in levels] + [r["others_share"] or 0 for r in levels if compare]
    top = max(60, 20 * math.ceil((max(shares) + 4) / 20))
    fig = _figure(3.4, 2.2)
    ax = fig.add_axes((0.13, 0.20, 0.84, 0.655))
    _clean(ax)
    ax.set_xlim(-0.6, len(levels) - 0.4)
    ax.set_ylim(0, top + 2)
    ticks = list(range(0, top + 1, 20))
    ax.set_yticks(ticks)
    ax.set_yticklabels(["0"] + [f"{t}%" for t in ticks[1:]], fontsize=7)
    _hgrid(ax, ticks[1:])
    ax.set_xticks(range(len(levels)))
    ax.set_xticklabels([f"{r['label']}\n{r['level']}" for r in levels], fontsize=6.8)
    fig.canvas.draw()
    w, g = 0.30, 0.035
    for i, r in enumerate(levels):
        # With a group the participant's bar sits left of the others';
        # alone, it sits on the tick.
        x0 = i - w - g / 2 if compare else i - w / 2
        if r["share"] > 0:
            _rounded(ax, x0, x0 + w, 0, r["share"], BLUE)
        ax.text(
            x0 + w / 2,
            r["share"] + top / 38,
            f"{r['share']:.0f}%",
            ha="center",
            va="bottom",
            fontsize=6.6,
            color=INK,
            fontweight="bold",
        )
        if compare and r["others_share"]:
            _rounded(ax, i + g / 2, i + w + g / 2, 0, r["others_share"], OTHERS)
    if compare:
        fig.legend(
            handles=[
                Line2D([], [], marker="s", ls="", color=BLUE, markersize=6.5, label="You"),
                Line2D([], [], marker="s", ls="", color=OTHERS, markersize=6.5, label="Others"),
            ],
            loc="upper left",
            bbox_to_anchor=(0.11, 1.0),
            ncol=2,
            columnspacing=1.4,
            handletextpad=0.4,
        )
    return _png(fig)


def calibration(d: dict) -> bytes:
    """Share correct at each rating. On the diagonal a rating means what
    it says."""
    levels, compare = d["levels"], d["compare"]
    fig = _figure(3.4, 2.2)
    ax = fig.add_axes((0.14, 0.20, 0.83, 0.655))
    _clean(ax)
    ax.set_xlim(0, 100)
    ax.set_ylim(0, 100)
    ax.set_yticks([0, 25, 50, 75, 100])
    ax.set_yticklabels(["0", "25%", "50%", "75%", "100%"], fontsize=7)
    _hgrid(ax, [25, 50, 75, 100])
    ax.set_xticks([r["level"] for r in levels])
    ax.set_xticklabels([f"{r['label']}\n{r['level']}" for r in levels], fontsize=6.8)
    ax.set_ylabel("Answers that were correct", fontsize=7.2)
    ax.plot([0, 100], [0, 100], color=AXIS, lw=0.9, zorder=1)
    ax.text(
        84,
        93,
        "a rating that\nmeans what it says",
        fontsize=6.4,
        color=MUTED,
        ha="right",
        va="center",
        linespacing=1.15,
    )
    theirs = [(r["level"], r["others_acc"]) for r in levels if r["others_acc"] is not None]
    if compare and theirs:
        ax.plot(*zip(*theirs, strict=True), color=OTHERS, lw=2, zorder=2, solid_capstyle="round")
        ax.scatter(
            *zip(*theirs, strict=True),
            s=36,
            color=OTHERS,
            edgecolor=SURFACE,
            linewidth=1.1,
            zorder=3,
        )
    mine = [r for r in levels if r["n"] > 0 and r["acc"] is not None]
    if mine:
        ax.plot(
            [r["level"] for r in mine],
            [r["acc"] for r in mine],
            color=BLUE,
            lw=2,
            zorder=4,
            solid_capstyle="round",
        )
        ax.scatter(
            [r["level"] for r in mine],
            [r["acc"] for r in mine],
            s=44,
            color=BLUE,
            edgecolor=SURFACE,
            linewidth=1.3,
            zorder=5,
        )
    for i, r in enumerate(mine):
        # The count goes on the open side of the line: under a dip,
        # over a peak, so it never sits on the segment it labels.
        around = [mine[j]["acc"] for j in (i - 1, i + 1) if 0 <= j < len(mine)]
        below = bool(around) and all(a > r["acc"] for a in around)
        if r["acc"] < 12:
            below = False
        elif r["acc"] > 88:
            below = True
        ax.annotate(
            f"{r['right']} of {r['n']}",
            (r["level"], r["acc"]),
            xytext=(0, -10 if below else 9),
            textcoords="offset points",
            fontsize=6.8,
            color=INK,
            ha="center",
            va="center",
        )
    if compare:
        fig.legend(
            handles=[
                Line2D([], [], marker="o", color=BLUE, markersize=5.5, lw=2, label="You"),
                Line2D([], [], marker="o", color=OTHERS, markersize=5, lw=2, label="Others"),
            ],
            loc="upper left",
            bbox_to_anchor=(0.11, 1.0),
            ncol=2,
            columnspacing=1.4,
            handletextpad=0.5,
        )
    return _png(fig)


def scatter(d: dict) -> bytes:
    """One dot per participant: how sure, against how right."""
    you, others = d["you"], d.get("others") or []
    fig = _figure(3.0, 2.9)
    ax = fig.add_axes((0.145, 0.135, 0.82, 0.80))
    _clean(ax, left=True)
    ax.set_xlim(0, 100)
    ax.set_ylim(0, 100)
    ax.set_aspect("equal")
    ticks = [0, 25, 50, 75, 100]
    ax.set_xticks(ticks)
    ax.set_yticks(ticks)
    ax.tick_params(labelsize=7)
    ax.set_yticklabels(["0", "25%", "50%", "75%", "100%"])
    for t in ticks[1:]:
        ax.axhline(t, color=GRID, lw=0.6, zorder=0)
        ax.axvline(t, color=GRID, lw=0.6, zorder=0)
    ax.set_xlabel("Average confidence (0 to 100)", fontsize=7.5)
    ax.set_ylabel("Questions correct", fontsize=7.5)
    ax.plot([0, 100], [0, 100], color=AXIS, lw=0.9, zorder=1)
    box = {"facecolor": SURFACE, "edgecolor": "none", "pad": 1.2}
    ax.text(
        27, 93, "less sure than right", fontsize=6.8, color=MUTED, ha="center", bbox=box, zorder=2
    )
    ax.text(
        53, 4, "more sure than right", fontsize=6.8, color=MUTED, ha="center", bbox=box, zorder=2
    )
    ax.text(
        88,
        80.5,
        "confidence\nmatches results",
        fontsize=6.4,
        color=MUTED,
        ha="center",
        va="center",
        rotation=45,
        linespacing=1.1,
    )
    # A big group is drawn with smaller, lighter dots so the cloud reads
    # as a cloud and the one blue dot still stands on it.
    size, alpha = (42, 1.0) if len(others) <= 60 else (18, 0.7)
    ax.scatter(
        [o["conf"] for o in others],
        [o["acc"] for o in others],
        s=size,
        color=OTHERS,
        alpha=alpha,
        edgecolor=SURFACE,
        linewidth=1.2 if size > 20 else 0.6,
        zorder=3,
    )
    ax.scatter(
        [you["conf"]], [you["acc"]], s=74, color=BLUE, edgecolor=SURFACE, linewidth=1.4, zorder=5
    )
    left = you["conf"] >= 14
    ax.annotate(
        "You",
        (you["conf"], you["acc"]),
        xytext=(-9 if left else 9, 9),
        textcoords="offset points",
        fontsize=8,
        fontweight="bold",
        color=INK,
        ha="right" if left else "left",
        va="center",
    )
    return _png(fig)


def families(d: dict) -> bytes:
    """Share correct by kind of question."""
    rows, compare = d["rows"], d["compare"]
    height = 0.68 + 0.3425 * len(rows)
    fig = _figure(7.0, height)
    ax = fig.add_axes((0.135, 0.39 / height, 0.80, (height - 0.39 - 0.287) / height))
    _clean(ax, bottom=False)
    ax.set_xlim(0, 100)
    ax.set_ylim(-0.6, len(rows) - 0.4)
    ax.invert_yaxis()
    ax.set_xticks([0, 25, 50, 75, 100])
    ax.set_xticklabels(["0", "25%", "50%", "75%", "100%"], fontsize=7)
    for x in (25, 50, 75, 100):
        ax.axvline(x, color=GRID, lw=0.6, zorder=0)
    ax.axvline(0, color=AXIS, lw=0.8, zorder=1)
    ax.set_yticks(range(len(rows)))
    ax.set_yticklabels([r["label"] for r in rows], fontsize=8.2, color=INK)
    ax.set_xlabel("Questions correct", fontsize=7.5)
    fig.canvas.draw()
    h, g = 0.25, 0.05
    for i, r in enumerate(rows):
        y0 = i - h - g / 2 if compare else i - h / 2
        if r["acc"] > 0:
            _rounded(ax, 0, r["acc"], y0, y0 + h, BLUE, end="right")
        ax.text(
            r["acc"] + 1.5,
            y0 + h / 2,
            f"{r['correct']} of {r['asked']}",
            va="center",
            fontsize=7.6,
            color=INK,
            fontweight="bold",
        )
        if compare and r["others_acc"] is not None:
            if r["others_acc"] > 0:
                _rounded(ax, 0, r["others_acc"], i + g / 2, i + h + g / 2, OTHERS, end="right")
            ax.text(
                r["others_acc"] + 1.5,
                i + h / 2 + g / 2,
                f"{r['others_acc']:.0f}%",
                va="center",
                fontsize=7.2,
                color=INK2,
            )
    if compare:
        fig.legend(
            handles=[
                Line2D([], [], marker="s", ls="", color=BLUE, markersize=6.5, label="You"),
                Line2D(
                    [],
                    [],
                    marker="s",
                    ls="",
                    color=OTHERS,
                    markersize=6.5,
                    label="Others, all their answers of that kind",
                ),
            ],
            loc="upper left",
            bbox_to_anchor=(0.125, 1.0),
            ncol=2,
            columnspacing=1.4,
            handletextpad=0.4,
        )
    return _png(fig)


_MARKS = {"correct": ("o", BLUE, 44), "lure": ("D", ORANGE, 36), "other": ("s", GREY, 34)}


def _second_ticks(limit: float) -> list[float]:
    step = 5 if limit <= 40 else 10 if limit <= 80 else 20
    ticks = [float(t) for t in range(0, int(limit) + 1, step)]
    if ticks[-1] != limit:
        if limit - ticks[-1] < step * 0.6:
            ticks.pop()
        ticks.append(limit)
    return ticks


def time_by_outcome(d: dict) -> bytes:
    """Answer times grouped by how the answer ended. It carries the
    same finding as the timeline without saying which question was
    which."""
    limit, rows = d["limit_s"], d["rows"]
    fig = _figure(7.0, 0.445 + 0.57 * len(rows))
    height = fig.get_figheight()
    ax = fig.add_axes((0.255, 0.505 / height, 0.715, (height - 0.505 - 0.14) / height))
    _clean(ax)
    ax.set_xlim(0, limit * 1.024)
    ax.set_ylim(-0.55, len(rows) - 0.4)
    ax.invert_yaxis()
    ticks = _second_ticks(limit)
    ax.set_xticks(ticks)
    ax.set_xticklabels(
        ["0"] + [f"{t:g} s" for t in ticks[1:-1]] + [f"{limit:g} s\nlimit"], fontsize=7
    )
    ax.set_yticks([])
    for x in ticks[1:-1]:
        if x != d["mark_s"]:
            ax.axvline(x, color=GRID, lw=0.6, zorder=0)
    ax.axvline(d["mark_s"], color=AXIS, lw=0.9, zorder=1)
    ax.axvline(limit, color=INK2, lw=0.9, zorder=1)
    ax.set_xlabel("Seconds taken to answer", fontsize=7.5)
    step = 0.62 * limit / 25
    tr = ax.get_yaxis_transform()
    for i, r in enumerate(rows):
        mk, colour, size = _MARKS.get(r["outcome"], _MARKS["other"])
        ax.text(
            -0.355,
            i - 0.17,
            r["label"],
            transform=tr,
            fontsize=8.6,
            fontweight="bold",
            color=INK,
            va="center",
            clip_on=False,
        )
        ax.text(
            -0.355,
            i + 0.13,
            r["detail"],
            transform=tr,
            fontsize=7.4,
            color=INK2,
            va="center",
            clip_on=False,
        )
        ax.axhline(i + 0.36, color=GRID, lw=0.6, zorder=0)
        seen: dict[int, int] = {}
        for v in sorted(r.get("times") or []):
            key = round(v / step)
            k = seen.get(key, 0)
            seen[key] = k + 1
            ax.scatter(
                [key * step],
                [i + 0.18 - min(k, 2) * 0.27],
                s=size * 0.9,
                marker=mk,
                color=colour,
                edgecolor=SURFACE,
                linewidth=1.0,
                zorder=3,
            )
    return _png(fig)


def timeline(d: dict) -> bytes:
    """Every answer in the order it was given: time taken, and how each
    ended. Owner's copy only, because the order says which was which."""
    limit, points = d["limit_s"], d.get("points") or []
    n = max((p["position"] for p in points), default=1)
    fig = _figure(7.0, 2.3)
    ax = fig.add_axes((0.075, 0.215, 0.80, 0.585))
    _clean(ax)
    ax.set_xlim(0.2, n + 0.8)
    ax.set_ylim(0, limit * 1.08)
    ticks = _second_ticks(limit)
    ax.set_yticks(ticks)
    ax.set_yticklabels(["0"] + [f"{t:g} s" for t in ticks[1:]], fontsize=7)
    _hgrid(ax, ticks[1:-1])
    ax.axhline(limit, color=INK2, lw=0.9, zorder=1)
    ax.text(n + 1.2, limit, "Time limit", fontsize=7, color=INK2, va="center", clip_on=False)
    typical = d.get("others_typical")
    if typical is not None:
        ax.axhline(typical, color=AXIS, lw=0.9, zorder=1)
        ax.text(
            n + 1.2,
            typical,
            f"Others' typical\naverage, {typical:.1f} s",
            fontsize=6.8,
            color=MUTED,
            va="center",
            clip_on=False,
            linespacing=1.15,
        )
    # Block boundaries, and a heading over the middle of each block.
    spans: dict[int, list[int]] = {}
    for p in points:
        spans.setdefault(p["block_no"], []).append(p["position"])
    for block, positions in sorted(spans.items()):
        if min(positions) > 1:
            ax.axvline(min(positions) - 0.5, color=GRID, lw=0.8, zorder=0)
        ax.text(
            (min(positions) + max(positions)) / 2,
            limit * 1.156,
            f"Block {block}",
            fontsize=7.4,
            color=INK2,
            ha="center",
            clip_on=False,
        )
    firsts = sorted({1} | {max(v) for v in spans.values()})
    ax.set_xticks(firsts)
    ax.tick_params(axis="x", labelsize=7)
    ax.set_xlabel("Question, in the order asked", fontsize=7.5)
    for outcome, (mk, colour, size) in _MARKS.items():
        pts = [p for p in points if p["outcome"] == outcome and p["seconds"] is not None]
        ax.scatter(
            [p["position"] for p in pts],
            [p["seconds"] for p in pts],
            s=size,
            marker=mk,
            color=colour,
            edgecolor=SURFACE,
            linewidth=1.1,
            zorder=4,
        )
    out = [p for p in points if p["outcome"] == "expired"]
    ax.scatter(
        [p["position"] for p in out],
        [limit] * len(out),
        s=44,
        marker="o",
        facecolor=SURFACE,
        edgecolor=INK2,
        linewidth=1.2,
        zorder=5,
    )
    fig.legend(
        handles=[
            Line2D(
                [],
                [],
                marker="o",
                ls="",
                color=BLUE,
                markersize=6.5,
                markeredgecolor=SURFACE,
                label="Correct",
            ),
            Line2D(
                [],
                [],
                marker="D",
                ls="",
                color=ORANGE,
                markersize=5.6,
                markeredgecolor=SURFACE,
                label="Intuitive trap",
            ),
            Line2D(
                [],
                [],
                marker="s",
                ls="",
                color=GREY,
                markersize=5.6,
                markeredgecolor=SURFACE,
                label="Other wrong answer",
            ),
            Line2D(
                [],
                [],
                marker="o",
                ls="",
                markerfacecolor=SURFACE,
                markeredgecolor=INK2,
                markersize=6.2,
                label="Ran out of time",
            ),
        ],
        loc="upper left",
        bbox_to_anchor=(0.065, 1.0),
        ncol=4,
        columnspacing=1.5,
        handletextpad=0.3,
    )
    return _png(fig)


KINDS = {
    "standing": standing,
    "blocks": blocks,
    "confidence_use": confidence_use,
    "calibration": calibration,
    "scatter": scatter,
    "families": families,
    "time_by_outcome": time_by_outcome,
    "timeline": timeline,
}


def draw(kind: str, data: dict) -> bytes:
    """Draw one chart. An unknown kind is the api and the sidecar
    disagreeing about the contract, and is an error rather than a gap in
    somebody's report."""
    if kind not in KINDS:
        raise ValueError(f"unknown chart kind {kind!r}")
    with matplotlib.rc_context(_rc()):
        return KINDS[kind](data)
