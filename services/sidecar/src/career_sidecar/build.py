"""Build-time metadata. VERSION is set at container build; otherwise reads from
the installed package metadata so tests and local runs report something real.
"""

from __future__ import annotations

import os
from importlib.metadata import PackageNotFoundError, version


def _resolve_version() -> str:
    if v := os.environ.get("SIDECAR_VERSION"):
        return v
    try:
        return version("career-sidecar")
    except PackageNotFoundError:
        return "0.0.0-dev"


VERSION = _resolve_version()
