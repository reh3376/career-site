"""SidecarService implementation. Phase 4 groundwork: Embed is live
(behind a pluggable provider — Ollama in prod, stub in dev/CI).
Rerank / Classify / RunJob / GetJob still return UNIMPLEMENTED.
"""

from __future__ import annotations

import time

import grpc

from career.sidecar.v1 import sidecar_pb2, sidecar_pb2_grpc
from career_sidecar.build import VERSION
from career_sidecar.embed import PURPOSE_DOCUMENT, PURPOSE_QUERY, Embedder
from career_sidecar.llm import LLM

_PURPOSE_BY_ENUM = {
    sidecar_pb2.EMBED_PURPOSE_QUERY: PURPOSE_QUERY,
    sidecar_pb2.EMBED_PURPOSE_DOCUMENT: PURPOSE_DOCUMENT,
}


class SidecarServicer(sidecar_pb2_grpc.SidecarServiceServicer):
    def __init__(self, embedder: Embedder, llm: LLM | None = None) -> None:
        # Providers are injected so tests can swap in fakes and prod
        # picks Ollama vs stub via config. See build_embedder() /
        # build_llm().
        self._embedder = embedder
        self._llm = llm

    def Health(
        self,
        request: sidecar_pb2.HealthRequest,
        context: grpc.ServicerContext,
    ) -> sidecar_pb2.HealthResponse:
        return sidecar_pb2.HealthResponse(
            ready=True,
            embedder_ready=self._embedder is not None,
            reranker_ready=False,
            storage_ready=False,
            version=VERSION,
            llm_ready=self._llm is not None,
            llm_provider=self._llm.name if self._llm is not None else "",
        )

    def Generate(
        self,
        request: sidecar_pb2.GenerateRequest,
        context: grpc.ServicerContext,
    ) -> sidecar_pb2.GenerateResponse:
        if self._llm is None:
            context.set_code(grpc.StatusCode.UNAVAILABLE)
            context.set_details("no LLM provider configured")
            return sidecar_pb2.GenerateResponse()
        if not request.user.strip():
            context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
            context.set_details("user is required")
            return sidecar_pb2.GenerateResponse()
        started = time.monotonic()
        try:
            res = self._llm.generate(
                system=request.system,
                user=request.user,
                max_tokens=request.max_tokens,
                temperature=request.temperature,
                json_mode=request.json,
                json_schema=request.json_schema,
            )
        except Exception as e:  # noqa: BLE001 — propagate error to gRPC
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(f"generate failed: {e}")
            return sidecar_pb2.GenerateResponse()
        return sidecar_pb2.GenerateResponse(
            text=res.text,
            model=res.model,
            prompt_tokens=res.prompt_tokens,
            completion_tokens=res.completion_tokens,
            latency_ms=int((time.monotonic() - started) * 1000),
            finish_reason=res.finish_reason,
        )

    def Embed(
        self,
        request: sidecar_pb2.EmbedRequest,
        context: grpc.ServicerContext,
    ) -> sidecar_pb2.EmbedResponse:
        texts = list(request.texts)
        if not texts:
            context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
            context.set_details("texts is required and must not be empty")
            return sidecar_pb2.EmbedResponse()
        purpose = _PURPOSE_BY_ENUM.get(request.purpose, PURPOSE_DOCUMENT)
        try:
            vectors = self._embedder.embed(texts, purpose)
        except Exception as e:  # noqa: BLE001 — propagate error to gRPC
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(f"embed failed: {e}")
            return sidecar_pb2.EmbedResponse()
        return sidecar_pb2.EmbedResponse(
            embeddings=[sidecar_pb2.Embedding(values=v) for v in vectors],
            model=self._embedder.name,
            dimensions=self._embedder.dimensions,
        )

    def Rerank(self, request, context):
        _unimplemented(context, "Rerank lands in Phase 4 proper.")

    def Classify(self, request, context):
        _unimplemented(context, "Classify lands in Phase 4 proper.")

    def RunJob(self, request, context):
        _unimplemented(context, "RunJob lands in Phase 2 (content ingestion CLI wiring).")

    def GetJob(self, request, context):
        _unimplemented(context, "GetJob lands in Phase 2 (content ingestion CLI wiring).")


def _unimplemented(context: grpc.ServicerContext, message: str) -> None:
    context.set_code(grpc.StatusCode.UNIMPLEMENTED)
    context.set_details(message)
    raise NotImplementedError(message)
