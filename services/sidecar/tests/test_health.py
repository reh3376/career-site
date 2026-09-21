"""Smoke test: boot the server, call Health + Embed over a real gRPC channel.

Phase 4 groundwork lit up Embed against a stub embedder (deterministic
pseudo-vectors); assertions here exercise the RPC + provider shape
without needing a live Ollama server.
"""

from __future__ import annotations

from collections.abc import Iterator

import grpc
import pytest

from career.sidecar.v1 import sidecar_pb2, sidecar_pb2_grpc
from career_sidecar.config import Config
from career_sidecar.server import build_server


@pytest.fixture
def running_server() -> Iterator[str]:
    cfg = Config(
        addr="[::]:0",
        max_workers=2,
        shutdown_grace_seconds=1,
        embed_provider="stub",
        ollama_url="http://ollama:11434",
        ollama_embed_model="nomic-embed-text",
        embed_dimensions=768,
    )
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
    assert resp.embedder_ready is True
    assert resp.reranker_ready is False
    assert resp.storage_ready is False
    assert resp.version != ""


def test_embed_returns_vectors_of_the_configured_dimension(running_server: str) -> None:
    with grpc.insecure_channel(running_server) as channel:
        stub = sidecar_pb2_grpc.SidecarServiceStub(channel)
        resp = stub.Embed(
            sidecar_pb2.EmbedRequest(texts=["hello", "world"]),
            timeout=5.0,
        )

    assert resp.model == "stub"
    assert resp.dimensions == 768
    assert len(resp.embeddings) == 2
    for emb in resp.embeddings:
        assert len(emb.values) == 768


def test_embed_is_deterministic(running_server: str) -> None:
    with grpc.insecure_channel(running_server) as channel:
        stub = sidecar_pb2_grpc.SidecarServiceStub(channel)
        a = stub.Embed(sidecar_pb2.EmbedRequest(texts=["hello"]), timeout=5.0)
        b = stub.Embed(sidecar_pb2.EmbedRequest(texts=["hello"]), timeout=5.0)

    assert list(a.embeddings[0].values) == list(b.embeddings[0].values)


def test_embed_rejects_empty_batch(running_server: str) -> None:
    with grpc.insecure_channel(running_server) as channel:
        stub = sidecar_pb2_grpc.SidecarServiceStub(channel)
        with pytest.raises(grpc.RpcError) as exc_info:
            stub.Embed(sidecar_pb2.EmbedRequest(), timeout=5.0)

    assert exc_info.value.code() == grpc.StatusCode.INVALID_ARGUMENT
