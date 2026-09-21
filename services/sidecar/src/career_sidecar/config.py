"""Environment-driven runtime configuration."""

from __future__ import annotations

import os
from dataclasses import dataclass


@dataclass(frozen=True)
class Config:
    addr: str
    max_workers: int
    shutdown_grace_seconds: int
    # Ask Roger Phase 4 groundwork — embedder wiring.
    # provider = "ollama" hits an Ollama HTTP server (production path);
    # provider = "stub" returns a deterministic pseudo-embedding (dev
    # and CI, no external dep). Anything else raises at boot so a typo
    # doesn't silently fall through to the stub.
    embed_provider: str
    ollama_url: str
    ollama_embed_model: str
    embed_dimensions: int
    # LLM gateway for Generate. "ollama" hits the same Ollama server as
    # the embedder (model = ollama_llm_model); "stub" returns a canned
    # completion. Timeout is generous because CPU inference of a
    # 14B model can take minutes for a page of output.
    llm_provider: str
    # The LLM may live on a different Ollama than the embedder (for
    # example the owner's workstation over a tailnet, while embeddings
    # stay on the box's CPU). Defaults to ollama_url.
    ollama_llm_url: str
    ollama_llm_model: str
    # Bearer token for a hosted Ollama endpoint (ollama.com); empty for
    # a local or tailnet server. Never logged.
    ollama_api_key: str
    llm_timeout_seconds: int
    # Context window requested on every Ollama chat call. The server
    # default silently truncates; size this to the judgment prompt
    # (~15k tokens for a 12-requirement JD) and to the box's RAM.
    llm_num_ctx: int

    @classmethod
    def from_env(cls) -> Config:
        return cls(
            addr=os.environ.get("SIDECAR_ADDR", "[::]:50051"),
            max_workers=int(os.environ.get("SIDECAR_MAX_WORKERS", "8")),
            shutdown_grace_seconds=int(os.environ.get("SIDECAR_SHUTDOWN_GRACE_SECONDS", "5")),
            embed_provider=os.environ.get("SIDECAR_EMBED_PROVIDER", "stub"),
            ollama_url=os.environ.get("OLLAMA_URL", "http://ollama:11434"),
            ollama_embed_model=os.environ.get("OLLAMA_EMBED_MODEL", "nomic-embed-text"),
            # Locked to 768 because the DB schema fixes the vector
            # dimension at that width (migration 00011). Change together
            # with a dual-index migration if a different model warrants it.
            embed_dimensions=int(os.environ.get("SIDECAR_EMBED_DIMENSIONS", "768")),
            llm_provider=os.environ.get("SIDECAR_LLM_PROVIDER", "stub"),
            ollama_llm_url=os.environ.get("OLLAMA_LLM_URL")
            or os.environ.get("OLLAMA_URL", "http://ollama:11434"),
            ollama_llm_model=os.environ.get("OLLAMA_LLM_MODEL", "qwen3:14b"),
            ollama_api_key=os.environ.get("OLLAMA_API_KEY", ""),
            llm_timeout_seconds=int(os.environ.get("SIDECAR_LLM_TIMEOUT_SECONDS", "600")),
            llm_num_ctx=int(os.environ.get("SIDECAR_LLM_NUM_CTX", "16384")),
        )
