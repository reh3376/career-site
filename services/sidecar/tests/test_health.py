"""Smoke test: boot the server, call Health over a real gRPC channel."""

from __future__ import annotations

from collections.abc import Iterator

import grpc
import pytest

from career.sidecar.v1 import sidecar_pb2, sidecar_pb2_grpc
from career_sidecar.config import Config
from career_sidecar.server import build_server


@pytest.fixture
def running_server() -> Iterator[str]:
    cfg = Config(addr="[::]:0", max_workers=2, shutdown_grace_seconds=1)
    server, addr = build_server(cfg)
    server.start()
    try:
        # `[::]:PORT` binds all interfaces; connect via loopback.
        _, _, port = addr.rpartition(":")
        yield f"localhost:{port}"
    finally:
        server.stop(0).wait()


def test_health_returns_ready(running_server: str) -> None:
    with grpc.insecure_channel(running_server) as channel:
        stub = sidecar_pb2_grpc.SidecarServiceStub(channel)
        resp = stub.Health(sidecar_pb2.HealthRequest(), timeout=5.0)

    assert resp.ready is True
    assert resp.embedder_ready is False
    assert resp.reranker_ready is False
    assert resp.storage_ready is False
    assert resp.version != ""


def test_embed_returns_unimplemented(running_server: str) -> None:
    with grpc.insecure_channel(running_server) as channel:
        stub = sidecar_pb2_grpc.SidecarServiceStub(channel)
        with pytest.raises(grpc.RpcError) as exc_info:
            stub.Embed(sidecar_pb2.EmbedRequest(), timeout=5.0)

    assert exc_info.value.code() == grpc.StatusCode.UNIMPLEMENTED
