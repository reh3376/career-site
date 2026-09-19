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

class ContentType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    CONTENT_TYPE_UNSPECIFIED: _ClassVar[ContentType]
    CONTENT_TYPE_ROLE: _ClassVar[ContentType]
    CONTENT_TYPE_PROJECT: _ClassVar[ContentType]
    CONTENT_TYPE_ARTICLE: _ClassVar[ContentType]
    CONTENT_TYPE_PRESENTATION: _ClassVar[ContentType]
    CONTENT_TYPE_SKILL: _ClassVar[ContentType]
    CONTENT_TYPE_CREDENTIAL: _ClassVar[ContentType]
    CONTENT_TYPE_PHOTO: _ClassVar[ContentType]
    CONTENT_TYPE_RESUME: _ClassVar[ContentType]
    CONTENT_TYPE_QA: _ClassVar[ContentType]

class InterestSource(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    INTEREST_SOURCE_UNSPECIFIED: _ClassVar[InterestSource]
    INTEREST_SOURCE_QUESTIONNAIRE: _ClassVar[InterestSource]
    INTEREST_SOURCE_EDITED: _ClassVar[InterestSource]
    INTEREST_SOURCE_INVITE: _ClassVar[InterestSource]

class MemberStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    MEMBER_STATUS_UNSPECIFIED: _ClassVar[MemberStatus]
    MEMBER_STATUS_UNVERIFIED: _ClassVar[MemberStatus]
    MEMBER_STATUS_ACTIVE: _ClassVar[MemberStatus]
    MEMBER_STATUS_PENDING_APPROVAL: _ClassVar[MemberStatus]
    MEMBER_STATUS_DISABLED: _ClassVar[MemberStatus]
    MEMBER_STATUS_DECLINED: _ClassVar[MemberStatus]
    MEMBER_STATUS_EXPIRED: _ClassVar[MemberStatus]

class MemberRole(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    MEMBER_ROLE_UNSPECIFIED: _ClassVar[MemberRole]
    MEMBER_ROLE_MEMBER: _ClassVar[MemberRole]
    MEMBER_ROLE_ADMIN: _ClassVar[MemberRole]

class Priority(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    PRIORITY_UNSPECIFIED: _ClassVar[Priority]
    PRIORITY_LEADERSHIP: _ClassVar[Priority]
    PRIORITY_HANDS_ON_DEPTH: _ClassVar[Priority]
    PRIORITY_RESEARCH: _ClassVar[Priority]
    PRIORITY_WRITING: _ClassVar[Priority]
CONTENT_TYPE_UNSPECIFIED: ContentType
CONTENT_TYPE_ROLE: ContentType
CONTENT_TYPE_PROJECT: ContentType
CONTENT_TYPE_ARTICLE: ContentType
CONTENT_TYPE_PRESENTATION: ContentType
CONTENT_TYPE_SKILL: ContentType
CONTENT_TYPE_CREDENTIAL: ContentType
CONTENT_TYPE_PHOTO: ContentType
CONTENT_TYPE_RESUME: ContentType
CONTENT_TYPE_QA: ContentType
INTEREST_SOURCE_UNSPECIFIED: InterestSource
INTEREST_SOURCE_QUESTIONNAIRE: InterestSource
INTEREST_SOURCE_EDITED: InterestSource
INTEREST_SOURCE_INVITE: InterestSource
MEMBER_STATUS_UNSPECIFIED: MemberStatus
MEMBER_STATUS_UNVERIFIED: MemberStatus
MEMBER_STATUS_ACTIVE: MemberStatus
MEMBER_STATUS_PENDING_APPROVAL: MemberStatus
MEMBER_STATUS_DISABLED: MemberStatus
MEMBER_STATUS_DECLINED: MemberStatus
MEMBER_STATUS_EXPIRED: MemberStatus
MEMBER_ROLE_UNSPECIFIED: MemberRole
MEMBER_ROLE_MEMBER: MemberRole
MEMBER_ROLE_ADMIN: MemberRole
PRIORITY_UNSPECIFIED: Priority
PRIORITY_LEADERSHIP: Priority
PRIORITY_HANDS_ON_DEPTH: Priority
PRIORITY_RESEARCH: Priority
PRIORITY_WRITING: Priority

class TrackWeight(_message.Message):
    __slots__ = ("track_id", "weight")
    TRACK_ID_FIELD_NUMBER: _ClassVar[int]
    WEIGHT_FIELD_NUMBER: _ClassVar[int]
    track_id: str
    weight: float
    def __init__(self, track_id: _Optional[str] = ..., weight: _Optional[float] = ...) -> None: ...

class Track(_message.Message):
    __slots__ = ("id", "name", "framing", "start_here", "suggested_questions", "resume_variant")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    FRAMING_FIELD_NUMBER: _ClassVar[int]
    START_HERE_FIELD_NUMBER: _ClassVar[int]
    SUGGESTED_QUESTIONS_FIELD_NUMBER: _ClassVar[int]
    RESUME_VARIANT_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    framing: str
    start_here: _containers.RepeatedScalarFieldContainer[str]
    suggested_questions: _containers.RepeatedScalarFieldContainer[str]
    resume_variant: str
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., framing: _Optional[str] = ..., start_here: _Optional[_Iterable[str]] = ..., suggested_questions: _Optional[_Iterable[str]] = ..., resume_variant: _Optional[str] = ...) -> None: ...

class ContentSummary(_message.Message):
    __slots__ = ("id", "slug", "type", "title", "summary", "published", "updated", "tags", "tracks", "relevance", "reason", "is_new", "viewed", "saved", "path")
    ID_FIELD_NUMBER: _ClassVar[int]
    SLUG_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    SUMMARY_FIELD_NUMBER: _ClassVar[int]
    PUBLISHED_FIELD_NUMBER: _ClassVar[int]
    UPDATED_FIELD_NUMBER: _ClassVar[int]
    TAGS_FIELD_NUMBER: _ClassVar[int]
    TRACKS_FIELD_NUMBER: _ClassVar[int]
    RELEVANCE_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    IS_NEW_FIELD_NUMBER: _ClassVar[int]
    VIEWED_FIELD_NUMBER: _ClassVar[int]
    SAVED_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    id: str
    slug: str
    type: ContentType
    title: str
    summary: str
    published: _timestamp_pb2.Timestamp
    updated: _timestamp_pb2.Timestamp
    tags: _containers.RepeatedScalarFieldContainer[str]
    tracks: _containers.RepeatedCompositeFieldContainer[TrackWeight]
    relevance: float
    reason: str
    is_new: bool
    viewed: bool
    saved: bool
    path: str
    def __init__(self, id: _Optional[str] = ..., slug: _Optional[str] = ..., type: _Optional[_Union[ContentType, str]] = ..., title: _Optional[str] = ..., summary: _Optional[str] = ..., published: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., updated: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., tags: _Optional[_Iterable[str]] = ..., tracks: _Optional[_Iterable[_Union[TrackWeight, _Mapping]]] = ..., relevance: _Optional[float] = ..., reason: _Optional[str] = ..., is_new: _Optional[bool] = ..., viewed: _Optional[bool] = ..., saved: _Optional[bool] = ..., path: _Optional[str] = ...) -> None: ...

class PageRequest(_message.Message):
    __slots__ = ("page_size", "page_token")
    PAGE_SIZE_FIELD_NUMBER: _ClassVar[int]
    PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    page_size: int
    page_token: str
    def __init__(self, page_size: _Optional[int] = ..., page_token: _Optional[str] = ...) -> None: ...

class PageResponse(_message.Message):
    __slots__ = ("next_page_token", "total_count")
    NEXT_PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    TOTAL_COUNT_FIELD_NUMBER: _ClassVar[int]
    next_page_token: str
    total_count: int
    def __init__(self, next_page_token: _Optional[str] = ..., total_count: _Optional[int] = ...) -> None: ...

class TrackInterest(_message.Message):
    __slots__ = ("track_id", "weight", "source")
    TRACK_ID_FIELD_NUMBER: _ClassVar[int]
    WEIGHT_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    track_id: str
    weight: float
    source: InterestSource
    def __init__(self, track_id: _Optional[str] = ..., weight: _Optional[float] = ..., source: _Optional[_Union[InterestSource, str]] = ...) -> None: ...

class Profile(_message.Message):
    __slots__ = ("seniority", "hiring_for", "priorities", "heard_from", "tailoring_enabled", "questionnaire_completed_at")
    SENIORITY_FIELD_NUMBER: _ClassVar[int]
    HIRING_FOR_FIELD_NUMBER: _ClassVar[int]
    PRIORITIES_FIELD_NUMBER: _ClassVar[int]
    HEARD_FROM_FIELD_NUMBER: _ClassVar[int]
    TAILORING_ENABLED_FIELD_NUMBER: _ClassVar[int]
    QUESTIONNAIRE_COMPLETED_AT_FIELD_NUMBER: _ClassVar[int]
    seniority: str
    hiring_for: str
    priorities: _containers.RepeatedScalarFieldContainer[Priority]
    heard_from: str
    tailoring_enabled: bool
    questionnaire_completed_at: _timestamp_pb2.Timestamp
    def __init__(self, seniority: _Optional[str] = ..., hiring_for: _Optional[str] = ..., priorities: _Optional[_Iterable[_Union[Priority, str]]] = ..., heard_from: _Optional[str] = ..., tailoring_enabled: _Optional[bool] = ..., questionnaire_completed_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class Me(_message.Message):
    __slots__ = ("id", "name", "email", "organization", "stated_role", "status", "role", "mfa_enrolled", "profile", "interests", "consent_version", "created_at", "last_seen_at", "linked_providers")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    EMAIL_FIELD_NUMBER: _ClassVar[int]
    ORGANIZATION_FIELD_NUMBER: _ClassVar[int]
    STATED_ROLE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    ROLE_FIELD_NUMBER: _ClassVar[int]
    MFA_ENROLLED_FIELD_NUMBER: _ClassVar[int]
    PROFILE_FIELD_NUMBER: _ClassVar[int]
    INTERESTS_FIELD_NUMBER: _ClassVar[int]
    CONSENT_VERSION_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    LAST_SEEN_AT_FIELD_NUMBER: _ClassVar[int]
    LINKED_PROVIDERS_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    email: str
    organization: str
    stated_role: str
    status: MemberStatus
    role: MemberRole
    mfa_enrolled: bool
    profile: Profile
    interests: _containers.RepeatedCompositeFieldContainer[TrackInterest]
    consent_version: str
    created_at: _timestamp_pb2.Timestamp
    last_seen_at: _timestamp_pb2.Timestamp
    linked_providers: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., email: _Optional[str] = ..., organization: _Optional[str] = ..., stated_role: _Optional[str] = ..., status: _Optional[_Union[MemberStatus, str]] = ..., role: _Optional[_Union[MemberRole, str]] = ..., mfa_enrolled: _Optional[bool] = ..., profile: _Optional[_Union[Profile, _Mapping]] = ..., interests: _Optional[_Iterable[_Union[TrackInterest, _Mapping]]] = ..., consent_version: _Optional[str] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., last_seen_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., linked_providers: _Optional[_Iterable[str]] = ...) -> None: ...

class ActivityEvent(_message.Message):
    __slots__ = ("kind", "content_id", "occurred_at", "dwell_ms", "query", "conversation_id", "variant", "client_event_id", "content")
    class Kind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        KIND_UNSPECIFIED: _ClassVar[ActivityEvent.Kind]
        KIND_VIEW: _ClassVar[ActivityEvent.Kind]
        KIND_DOWNLOAD: _ClassVar[ActivityEvent.Kind]
        KIND_SEARCH: _ClassVar[ActivityEvent.Kind]
        KIND_SAVE: _ClassVar[ActivityEvent.Kind]
        KIND_CHAT: _ClassVar[ActivityEvent.Kind]
        KIND_ESCALATE: _ClassVar[ActivityEvent.Kind]
    KIND_UNSPECIFIED: ActivityEvent.Kind
    KIND_VIEW: ActivityEvent.Kind
    KIND_DOWNLOAD: ActivityEvent.Kind
    KIND_SEARCH: ActivityEvent.Kind
    KIND_SAVE: ActivityEvent.Kind
    KIND_CHAT: ActivityEvent.Kind
    KIND_ESCALATE: ActivityEvent.Kind
    KIND_FIELD_NUMBER: _ClassVar[int]
    CONTENT_ID_FIELD_NUMBER: _ClassVar[int]
    OCCURRED_AT_FIELD_NUMBER: _ClassVar[int]
    DWELL_MS_FIELD_NUMBER: _ClassVar[int]
    QUERY_FIELD_NUMBER: _ClassVar[int]
    CONVERSATION_ID_FIELD_NUMBER: _ClassVar[int]
    VARIANT_FIELD_NUMBER: _ClassVar[int]
    CLIENT_EVENT_ID_FIELD_NUMBER: _ClassVar[int]
    CONTENT_FIELD_NUMBER: _ClassVar[int]
    kind: ActivityEvent.Kind
    content_id: str
    occurred_at: _timestamp_pb2.Timestamp
    dwell_ms: int
    query: str
    conversation_id: str
    variant: str
    client_event_id: str
    content: ContentSummary
    def __init__(self, kind: _Optional[_Union[ActivityEvent.Kind, str]] = ..., content_id: _Optional[str] = ..., occurred_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., dwell_ms: _Optional[int] = ..., query: _Optional[str] = ..., conversation_id: _Optional[str] = ..., variant: _Optional[str] = ..., client_event_id: _Optional[str] = ..., content: _Optional[_Union[ContentSummary, _Mapping]] = ...) -> None: ...
