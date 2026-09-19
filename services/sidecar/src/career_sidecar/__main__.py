"""Entry point: `python -m career_sidecar` or `career-sidecar`."""

from __future__ import annotations

import logging
import signal
import sys

from career_sidecar.build import VERSION
from career_sidecar.config import Config
from career_sidecar.server import build_server


def main() -> int:
    logging.basicConfig(
        level=logging.INFO,
        format='{"level":"%(levelname)s","logger":"%(name)s","msg":"%(message)s"}',
    )
    log = logging.getLogger("career_sidecar")

    cfg = Config.from_env()
    server, bound = build_server(cfg)

    log.info("sidecar listening addr=%s version=%s", bound, VERSION)
    server.start()

    def _stop(signum: int, _frame) -> None:
        log.info("shutdown signal received signum=%d", signum)
        server.stop(cfg.shutdown_grace_seconds).wait()

    signal.signal(signal.SIGINT, _stop)
    signal.signal(signal.SIGTERM, _stop)

    server.wait_for_termination()
    log.info("shutdown complete")
    return 0


if __name__ == "__main__":
    sys.exit(main())
