"""One sans family for the whole report, the same in the charts and on
the page.

Liberation Sans is the production face: it is metric-compatible with
Arial, so a report laid out on a workstation and one laid out in the
container break their lines in the same places. The image installs it
from Debian (`fonts-liberation`), which keeps font binaries out of the
repository. A workstation without it falls back to Arial, and anything
without either falls back to the DejaVu Sans that matplotlib carries,
so rendering never fails for want of a font; the engine string says
which face was used.
"""

from __future__ import annotations

from dataclasses import dataclass
from functools import lru_cache
from pathlib import Path


@dataclass(frozen=True)
class Family:
    name: str  # what the face is called, for the engine string
    regular: Path
    bold: Path
    italic: Path
    bold_italic: Path


def _candidates() -> list[Family]:
    out: list[Family] = []

    def family(name: str, directory: str, files: tuple[str, str, str, str]) -> None:
        d = Path(directory)
        paths = [d / f for f in files]
        if all(p.is_file() for p in paths):
            out.append(Family(name, *paths))

    liberation = (
        "LiberationSans-Regular.ttf",
        "LiberationSans-Bold.ttf",
        "LiberationSans-Italic.ttf",
        "LiberationSans-BoldItalic.ttf",
    )
    for directory in (
        "/usr/share/fonts/truetype/liberation",
        "/usr/share/fonts/truetype/liberation2",
        "/usr/share/fonts/liberation-sans",
        "/opt/homebrew/share/fonts",
    ):
        family("Liberation Sans", directory, liberation)
    family(
        "Arial",
        "/System/Library/Fonts/Supplemental",
        ("Arial.ttf", "Arial Bold.ttf", "Arial Italic.ttf", "Arial Bold Italic.ttf"),
    )
    try:
        import matplotlib

        family(
            "DejaVu Sans",
            str(Path(matplotlib.get_data_path()) / "fonts" / "ttf"),
            (
                "DejaVuSans.ttf",
                "DejaVuSans-Bold.ttf",
                "DejaVuSans-Oblique.ttf",
                "DejaVuSans-BoldOblique.ttf",
            ),
        )
    except Exception:  # noqa: BLE001 - no matplotlib means no report at all
        pass
    return out


@lru_cache(maxsize=1)
def family() -> Family:
    found = _candidates()
    if not found:
        raise RuntimeError("no sans font found for the report")
    return found[0]
