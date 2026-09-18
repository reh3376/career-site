import datetime

from career.v1 import common_pb2 as _common_pb2
from career.v1 import content_pb2 as _content_pb2
from career.v1 import options_pb2 as _options_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class GetHomeRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class WelcomeBack(_message.Message):
    __slots__ = ("last_visit_at", "previous_visits", "recently_viewed", "recent_topics", "new_since_last_visit", "resume_conversation_id")
    LAST_VISIT_AT_FIELD_NUMBER: _ClassVar[int]
    PREVIOUS_VISITS_FIELD_NUMBER: _ClassVar[int]
    RECENTLY_VIEWED_FIELD_NUMBER: _ClassVar[int]
    RECENT_TOPICS_FIELD_NUMBER: _ClassVar[int]
    NEW_SINCE_LAST_VISIT_FIELD_NUMBER: _ClassVar[int]
    RESUME_CONVERSATION_ID_FIELD_NUMBER: _ClassVar[int]
    last_visit_at: _timestamp_pb2.Timestamp
    previous_visits: int
    recently_viewed: _containers.RepeatedCompositeFieldContainer[_common_pb2.ContentSummary]
    recent_topics: _containers.RepeatedScalarFieldContainer[str]
    new_since_last_visit: _containers.RepeatedCompositeFieldContainer[_common_pb2.ContentSummary]
    resume_conversation_id: str
    def __init__(self, last_visit_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., previous_visits: _Optional[int] = ..., recently_viewed: _Optional[_Iterable[_Union[_common_pb2.ContentSummary, _Mapping]]] = ..., recent_topics: _Optional[_Iterable[str]] = ..., new_since_last_visit: _Optional[_Iterable[_Union[_common_pb2.ContentSummary, _Mapping]]] = ..., resume_conversation_id: _Optional[str] = ...) -> None: ...

class PathItem(_message.Message):
    __slots__ = ("position", "item", "completed")
    POSITION_FIELD_NUMBER: _ClassVar[int]
    ITEM_FIELD_NUMBER: _ClassVar[int]
    COMPLETED_FIELD_NUMBER: _ClassVar[int]
    position: int
    item: _common_pb2.ContentSummary
    completed: bool
    def __init__(self, position: _Optional[int] = ..., item: _Optional[_Union[_common_pb2.ContentSummary, _Mapping]] = ..., completed: _Optional[bool] = ...) -> None: ...

class HighlightSection(_message.Message):
    __slots__ = ("title", "reason", "track_id", "items")
    TITLE_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    TRACK_ID_FIELD_NUMBER: _ClassVar[int]
    ITEMS_FIELD_NUMBER: _ClassVar[int]
    title: str
    reason: str
    track_id: str
    items: _containers.RepeatedCompositeFieldContainer[_common_pb2.ContentSummary]
    def __init__(self, title: _Optional[str] = ..., reason: _Optional[str] = ..., track_id: _Optional[str] = ..., items: _Optional[_Iterable[_Union[_common_pb2.ContentSummary, _Mapping]]] = ...) -> None: ...

class RecommendedResume(_message.Message):
    __slots__ = ("variant", "title", "version", "download_path", "reason")
    VARIANT_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    DOWNLOAD_PATH_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    variant: str
    title: str
    version: str
    download_path: str
    reason: str
    def __init__(self, variant: _Optional[str] = ..., title: _Optional[str] = ..., version: _Optional[str] = ..., download_path: _Optional[str] = ..., reason: _Optional[str] = ...) -> None: ...

class Hero(_message.Message):
    __slots__ = ("variants", "alt", "caption")
    VARIANTS_FIELD_NUMBER: _ClassVar[int]
    ALT_FIELD_NUMBER: _ClassVar[int]
    CAPTION_FIELD_NUMBER: _ClassVar[int]
    variants: _containers.RepeatedCompositeFieldContainer[_content_pb2.ImageVariant]
    alt: str
    caption: str
    def __init__(self, variants: _Optional[_Iterable[_Union[_content_pb2.ImageVariant, _Mapping]]] = ..., alt: _Optional[str] = ..., caption: _Optional[str] = ...) -> None: ...

class GetHomeResponse(_message.Message):
    __slots__ = ("me", "welcome_back", "start_here", "start_here_track_id", "highlights", "resume", "hero", "suggested_questions", "questionnaire_pending", "tailored")
    ME_FIELD_NUMBER: _ClassVar[int]
    WELCOME_BACK_FIELD_NUMBER: _ClassVar[int]
    START_HERE_FIELD_NUMBER: _ClassVar[int]
    START_HERE_TRACK_ID_FIELD_NUMBER: _ClassVar[int]
    HIGHLIGHTS_FIELD_NUMBER: _ClassVar[int]
    RESUME_FIELD_NUMBER: _ClassVar[int]
    HERO_FIELD_NUMBER: _ClassVar[int]
    SUGGESTED_QUESTIONS_FIELD_NUMBER: _ClassVar[int]
    QUESTIONNAIRE_PENDING_FIELD_NUMBER: _ClassVar[int]
    TAILORED_FIELD_NUMBER: _ClassVar[int]
    me: _common_pb2.Me
    welcome_back: WelcomeBack
    start_here: _containers.RepeatedCompositeFieldContainer[PathItem]
    start_here_track_id: str
    highlights: _containers.RepeatedCompositeFieldContainer[HighlightSection]
    resume: RecommendedResume
    hero: Hero
    suggested_questions: _containers.RepeatedScalarFieldContainer[str]
    questionnaire_pending: bool
    tailored: bool
    def __init__(self, me: _Optional[_Union[_common_pb2.Me, _Mapping]] = ..., welcome_back: _Optional[_Union[WelcomeBack, _Mapping]] = ..., start_here: _Optional[_Iterable[_Union[PathItem, _Mapping]]] = ..., start_here_track_id: _Optional[str] = ..., highlights: _Optional[_Iterable[_Union[HighlightSection, _Mapping]]] = ..., resume: _Optional[_Union[RecommendedResume, _Mapping]] = ..., hero: _Optional[_Union[Hero, _Mapping]] = ..., suggested_questions: _Optional[_Iterable[str]] = ..., questionnaire_pending: _Optional[bool] = ..., tailored: _Optional[bool] = ...) -> None: ...
