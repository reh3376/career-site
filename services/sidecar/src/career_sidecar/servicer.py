"""SidecarService implementation. Phase 0: only Health is functional; every
other RPC returns UNIMPLEMENTED with a message pointing at the phase that
lights it up.
"""

from __future__ import annotations

import grpc

from career.sidecar.v1 import sidecar_pb2, sidecar_pb2_grpc
from career_sidecar.build import VERSION


class SidecarServicer(sidecar_pb2_grpc.SidecarServiceServicer):
    def Health(
        self,
        request: sidecar_pb2.HealthRequest,
        context: grpc.ServicerContext,
    ) -> sidecar_pb2.HealthResponse:
        return sidecar_pb2.HealthResponse(
            ready=True,
            embedder_ready=False,
            reranker_ready=False,
            storage_ready=False,
            version=VERSION,
        )

    def Embed(self, request, context):
        _unimplemented(context, "Embed lands in Phase 4 (Ask Roger).")

    def Rerank(self, request, context):
        _unimplemented(context, "Rerank lands in Phase 4 (Ask Roger).")

    def Classify(self, request, context):
        _unimplemented(context, "Classify lands in Phase 4 (Ask Roger).")

    def RunJob(self, request, context):
        _unimplemented(context, "RunJob lands in Phase 2 (content ingestion CLI wiring).")

    def GetJob(self, request, context):
        _unimplemented(context, "GetJob lands in Phase 2 (content ingestion CLI wiring).")


def _unimplemented(context: grpc.ServicerContext, message: str) -> None:
    context.set_code(grpc.StatusCode.UNIMPLEMENTED)
    context.set_details(message)
    raise NotImplementedError(message)
