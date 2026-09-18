from buf.validate import validate_pb2 as _validate_pb2
from career.v1 import common_pb2 as _common_pb2
from career.v1 import options_pb2 as _options_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class RecordEventsRequest(_message.Message):
    __slots__ = ("events",)
    EVENTS_FIELD_NUMBER: _ClassVar[int]
    events: _containers.RepeatedCompositeFieldContainer[_common_pb2.ActivityEvent]
    def __init__(self, events: _Optional[_Iterable[_Union[_common_pb2.ActivityEvent, _Mapping]]] = ...) -> None: ...

class RecordEventsResponse(_message.Message):
    __slots__ = ("accepted", "duplicates")
    ACCEPTED_FIELD_NUMBER: _ClassVar[int]
    DUPLICATES_FIELD_NUMBER: _ClassVar[int]
    accepted: int
    duplicates: int
    def __init__(self, accepted: _Optional[int] = ..., duplicates: _Optional[int] = ...) -> None: ...
