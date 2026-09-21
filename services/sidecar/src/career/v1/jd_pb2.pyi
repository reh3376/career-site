import datetime

from buf.validate import validate_pb2 as _validate_pb2
from career.v1 import options_pb2 as _options_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Mapping as _Mapping
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

class SubmitJdRequest(_message.Message):
    __slots__ = ("jd_text", "source", "role_hint", "employer_hint", "contact_email")
    JD_TEXT_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    ROLE_HINT_FIELD_NUMBER: _ClassVar[int]
    EMPLOYER_HINT_FIELD_NUMBER: _ClassVar[int]
    CONTACT_EMAIL_FIELD_NUMBER: _ClassVar[int]
    jd_text: str
    source: JdSource
    role_hint: str
    employer_hint: str
    contact_email: str
    def __init__(self, jd_text: _Optional[str] = ..., source: _Optional[_Union[JdSource, str]] = ..., role_hint: _Optional[str] = ..., employer_hint: _Optional[str] = ..., contact_email: _Optional[str] = ...) -> None: ...

class SubmitJdResponse(_message.Message):
    __slots__ = ("submission_id", "status", "message")
    SUBMISSION_ID_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    submission_id: str
    status: JdStatus
    message: str
    def __init__(self, submission_id: _Optional[str] = ..., status: _Optional[_Union[JdStatus, str]] = ..., message: _Optional[str] = ...) -> None: ...

class GetJdResultRequest(_message.Message):
    __slots__ = ("submission_id",)
    SUBMISSION_ID_FIELD_NUMBER: _ClassVar[int]
    submission_id: str
    def __init__(self, submission_id: _Optional[str] = ...) -> None: ...

class GetJdResultResponse(_message.Message):
    __slots__ = ("status", "match_score", "generated_resume_url", "error_message", "created_at", "completed_at")
    STATUS_FIELD_NUMBER: _ClassVar[int]
    MATCH_SCORE_FIELD_NUMBER: _ClassVar[int]
    GENERATED_RESUME_URL_FIELD_NUMBER: _ClassVar[int]
    ERROR_MESSAGE_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    COMPLETED_AT_FIELD_NUMBER: _ClassVar[int]
    status: JdStatus
    match_score: float
    generated_resume_url: str
    error_message: str
    created_at: _timestamp_pb2.Timestamp
    completed_at: _timestamp_pb2.Timestamp
    def __init__(self, status: _Optional[_Union[JdStatus, str]] = ..., match_score: _Optional[float] = ..., generated_resume_url: _Optional[str] = ..., error_message: _Optional[str] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., completed_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...
