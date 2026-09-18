import datetime

from buf.validate import validate_pb2 as _validate_pb2
from career.v1 import common_pb2 as _common_pb2
from career.v1 import options_pb2 as _options_pb2
from google.protobuf import field_mask_pb2 as _field_mask_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class GetMeRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetMeResponse(_message.Message):
    __slots__ = ("me",)
    ME_FIELD_NUMBER: _ClassVar[int]
    me: _common_pb2.Me
    def __init__(self, me: _Optional[_Union[_common_pb2.Me, _Mapping]] = ...) -> None: ...

class UpdateMeRequest(_message.Message):
    __slots__ = ("name", "organization", "stated_role", "profile", "update_mask")
    NAME_FIELD_NUMBER: _ClassVar[int]
    ORGANIZATION_FIELD_NUMBER: _ClassVar[int]
    STATED_ROLE_FIELD_NUMBER: _ClassVar[int]
    PROFILE_FIELD_NUMBER: _ClassVar[int]
    UPDATE_MASK_FIELD_NUMBER: _ClassVar[int]
    name: str
    organization: str
    stated_role: str
    profile: _common_pb2.Profile
    update_mask: _field_mask_pb2.FieldMask
    def __init__(self, name: _Optional[str] = ..., organization: _Optional[str] = ..., stated_role: _Optional[str] = ..., profile: _Optional[_Union[_common_pb2.Profile, _Mapping]] = ..., update_mask: _Optional[_Union[_field_mask_pb2.FieldMask, _Mapping]] = ...) -> None: ...

class UpdateMeResponse(_message.Message):
    __slots__ = ("me",)
    ME_FIELD_NUMBER: _ClassVar[int]
    me: _common_pb2.Me
    def __init__(self, me: _Optional[_Union[_common_pb2.Me, _Mapping]] = ...) -> None: ...

class SetInterestsRequest(_message.Message):
    __slots__ = ("interests", "tailoring_enabled", "questionnaire_completed")
    INTERESTS_FIELD_NUMBER: _ClassVar[int]
    TAILORING_ENABLED_FIELD_NUMBER: _ClassVar[int]
    QUESTIONNAIRE_COMPLETED_FIELD_NUMBER: _ClassVar[int]
    interests: _containers.RepeatedCompositeFieldContainer[_common_pb2.TrackInterest]
    tailoring_enabled: bool
    questionnaire_completed: bool
    def __init__(self, interests: _Optional[_Iterable[_Union[_common_pb2.TrackInterest, _Mapping]]] = ..., tailoring_enabled: _Optional[bool] = ..., questionnaire_completed: _Optional[bool] = ...) -> None: ...

class SetInterestsResponse(_message.Message):
    __slots__ = ("me",)
    ME_FIELD_NUMBER: _ClassVar[int]
    me: _common_pb2.Me
    def __init__(self, me: _Optional[_Union[_common_pb2.Me, _Mapping]] = ...) -> None: ...

class GetHistoryRequest(_message.Message):
    __slots__ = ("kind", "page")
    KIND_FIELD_NUMBER: _ClassVar[int]
    PAGE_FIELD_NUMBER: _ClassVar[int]
    kind: _common_pb2.ActivityEvent.Kind
    page: _common_pb2.PageRequest
    def __init__(self, kind: _Optional[_Union[_common_pb2.ActivityEvent.Kind, str]] = ..., page: _Optional[_Union[_common_pb2.PageRequest, _Mapping]] = ...) -> None: ...

class GetHistoryResponse(_message.Message):
    __slots__ = ("events", "page")
    EVENTS_FIELD_NUMBER: _ClassVar[int]
    PAGE_FIELD_NUMBER: _ClassVar[int]
    events: _containers.RepeatedCompositeFieldContainer[_common_pb2.ActivityEvent]
    page: _common_pb2.PageResponse
    def __init__(self, events: _Optional[_Iterable[_Union[_common_pb2.ActivityEvent, _Mapping]]] = ..., page: _Optional[_Union[_common_pb2.PageResponse, _Mapping]] = ...) -> None: ...

class ListSavedRequest(_message.Message):
    __slots__ = ("page",)
    PAGE_FIELD_NUMBER: _ClassVar[int]
    page: _common_pb2.PageRequest
    def __init__(self, page: _Optional[_Union[_common_pb2.PageRequest, _Mapping]] = ...) -> None: ...

class SavedItem(_message.Message):
    __slots__ = ("content", "note", "saved_at")
    CONTENT_FIELD_NUMBER: _ClassVar[int]
    NOTE_FIELD_NUMBER: _ClassVar[int]
    SAVED_AT_FIELD_NUMBER: _ClassVar[int]
    content: _common_pb2.ContentSummary
    note: str
    saved_at: _timestamp_pb2.Timestamp
    def __init__(self, content: _Optional[_Union[_common_pb2.ContentSummary, _Mapping]] = ..., note: _Optional[str] = ..., saved_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ListSavedResponse(_message.Message):
    __slots__ = ("items", "page")
    ITEMS_FIELD_NUMBER: _ClassVar[int]
    PAGE_FIELD_NUMBER: _ClassVar[int]
    items: _containers.RepeatedCompositeFieldContainer[SavedItem]
    page: _common_pb2.PageResponse
    def __init__(self, items: _Optional[_Iterable[_Union[SavedItem, _Mapping]]] = ..., page: _Optional[_Union[_common_pb2.PageResponse, _Mapping]] = ...) -> None: ...

class SaveItemRequest(_message.Message):
    __slots__ = ("content_id", "note")
    CONTENT_ID_FIELD_NUMBER: _ClassVar[int]
    NOTE_FIELD_NUMBER: _ClassVar[int]
    content_id: str
    note: str
    def __init__(self, content_id: _Optional[str] = ..., note: _Optional[str] = ...) -> None: ...

class SaveItemResponse(_message.Message):
    __slots__ = ("item",)
    ITEM_FIELD_NUMBER: _ClassVar[int]
    item: SavedItem
    def __init__(self, item: _Optional[_Union[SavedItem, _Mapping]] = ...) -> None: ...

class UnsaveItemRequest(_message.Message):
    __slots__ = ("content_id",)
    CONTENT_ID_FIELD_NUMBER: _ClassVar[int]
    content_id: str
    def __init__(self, content_id: _Optional[str] = ...) -> None: ...

class UnsaveItemResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class RequestExportRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class RequestExportResponse(_message.Message):
    __slots__ = ("export_id", "estimated_ready_at")
    EXPORT_ID_FIELD_NUMBER: _ClassVar[int]
    ESTIMATED_READY_AT_FIELD_NUMBER: _ClassVar[int]
    export_id: str
    estimated_ready_at: _timestamp_pb2.Timestamp
    def __init__(self, export_id: _Optional[str] = ..., estimated_ready_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class GetExportRequest(_message.Message):
    __slots__ = ("export_id",)
    EXPORT_ID_FIELD_NUMBER: _ClassVar[int]
    export_id: str
    def __init__(self, export_id: _Optional[str] = ...) -> None: ...

class GetExportResponse(_message.Message):
    __slots__ = ("status", "download_url", "download_url_expires_at", "size_bytes")
    class Status(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        STATUS_UNSPECIFIED: _ClassVar[GetExportResponse.Status]
        STATUS_PENDING: _ClassVar[GetExportResponse.Status]
        STATUS_READY: _ClassVar[GetExportResponse.Status]
        STATUS_FAILED: _ClassVar[GetExportResponse.Status]
        STATUS_EXPIRED: _ClassVar[GetExportResponse.Status]
    STATUS_UNSPECIFIED: GetExportResponse.Status
    STATUS_PENDING: GetExportResponse.Status
    STATUS_READY: GetExportResponse.Status
    STATUS_FAILED: GetExportResponse.Status
    STATUS_EXPIRED: GetExportResponse.Status
    STATUS_FIELD_NUMBER: _ClassVar[int]
    DOWNLOAD_URL_FIELD_NUMBER: _ClassVar[int]
    DOWNLOAD_URL_EXPIRES_AT_FIELD_NUMBER: _ClassVar[int]
    SIZE_BYTES_FIELD_NUMBER: _ClassVar[int]
    status: GetExportResponse.Status
    download_url: str
    download_url_expires_at: _timestamp_pb2.Timestamp
    size_bytes: int
    def __init__(self, status: _Optional[_Union[GetExportResponse.Status, str]] = ..., download_url: _Optional[str] = ..., download_url_expires_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., size_bytes: _Optional[int] = ...) -> None: ...

class DeleteAccountRequest(_message.Message):
    __slots__ = ("confirmation", "current_password")
    CONFIRMATION_FIELD_NUMBER: _ClassVar[int]
    CURRENT_PASSWORD_FIELD_NUMBER: _ClassVar[int]
    confirmation: str
    current_password: str
    def __init__(self, confirmation: _Optional[str] = ..., current_password: _Optional[str] = ...) -> None: ...

class DeleteAccountResponse(_message.Message):
    __slots__ = ("purge_by",)
    PURGE_BY_FIELD_NUMBER: _ClassVar[int]
    purge_by: _timestamp_pb2.Timestamp
    def __init__(self, purge_by: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...
