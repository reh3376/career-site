"""Environment-driven runtime configuration."""

from __future__ import annotations

import os
from dataclasses import dataclass


@dataclass(frozen=True)
class Config:
    addr: str
    max_workers: int
    shutdown_grace_seconds: int

    @classmethod
    def from_env(cls) -> Config:
        return cls(
            addr=os.environ.get("SIDECAR_ADDR", "[::]:50051"),
            max_workers=int(os.environ.get("SIDECAR_MAX_WORKERS", "8")),
            shutdown_grace_seconds=int(os.environ.get("SIDECAR_SHUTDOWN_GRACE_SECONDS", "5")),
        )
