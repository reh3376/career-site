import datetime

from career.v1 import options_pb2 as _options_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ListDownloadsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class DownloadItem(_message.Message):
    __slots__ = ("variant", "title", "description", "version", "track_ids", "size_bytes", "mime_type", "updated_at", "recommended", "download_path")
    VARIANT_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    TRACK_IDS_FIELD_NUMBER: _ClassVar[int]
    SIZE_BYTES_FIELD_NUMBER: _ClassVar[int]
    MIME_TYPE_FIELD_NUMBER: _ClassVar[int]
    UPDATED_AT_FIELD_NUMBER: _ClassVar[int]
    RECOMMENDED_FIELD_NUMBER: _ClassVar[int]
    DOWNLOAD_PATH_FIELD_NUMBER: _ClassVar[int]
    variant: str
    title: str
    description: str
    version: str
    track_ids: _containers.RepeatedScalarFieldContainer[str]
    size_bytes: int
    mime_type: str
    updated_at: _timestamp_pb2.Timestamp
    recommended: bool
    download_path: str
    def __init__(self, variant: _Optional[str] = ..., title: _Optional[str] = ..., description: _Optional[str] = ..., version: _Optional[str] = ..., track_ids: _Optional[_Iterable[str]] = ..., size_bytes: _Optional[int] = ..., mime_type: _Optional[str] = ..., updated_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., recommended: _Optional[bool] = ..., download_path: _Optional[str] = ...) -> None: ...

class ListDownloadsResponse(_message.Message):
    __slots__ = ("items",)
    ITEMS_FIELD_NUMBER: _ClassVar[int]
    items: _containers.RepeatedCompositeFieldContainer[DownloadItem]
    def __init__(self, items: _Optional[_Iterable[_Union[DownloadItem, _Mapping]]] = ...) -> None: ...
