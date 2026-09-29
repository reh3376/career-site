import datetime

from buf.validate import validate_pb2 as _validate_pb2
from career.v1 import options_pb2 as _options_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class GetMeetingOptionsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetMeetingOptionsResponse(_message.Message):
    __slots__ = ("duration_minutes", "zone", "zone_label", "horizon_days", "lead_hours", "hours_summary", "available", "unavailable_reason")
    DURATION_MINUTES_FIELD_NUMBER: _ClassVar[int]
    ZONE_FIELD_NUMBER: _ClassVar[int]
    ZONE_LABEL_FIELD_NUMBER: _ClassVar[int]
    HORIZON_DAYS_FIELD_NUMBER: _ClassVar[int]
    LEAD_HOURS_FIELD_NUMBER: _ClassVar[int]
    HOURS_SUMMARY_FIELD_NUMBER: _ClassVar[int]
    AVAILABLE_FIELD_NUMBER: _ClassVar[int]
    UNAVAILABLE_REASON_FIELD_NUMBER: _ClassVar[int]
    duration_minutes: _containers.RepeatedScalarFieldContainer[int]
    zone: str
    zone_label: str
    horizon_days: int
    lead_hours: int
    hours_summary: str
    available: bool
    unavailable_reason: str
    def __init__(self, duration_minutes: _Optional[_Iterable[int]] = ..., zone: _Optional[str] = ..., zone_label: _Optional[str] = ..., horizon_days: _Optional[int] = ..., lead_hours: _Optional[int] = ..., hours_summary: _Optional[str] = ..., available: _Optional[bool] = ..., unavailable_reason: _Optional[str] = ...) -> None: ...

class GetAvailabilityRequest(_message.Message):
    __slots__ = ("duration_minutes", "to")
    DURATION_MINUTES_FIELD_NUMBER: _ClassVar[int]
    FROM_FIELD_NUMBER: _ClassVar[int]
    TO_FIELD_NUMBER: _ClassVar[int]
    duration_minutes: int
    to: _timestamp_pb2.Timestamp
    def __init__(self, duration_minutes: _Optional[int] = ..., to: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., **kwargs) -> None: ...

class MeetingSlot(_message.Message):
    __slots__ = ("start", "end")
    START_FIELD_NUMBER: _ClassVar[int]
    END_FIELD_NUMBER: _ClassVar[int]
    start: _timestamp_pb2.Timestamp
    end: _timestamp_pb2.Timestamp
    def __init__(self, start: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., end: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class GetAvailabilityResponse(_message.Message):
    __slots__ = ("slots", "zone", "available", "unavailable_reason")
    SLOTS_FIELD_NUMBER: _ClassVar[int]
    ZONE_FIELD_NUMBER: _ClassVar[int]
    AVAILABLE_FIELD_NUMBER: _ClassVar[int]
    UNAVAILABLE_REASON_FIELD_NUMBER: _ClassVar[int]
    slots: _containers.RepeatedCompositeFieldContainer[MeetingSlot]
    zone: str
    available: bool
    unavailable_reason: str
    def __init__(self, slots: _Optional[_Iterable[_Union[MeetingSlot, _Mapping]]] = ..., zone: _Optional[str] = ..., available: _Optional[bool] = ..., unavailable_reason: _Optional[str] = ...) -> None: ...

class BookMeetingRequest(_message.Message):
    __slots__ = ("start", "duration_minutes", "topic", "contact_preference")
    START_FIELD_NUMBER: _ClassVar[int]
    DURATION_MINUTES_FIELD_NUMBER: _ClassVar[int]
    TOPIC_FIELD_NUMBER: _ClassVar[int]
    CONTACT_PREFERENCE_FIELD_NUMBER: _ClassVar[int]
    start: _timestamp_pb2.Timestamp
    duration_minutes: int
    topic: str
    contact_preference: str
    def __init__(self, start: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., duration_minutes: _Optional[int] = ..., topic: _Optional[str] = ..., contact_preference: _Optional[str] = ...) -> None: ...

class Meeting(_message.Message):
    __slots__ = ("id", "start", "end", "duration_minutes", "topic", "zone", "ics_url", "cancelled_at")
    ID_FIELD_NUMBER: _ClassVar[int]
    START_FIELD_NUMBER: _ClassVar[int]
    END_FIELD_NUMBER: _ClassVar[int]
    DURATION_MINUTES_FIELD_NUMBER: _ClassVar[int]
    TOPIC_FIELD_NUMBER: _ClassVar[int]
    ZONE_FIELD_NUMBER: _ClassVar[int]
    ICS_URL_FIELD_NUMBER: _ClassVar[int]
    CANCELLED_AT_FIELD_NUMBER: _ClassVar[int]
    id: int
    start: _timestamp_pb2.Timestamp
    end: _timestamp_pb2.Timestamp
    duration_minutes: int
    topic: str
    zone: str
    ics_url: str
    cancelled_at: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[int] = ..., start: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., end: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., duration_minutes: _Optional[int] = ..., topic: _Optional[str] = ..., zone: _Optional[str] = ..., ics_url: _Optional[str] = ..., cancelled_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class BookMeetingResponse(_message.Message):
    __slots__ = ("meeting",)
    MEETING_FIELD_NUMBER: _ClassVar[int]
    meeting: Meeting
    def __init__(self, meeting: _Optional[_Union[Meeting, _Mapping]] = ...) -> None: ...

class ListMyMeetingsRequest(_message.Message):
    __slots__ = ("include_past",)
    INCLUDE_PAST_FIELD_NUMBER: _ClassVar[int]
    include_past: bool
    def __init__(self, include_past: _Optional[bool] = ...) -> None: ...

class ListMyMeetingsResponse(_message.Message):
    __slots__ = ("meetings",)
    MEETINGS_FIELD_NUMBER: _ClassVar[int]
    meetings: _containers.RepeatedCompositeFieldContainer[Meeting]
    def __init__(self, meetings: _Optional[_Iterable[_Union[Meeting, _Mapping]]] = ...) -> None: ...

class CancelMeetingRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: int
    def __init__(self, id: _Optional[int] = ...) -> None: ...

class CancelMeetingResponse(_message.Message):
    __slots__ = ("meeting",)
    MEETING_FIELD_NUMBER: _ClassVar[int]
    meeting: Meeting
    def __init__(self, meeting: _Optional[_Union[Meeting, _Mapping]] = ...) -> None: ...
