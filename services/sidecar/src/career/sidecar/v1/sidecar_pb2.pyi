import datetime

from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class EmbedPurpose(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    EMBED_PURPOSE_UNSPECIFIED: _ClassVar[EmbedPurpose]
    EMBED_PURPOSE_QUERY: _ClassVar[EmbedPurpose]
    EMBED_PURPOSE_DOCUMENT: _ClassVar[EmbedPurpose]

class Scope(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SCOPE_UNSPECIFIED: _ClassVar[Scope]
    SCOPE_IN_SCOPE: _ClassVar[Scope]
    SCOPE_OUT_OF_SCOPE: _ClassVar[Scope]
    SCOPE_INJECTION_SUSPECTED: _ClassVar[Scope]
    SCOPE_UNCERTAIN: _ClassVar[Scope]

class JobKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    JOB_KIND_UNSPECIFIED: _ClassVar[JobKind]
    JOB_KIND_CONTENT_VALIDATE: _ClassVar[JobKind]
    JOB_KIND_CONTENT_INDEX: _ClassVar[JobKind]
    JOB_KIND_INGEST_CHANGED: _ClassVar[JobKind]
    JOB_KIND_INGEST_FULL: _ClassVar[JobKind]
    JOB_KIND_ASSETS_PUSH: _ClassVar[JobKind]
    JOB_KIND_RESUME_BUILD: _ClassVar[JobKind]
    JOB_KIND_EVAL_QUICK: _ClassVar[JobKind]
    JOB_KIND_EVAL_FULL: _ClassVar[JobKind]

class JobStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    JOB_STATUS_UNSPECIFIED: _ClassVar[JobStatus]
    JOB_STATUS_QUEUED: _ClassVar[JobStatus]
    JOB_STATUS_RUNNING: _ClassVar[JobStatus]
    JOB_STATUS_SUCCEEDED: _ClassVar[JobStatus]
    JOB_STATUS_FAILED: _ClassVar[JobStatus]
EMBED_PURPOSE_UNSPECIFIED: EmbedPurpose
EMBED_PURPOSE_QUERY: EmbedPurpose
EMBED_PURPOSE_DOCUMENT: EmbedPurpose
SCOPE_UNSPECIFIED: Scope
SCOPE_IN_SCOPE: Scope
SCOPE_OUT_OF_SCOPE: Scope
SCOPE_INJECTION_SUSPECTED: Scope
SCOPE_UNCERTAIN: Scope
JOB_KIND_UNSPECIFIED: JobKind
JOB_KIND_CONTENT_VALIDATE: JobKind
JOB_KIND_CONTENT_INDEX: JobKind
JOB_KIND_INGEST_CHANGED: JobKind
JOB_KIND_INGEST_FULL: JobKind
JOB_KIND_ASSETS_PUSH: JobKind
JOB_KIND_RESUME_BUILD: JobKind
JOB_KIND_EVAL_QUICK: JobKind
JOB_KIND_EVAL_FULL: JobKind
JOB_STATUS_UNSPECIFIED: JobStatus
JOB_STATUS_QUEUED: JobStatus
JOB_STATUS_RUNNING: JobStatus
JOB_STATUS_SUCCEEDED: JobStatus
JOB_STATUS_FAILED: JobStatus

class EmbedRequest(_message.Message):
    __slots__ = ("texts", "purpose")
    TEXTS_FIELD_NUMBER: _ClassVar[int]
    PURPOSE_FIELD_NUMBER: _ClassVar[int]
    texts: _containers.RepeatedScalarFieldContainer[str]
    purpose: EmbedPurpose
    def __init__(self, texts: _Optional[_Iterable[str]] = ..., purpose: _Optional[_Union[EmbedPurpose, str]] = ...) -> None: ...

class Embedding(_message.Message):
    __slots__ = ("values",)
    VALUES_FIELD_NUMBER: _ClassVar[int]
    values: _containers.RepeatedScalarFieldContainer[float]
    def __init__(self, values: _Optional[_Iterable[float]] = ...) -> None: ...

class EmbedResponse(_message.Message):
    __slots__ = ("embeddings", "model", "dimensions")
    EMBEDDINGS_FIELD_NUMBER: _ClassVar[int]
    MODEL_FIELD_NUMBER: _ClassVar[int]
    DIMENSIONS_FIELD_NUMBER: _ClassVar[int]
    embeddings: _containers.RepeatedCompositeFieldContainer[Embedding]
    model: str
    dimensions: int
    def __init__(self, embeddings: _Optional[_Iterable[_Union[Embedding, _Mapping]]] = ..., model: _Optional[str] = ..., dimensions: _Optional[int] = ...) -> None: ...

class RerankCandidate(_message.Message):
    __slots__ = ("id", "text")
    ID_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    id: str
    text: str
    def __init__(self, id: _Optional[str] = ..., text: _Optional[str] = ...) -> None: ...

class RerankRequest(_message.Message):
    __slots__ = ("query", "candidates", "top_k")
    QUERY_FIELD_NUMBER: _ClassVar[int]
    CANDIDATES_FIELD_NUMBER: _ClassVar[int]
    TOP_K_FIELD_NUMBER: _ClassVar[int]
    query: str
    candidates: _containers.RepeatedCompositeFieldContainer[RerankCandidate]
    top_k: int
    def __init__(self, query: _Optional[str] = ..., candidates: _Optional[_Iterable[_Union[RerankCandidate, _Mapping]]] = ..., top_k: _Optional[int] = ...) -> None: ...

class RerankScore(_message.Message):
    __slots__ = ("id", "score")
    ID_FIELD_NUMBER: _ClassVar[int]
    SCORE_FIELD_NUMBER: _ClassVar[int]
    id: str
    score: float
    def __init__(self, id: _Optional[str] = ..., score: _Optional[float] = ...) -> None: ...

class RerankResponse(_message.Message):
    __slots__ = ("scores", "model")
    SCORES_FIELD_NUMBER: _ClassVar[int]
    MODEL_FIELD_NUMBER: _ClassVar[int]
    scores: _containers.RepeatedCompositeFieldContainer[RerankScore]
    model: str
    def __init__(self, scores: _Optional[_Iterable[_Union[RerankScore, _Mapping]]] = ..., model: _Optional[str] = ...) -> None: ...

class ClassifyRequest(_message.Message):
    __slots__ = ("text", "conversation_summary")
    TEXT_FIELD_NUMBER: _ClassVar[int]
    CONVERSATION_SUMMARY_FIELD_NUMBER: _ClassVar[int]
    text: str
    conversation_summary: str
    def __init__(self, text: _Optional[str] = ..., conversation_summary: _Optional[str] = ...) -> None: ...

class ClassifyResponse(_message.Message):
    __slots__ = ("scope", "confidence", "reason")
    SCOPE_FIELD_NUMBER: _ClassVar[int]
    CONFIDENCE_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    scope: Scope
    confidence: float
    reason: str
    def __init__(self, scope: _Optional[_Union[Scope, str]] = ..., confidence: _Optional[float] = ..., reason: _Optional[str] = ...) -> None: ...

class RunJobRequest(_message.Message):
    __slots__ = ("kind", "args", "requested_by")
    class ArgsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    KIND_FIELD_NUMBER: _ClassVar[int]
    ARGS_FIELD_NUMBER: _ClassVar[int]
    REQUESTED_BY_FIELD_NUMBER: _ClassVar[int]
    kind: JobKind
    args: _containers.ScalarMap[str, str]
    requested_by: str
    def __init__(self, kind: _Optional[_Union[JobKind, str]] = ..., args: _Optional[_Mapping[str, str]] = ..., requested_by: _Optional[str] = ...) -> None: ...

class RunJobResponse(_message.Message):
    __slots__ = ("job_id",)
    JOB_ID_FIELD_NUMBER: _ClassVar[int]
    job_id: str
    def __init__(self, job_id: _Optional[str] = ...) -> None: ...

class GetJobRequest(_message.Message):
    __slots__ = ("job_id",)
    JOB_ID_FIELD_NUMBER: _ClassVar[int]
    job_id: str
    def __init__(self, job_id: _Optional[str] = ...) -> None: ...

class GetJobResponse(_message.Message):
    __slots__ = ("job_id", "kind", "status", "progress_pct", "started_at", "finished_at", "summary", "log_tail")
    JOB_ID_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    PROGRESS_PCT_FIELD_NUMBER: _ClassVar[int]
    STARTED_AT_FIELD_NUMBER: _ClassVar[int]
    FINISHED_AT_FIELD_NUMBER: _ClassVar[int]
    SUMMARY_FIELD_NUMBER: _ClassVar[int]
    LOG_TAIL_FIELD_NUMBER: _ClassVar[int]
    job_id: str
    kind: JobKind
    status: JobStatus
    progress_pct: int
    started_at: _timestamp_pb2.Timestamp
    finished_at: _timestamp_pb2.Timestamp
    summary: str
    log_tail: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, job_id: _Optional[str] = ..., kind: _Optional[_Union[JobKind, str]] = ..., status: _Optional[_Union[JobStatus, str]] = ..., progress_pct: _Optional[int] = ..., started_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., finished_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., summary: _Optional[str] = ..., log_tail: _Optional[_Iterable[str]] = ...) -> None: ...

class HealthRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class HealthResponse(_message.Message):
    __slots__ = ("ready", "embedder_ready", "reranker_ready", "storage_ready", "version")
    READY_FIELD_NUMBER: _ClassVar[int]
    EMBEDDER_READY_FIELD_NUMBER: _ClassVar[int]
    RERANKER_READY_FIELD_NUMBER: _ClassVar[int]
    STORAGE_READY_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    ready: bool
    embedder_ready: bool
    reranker_ready: bool
    storage_ready: bool
    version: str
    def __init__(self, ready: _Optional[bool] = ..., embedder_ready: _Optional[bool] = ..., reranker_ready: _Optional[bool] = ..., storage_ready: _Optional[bool] = ..., version: _Optional[str] = ...) -> None: ...
