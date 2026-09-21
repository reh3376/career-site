"""Embedding providers for the sidecar's Embed RPC.

Two providers ship in this module: ``OllamaEmbedder`` (production path,
HTTP POST to a running Ollama server) and ``StubEmbedder`` (dev + CI
path, deterministic pseudo-embedding derived from SHA-256 of the input
text so tests don't need a network round-trip).

Both implement ``Embedder`` — a tiny protocol with one method that
takes a list of strings and returns a list of float lists of the
configured dimension. The servicer picks between them at boot based
on ``Config.embed_provider``.
"""

from __future__ import annotations

import hashlib
import json
import math
import urllib.error
import urllib.request
from dataclasses import dataclass
from typing import Protocol

PURPOSE_DOCUMENT = "document"
PURPOSE_QUERY = "query"

# Task prefixes nomic-embed-text was trained with. Embedding a corpus
# chunk and a question with the same prefix (or none) measurably
# flattens the similarity distribution; the asymmetric prefixes are
# what give retrieval its dynamic range. Keyed by model-name prefix so
# a non-nomic model gets no prefix.
_TASK_PREFIXES: dict[str, dict[str, str]] = {
    "nomic-embed-text": {
        PURPOSE_DOCUMENT: "search_document: ",
        PURPOSE_QUERY: "search_query: ",
    },
}

# Bumped whenever the text handed to the model changes for the same
# model (prefixes, normalisation). It is part of the embedder name the
# api records per chunk, so a recipe change makes every existing vector
# stale and the embed sweep re-embeds the corpus.
_RECIPE = "p1"


class Embedder(Protocol):
    """One method — batch embed. Both implementations honour a
    fixed dimension so the sidecar can advertise it back to callers
    (matches the DB's vector(N) column width — see migration 00011).
    `purpose` is PURPOSE_DOCUMENT (corpus chunk) or PURPOSE_QUERY
    (a question or JD at retrieval time).
    """

    name: str
    dimensions: int

    def embed(self, texts: list[str], purpose: str = PURPOSE_DOCUMENT) -> list[list[float]]:
        """Embed a batch of texts; one vector per input, in order."""
        ...


def task_prefix(model: str, purpose: str) -> str:
    """Prefix to prepend for (model, purpose); empty when the model has none."""
    for model_prefix, table in _TASK_PREFIXES.items():
        if model.startswith(model_prefix):
            return table.get(purpose, table[PURPOSE_DOCUMENT])
    return ""


@dataclass
class OllamaEmbedder:
    """POSTs to Ollama's /api/embeddings endpoint, one call per text.

    Ollama has a batch endpoint at ``/api/embed`` in newer versions but
    the single-text ``/api/embeddings`` endpoint is available on every
    release we might see in prod, so we call it in a loop. A future
    optimisation switches to the batch path when we know the server is
    new enough.
    """

    base_url: str
    model: str
    dimensions: int
    name: str = "ollama"

    def __post_init__(self) -> None:
        # The api records this per chunk and re-embeds anything whose
        # recorded name differs, so the model AND the text recipe must be
        # part of the name: switching models, or changing the prefixes,
        # has to look like a new embedder, not the same one.
        recipe = _RECIPE if task_prefix(self.model, PURPOSE_DOCUMENT) else "raw"
        self.name = f"ollama:{self.model}#{recipe}"

    def embed(self, texts: list[str], purpose: str = PURPOSE_DOCUMENT) -> list[list[float]]:
        prefix = task_prefix(self.model, purpose)
        out: list[list[float]] = []
        for text in texts:
            payload = json.dumps({"model": self.model, "prompt": prefix + text}).encode()
            req = urllib.request.Request(
                f"{self.base_url.rstrip('/')}/api/embeddings",
                data=payload,
                headers={"Content-Type": "application/json"},
                method="POST",
            )
            try:
                with urllib.request.urlopen(req, timeout=30) as resp:
                    body = json.loads(resp.read().decode("utf-8"))
            except urllib.error.URLError as e:
                raise RuntimeError(f"ollama embed failed: {e}") from e
            emb = body.get("embedding")
            if not isinstance(emb, list) or len(emb) != self.dimensions:
                raise RuntimeError(
                    f"ollama returned unexpected embedding shape "
                    f"(got {len(emb) if isinstance(emb, list) else type(emb).__name__}, "
                    f"expected {self.dimensions})"
                )
            out.append([float(x) for x in emb])
        return out


@dataclass
class StubEmbedder:
    """Deterministic pseudo-embedding for dev + CI.

    Derives ``dimensions`` floats in [-1, 1] from a rolling SHA-256 of
    the input text. Same input → same output, always. NOT semantically
    meaningful — retrieval against a corpus embedded with this provider
    will return arbitrary chunks. Its job is to keep the interface
    exercised (proto shapes, RPC handler, DB writes) without requiring
    a live LLM.

    Vectors are L2-normalised so cosine distance behaves sanely (all
    hits fall in a small range instead of collapsing to identical
    similarity scores).
    """

    dimensions: int
    name: str = "stub"

    def embed(self, texts: list[str], purpose: str = PURPOSE_DOCUMENT) -> list[list[float]]:
        out: list[list[float]] = []
        for text in texts:
            vec = _pseudo_vector(text, self.dimensions)
            out.append(_l2_normalise(vec))
        return out


def _pseudo_vector(text: str, dim: int) -> list[float]:
    """Roll SHA-256 over the input, take groups of 4 bytes as int32,
    map to [-1, 1]. Deterministic and dependency-free.
    """
    values: list[float] = []
    counter = 0
    while len(values) < dim:
        digest = hashlib.sha256(f"{counter}:{text}".encode()).digest()
        # 32 bytes → 8 int32s → 8 floats in [-1, 1]
        for i in range(0, 32, 4):
            n = int.from_bytes(digest[i : i + 4], "little", signed=True)
            values.append(n / 2_147_483_648.0)
            if len(values) >= dim:
                break
        counter += 1
    return values


def _l2_normalise(v: list[float]) -> list[float]:
    norm = math.sqrt(sum(x * x for x in v))
    if norm == 0:
        return v
    return [x / norm for x in v]


def build_embedder(provider: str, ollama_url: str, model: str, dimensions: int) -> Embedder:
    """Construct the configured embedder or raise. Called once at boot."""
    if provider == "ollama":
        return OllamaEmbedder(base_url=ollama_url, model=model, dimensions=dimensions)
    if provider == "stub":
        return StubEmbedder(dimensions=dimensions)
    raise RuntimeError(f"unknown SIDECAR_EMBED_PROVIDER: {provider!r} (want 'ollama' or 'stub')")
