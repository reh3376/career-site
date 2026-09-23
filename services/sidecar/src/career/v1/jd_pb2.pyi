import datetime

from buf.validate import validate_pb2 as _validate_pb2
from career.v1 import options_pb2 as _options_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class JdSource(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    JD_SOURCE_UNSPECIFIED: _ClassVar[JdSource]
    JD_SOURCE_PASTE: _ClassVar[JdSource]
    JD_SOURCE_PDF: _ClassVar[JdSource]
    JD_SOURCE_TEXT_UPLOAD: _ClassVar[JdSource]

class JdStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    JD_STATUS_UNSPECIFIED: _ClassVar[JdStatus]
    JD_STATUS_RECEIVED: _ClassVar[JdStatus]
    JD_STATUS_SCORING: _ClassVar[JdStatus]
    JD_STATUS_BELOW_THRESHOLD: _ClassVar[JdStatus]
    JD_STATUS_GENERATING: _ClassVar[JdStatus]
    JD_STATUS_READY: _ClassVar[JdStatus]
    JD_STATUS_FAILED: _ClassVar[JdStatus]
    JD_STATUS_NOT_A_POSTING: _ClassVar[JdStatus]
JD_SOURCE_UNSPECIFIED: JdSource
JD_SOURCE_PASTE: JdSource
JD_SOURCE_PDF: JdSource
JD_SOURCE_TEXT_UPLOAD: JdSource
JD_STATUS_UNSPECIFIED: JdStatus
JD_STATUS_RECEIVED: JdStatus
JD_STATUS_SCORING: JdStatus
JD_STATUS_BELOW_THRESHOLD: JdStatus
JD_STATUS_GENERATING: JdStatus
JD_STATUS_READY: JdStatus
JD_STATUS_FAILED: JdStatus
JD_STATUS_NOT_A_POSTING: JdStatus

class SubmitJdRequest(_message.Message):
    __slots__ = ("jd_text", "source", "role_hint", "employer_hint", "contact_email", "apply_url")
    JD_TEXT_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    ROLE_HINT_FIELD_NUMBER: _ClassVar[int]
    EMPLOYER_HINT_FIELD_NUMBER: _ClassVar[int]
    CONTACT_EMAIL_FIELD_NUMBER: _ClassVar[int]
    APPLY_URL_FIELD_NUMBER: _ClassVar[int]
    jd_text: str
    source: JdSource
    role_hint: str
    employer_hint: str
    contact_email: str
    apply_url: str
    def __init__(self, jd_text: _Optional[str] = ..., source: _Optional[_Union[JdSource, str]] = ..., role_hint: _Optional[str] = ..., employer_hint: _Optional[str] = ..., contact_email: _Optional[str] = ..., apply_url: _Optional[str] = ...) -> None: ...

class SubmitJdResponse(_message.Message):
    __slots__ = ("submission_id", "status", "message", "result_token")
    SUBMISSION_ID_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    RESULT_TOKEN_FIELD_NUMBER: _ClassVar[int]
    submission_id: str
    status: JdStatus
    message: str
    result_token: str
    def __init__(self, submission_id: _Optional[str] = ..., status: _Optional[_Union[JdStatus, str]] = ..., message: _Optional[str] = ..., result_token: _Optional[str] = ...) -> None: ...

class GetJdResultRequest(_message.Message):
    __slots__ = ("submission_id", "result_token")
    SUBMISSION_ID_FIELD_NUMBER: _ClassVar[int]
    RESULT_TOKEN_FIELD_NUMBER: _ClassVar[int]
    submission_id: str
    result_token: str
    def __init__(self, submission_id: _Optional[str] = ..., result_token: _Optional[str] = ...) -> None: ...

class GetJdResultResponse(_message.Message):
    __slots__ = ("status", "match_score", "generated_resume_url", "error_message", "created_at", "completed_at", "resume_markdown", "verdicts", "met_count", "partial_count", "unmet_count", "match_threshold", "progress_pct", "progress_stage", "fit_category")
    STATUS_FIELD_NUMBER: _ClassVar[int]
    MATCH_SCORE_FIELD_NUMBER: _ClassVar[int]
    GENERATED_RESUME_URL_FIELD_NUMBER: _ClassVar[int]
    ERROR_MESSAGE_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    COMPLETED_AT_FIELD_NUMBER: _ClassVar[int]
    RESUME_MARKDOWN_FIELD_NUMBER: _ClassVar[int]
    VERDICTS_FIELD_NUMBER: _ClassVar[int]
    MET_COUNT_FIELD_NUMBER: _ClassVar[int]
    PARTIAL_COUNT_FIELD_NUMBER: _ClassVar[int]
    UNMET_COUNT_FIELD_NUMBER: _ClassVar[int]
    MATCH_THRESHOLD_FIELD_NUMBER: _ClassVar[int]
    PROGRESS_PCT_FIELD_NUMBER: _ClassVar[int]
    PROGRESS_STAGE_FIELD_NUMBER: _ClassVar[int]
    FIT_CATEGORY_FIELD_NUMBER: _ClassVar[int]
    status: JdStatus
    match_score: float
    generated_resume_url: str
    error_message: str
    created_at: _timestamp_pb2.Timestamp
    completed_at: _timestamp_pb2.Timestamp
    resume_markdown: str
    verdicts: _containers.RepeatedCompositeFieldContainer[RequirementVerdict]
    met_count: int
    partial_count: int
    unmet_count: int
    match_threshold: float
    progress_pct: int
    progress_stage: str
    fit_category: str
    def __init__(self, status: _Optional[_Union[JdStatus, str]] = ..., match_score: _Optional[float] = ..., generated_resume_url: _Optional[str] = ..., error_message: _Optional[str] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., completed_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., resume_markdown: _Optional[str] = ..., verdicts: _Optional[_Iterable[_Union[RequirementVerdict, _Mapping]]] = ..., met_count: _Optional[int] = ..., partial_count: _Optional[int] = ..., unmet_count: _Optional[int] = ..., match_threshold: _Optional[float] = ..., progress_pct: _Optional[int] = ..., progress_stage: _Optional[str] = ..., fit_category: _Optional[str] = ...) -> None: ...

class JdFitBands(_message.Message):
    __slots__ = ("very_strong", "strong", "possible", "weak")
    VERY_STRONG_FIELD_NUMBER: _ClassVar[int]
    STRONG_FIELD_NUMBER: _ClassVar[int]
    POSSIBLE_FIELD_NUMBER: _ClassVar[int]
    WEAK_FIELD_NUMBER: _ClassVar[int]
    very_strong: float
    strong: float
    possible: float
    weak: float
    def __init__(self, very_strong: _Optional[float] = ..., strong: _Optional[float] = ..., possible: _Optional[float] = ..., weak: _Optional[float] = ...) -> None: ...

class GetJdReviewConfigRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetJdReviewConfigResponse(_message.Message):
    __slots__ = ("bands",)
    BANDS_FIELD_NUMBER: _ClassVar[int]
    bands: JdFitBands
    def __init__(self, bands: _Optional[_Union[JdFitBands, _Mapping]] = ...) -> None: ...

class ListMySubmissionsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListMySubmissionsResponse(_message.Message):
    __slots__ = ("submissions",)
    SUBMISSIONS_FIELD_NUMBER: _ClassVar[int]
    submissions: _containers.RepeatedCompositeFieldContainer[MySubmission]
    def __init__(self, submissions: _Optional[_Iterable[_Union[MySubmission, _Mapping]]] = ...) -> None: ...

class MySubmission(_message.Message):
    __slots__ = ("id", "status", "match_score", "role_hint", "employer_hint", "created_at", "completed_at", "has_resume", "fit_category", "progress_pct")
    ID_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    MATCH_SCORE_FIELD_NUMBER: _ClassVar[int]
    ROLE_HINT_FIELD_NUMBER: _ClassVar[int]
    EMPLOYER_HINT_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    COMPLETED_AT_FIELD_NUMBER: _ClassVar[int]
    HAS_RESUME_FIELD_NUMBER: _ClassVar[int]
    FIT_CATEGORY_FIELD_NUMBER: _ClassVar[int]
    PROGRESS_PCT_FIELD_NUMBER: _ClassVar[int]
    id: str
    status: JdStatus
    match_score: float
    role_hint: str
    employer_hint: str
    created_at: _timestamp_pb2.Timestamp
    completed_at: _timestamp_pb2.Timestamp
    has_resume: bool
    fit_category: str
    progress_pct: int
    def __init__(self, id: _Optional[str] = ..., status: _Optional[_Union[JdStatus, str]] = ..., match_score: _Optional[float] = ..., role_hint: _Optional[str] = ..., employer_hint: _Optional[str] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., completed_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., has_resume: _Optional[bool] = ..., fit_category: _Optional[str] = ..., progress_pct: _Optional[int] = ...) -> None: ...

class RequirementVerdict(_message.Message):
    __slots__ = ("id", "text", "category", "weight", "verdict", "rationale")
    ID_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    CATEGORY_FIELD_NUMBER: _ClassVar[int]
    WEIGHT_FIELD_NUMBER: _ClassVar[int]
    VERDICT_FIELD_NUMBER: _ClassVar[int]
    RATIONALE_FIELD_NUMBER: _ClassVar[int]
    id: str
    text: str
    category: str
    weight: int
    verdict: str
    rationale: str
    def __init__(self, id: _Optional[str] = ..., text: _Optional[str] = ..., category: _Optional[str] = ..., weight: _Optional[int] = ..., verdict: _Optional[str] = ..., rationale: _Optional[str] = ...) -> None: ...
