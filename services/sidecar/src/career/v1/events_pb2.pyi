from buf.validate import validate_pb2 as _validate_pb2
from career.v1 import options_pb2 as _options_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class BrowserEvent(_message.Message):
    __slots__ = ("event_id", "name", "client_ts_ms", "path", "referrer", "ui_mode", "props_json")
    EVENT_ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    CLIENT_TS_MS_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    REFERRER_FIELD_NUMBER: _ClassVar[int]
    UI_MODE_FIELD_NUMBER: _ClassVar[int]
    PROPS_JSON_FIELD_NUMBER: _ClassVar[int]
    event_id: str
    name: str
    client_ts_ms: int
    path: str
    referrer: str
    ui_mode: str
    props_json: str
    def __init__(self, event_id: _Optional[str] = ..., name: _Optional[str] = ..., client_ts_ms: _Optional[int] = ..., path: _Optional[str] = ..., referrer: _Optional[str] = ..., ui_mode: _Optional[str] = ..., props_json: _Optional[str] = ...) -> None: ...

class RecordRequest(_message.Message):
    __slots__ = ("events",)
    EVENTS_FIELD_NUMBER: _ClassVar[int]
    events: _containers.RepeatedCompositeFieldContainer[BrowserEvent]
    def __init__(self, events: _Optional[_Iterable[_Union[BrowserEvent, _Mapping]]] = ...) -> None: ...

class RecordResponse(_message.Message):
    __slots__ = ("accepted",)
    ACCEPTED_FIELD_NUMBER: _ClassVar[int]
    accepted: int
    def __init__(self, accepted: _Optional[int] = ...) -> None: ...
