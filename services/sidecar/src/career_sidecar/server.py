"""gRPC server lifecycle: build, start, stop, graceful shutdown."""

from __future__ import annotations

import logging
from concurrent import futures

import grpc

from career.sidecar.v1 import sidecar_pb2_grpc
from career_sidecar.config import Config
from career_sidecar.embed import build_embedder
from career_sidecar.llm import build_llm
from career_sidecar.servicer import SidecarServicer

log = logging.getLogger(__name__)


def build_server(cfg: Config) -> tuple[grpc.Server, str]:
    """Construct a gRPC server, register services, and bind the listen port.
    Returns the server and the bound address (with resolved port) so tests can
    connect to an ephemeral port.
    """
    server = grpc.server(
        futures.ThreadPoolExecutor(max_workers=cfg.max_workers),
        options=[
            ("grpc.max_receive_message_length", 16 * 1024 * 1024),
            ("grpc.max_send_message_length", 16 * 1024 * 1024),
        ],
    )
    embedder = build_embedder(
        provider=cfg.embed_provider,
        ollama_url=cfg.ollama_url,
        model=cfg.ollama_embed_model,
        dimensions=cfg.embed_dimensions,
    )
    log.info(
        "sidecar embedder ready",
        extra={"provider": embedder.name, "dimensions": embedder.dimensions},
    )
    llm = build_llm(
        provider=cfg.llm_provider,
        ollama_url=cfg.ollama_url,
        model=cfg.ollama_llm_model,
        timeout_seconds=cfg.llm_timeout_seconds,
    )
    log.info("sidecar llm ready", extra={"provider": llm.name})
    sidecar_pb2_grpc.add_SidecarServiceServicer_to_server(
        SidecarServicer(embedder=embedder, llm=llm), server
    )
    bound_port = server.add_insecure_port(cfg.addr)
    bound_addr = _rewrite_port(cfg.addr, bound_port)
    return server, bound_addr


def _rewrite_port(addr: str, port: int) -> str:
    """Replace the port in `addr` with the actually-bound port. Handles both
    IPv4 (`host:port`) and IPv6 (`[::]:port`) forms.
    """
    if addr.startswith("["):
        host, _, _ = addr.rpartition(":")
        return f"{host}:{port}"
    host, _, _ = addr.rpartition(":")
    return f"{host}:{port}"
