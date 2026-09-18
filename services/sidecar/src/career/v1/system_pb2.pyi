import datetime

from career.v1 import options_pb2 as _options_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class GetVersionRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetVersionResponse(_message.Message):
    __slots__ = ("version", "commit", "built_at", "go_version", "persona_version")
    VERSION_FIELD_NUMBER: _ClassVar[int]
    COMMIT_FIELD_NUMBER: _ClassVar[int]
    BUILT_AT_FIELD_NUMBER: _ClassVar[int]
    GO_VERSION_FIELD_NUMBER: _ClassVar[int]
    PERSONA_VERSION_FIELD_NUMBER: _ClassVar[int]
    version: str
    commit: str
    built_at: _timestamp_pb2.Timestamp
    go_version: str
    persona_version: str
    def __init__(self, version: _Optional[str] = ..., commit: _Optional[str] = ..., built_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., go_version: _Optional[str] = ..., persona_version: _Optional[str] = ...) -> None: ...

class GetGovernanceStatusRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class FrameworkStatus(_message.Message):
    __slots__ = ("acronym", "name", "status", "gate_mode", "spec_count", "passed", "failed", "hashes_verified", "hashes_mismatched", "last_run_at")
    ACRONYM_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    GATE_MODE_FIELD_NUMBER: _ClassVar[int]
    SPEC_COUNT_FIELD_NUMBER: _ClassVar[int]
    PASSED_FIELD_NUMBER: _ClassVar[int]
    FAILED_FIELD_NUMBER: _ClassVar[int]
    HASHES_VERIFIED_FIELD_NUMBER: _ClassVar[int]
    HASHES_MISMATCHED_FIELD_NUMBER: _ClassVar[int]
    LAST_RUN_AT_FIELD_NUMBER: _ClassVar[int]
    acronym: str
    name: str
    status: str
    gate_mode: str
    spec_count: int
    passed: int
    failed: int
    hashes_verified: int
    hashes_mismatched: int
    last_run_at: _timestamp_pb2.Timestamp
    def __init__(self, acronym: _Optional[str] = ..., name: _Optional[str] = ..., status: _Optional[str] = ..., gate_mode: _Optional[str] = ..., spec_count: _Optional[int] = ..., passed: _Optional[int] = ..., failed: _Optional[int] = ..., hashes_verified: _Optional[int] = ..., hashes_mismatched: _Optional[int] = ..., last_run_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class GetGovernanceStatusResponse(_message.Message):
    __slots__ = ("frameworks", "commit", "published_at")
    FRAMEWORKS_FIELD_NUMBER: _ClassVar[int]
    COMMIT_FIELD_NUMBER: _ClassVar[int]
    PUBLISHED_AT_FIELD_NUMBER: _ClassVar[int]
    frameworks: _containers.RepeatedCompositeFieldContainer[FrameworkStatus]
    commit: str
    published_at: _timestamp_pb2.Timestamp
    def __init__(self, frameworks: _Optional[_Iterable[_Union[FrameworkStatus, _Mapping]]] = ..., commit: _Optional[str] = ..., published_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...
