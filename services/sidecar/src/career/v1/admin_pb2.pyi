import datetime

from buf.validate import validate_pb2 as _validate_pb2
from career.v1 import chat_pb2 as _chat_pb2
from career.v1 import common_pb2 as _common_pb2
from career.v1 import contact_pb2 as _contact_pb2
from career.v1 import jd_pb2 as _jd_pb2
from career.v1 import options_pb2 as _options_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ReviewKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    REVIEW_KIND_UNSPECIFIED: _ClassVar[ReviewKind]
    REVIEW_KIND_NEGATIVE_FEEDBACK: _ClassVar[ReviewKind]
    REVIEW_KIND_NO_SUPPORT: _ClassVar[ReviewKind]
    REVIEW_KIND_ESCALATION: _ClassVar[ReviewKind]
    REVIEW_KIND_OUT_OF_SCOPE: _ClassVar[ReviewKind]

class ReviewStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    REVIEW_STATUS_UNSPECIFIED: _ClassVar[ReviewStatus]
    REVIEW_STATUS_OPEN: _ClassVar[ReviewStatus]
    REVIEW_STATUS_RESOLVED: _ClassVar[ReviewStatus]
    REVIEW_STATUS_CONVERTED_TO_QA: _ClassVar[ReviewStatus]
    REVIEW_STATUS_REPLIED: _ClassVar[ReviewStatus]

class JobKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    JOB_KIND_UNSPECIFIED: _ClassVar[JobKind]
    JOB_KIND_CONTENT_VALIDATE: _ClassVar[JobKind]
    JOB_KIND_CONTENT_INDEX: _ClassVar[JobKind]
    JOB_KIND_INGEST_CHANGED: _ClassVar[JobKind]
    JOB_KIND_INGEST_FULL: _ClassVar[JobKind]
    JOB_KIND_RESUME_BUILD: _ClassVar[JobKind]
    JOB_KIND_EVAL_QUICK: _ClassVar[JobKind]
    JOB_KIND_GITHUB_SYNC: _ClassVar[JobKind]

class JobStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    JOB_STATUS_UNSPECIFIED: _ClassVar[JobStatus]
    JOB_STATUS_QUEUED: _ClassVar[JobStatus]
    JOB_STATUS_RUNNING: _ClassVar[JobStatus]
    JOB_STATUS_SUCCEEDED: _ClassVar[JobStatus]
    JOB_STATUS_FAILED: _ClassVar[JobStatus]

class SupportStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SUPPORT_STATUS_UNSPECIFIED: _ClassVar[SupportStatus]
    SUPPORT_STATUS_OPEN: _ClassVar[SupportStatus]
    SUPPORT_STATUS_RESOLVED: _ClassVar[SupportStatus]

class GrantTTL(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    GRANT_TTL_UNSPECIFIED: _ClassVar[GrantTTL]
    GRANT_TTL_1D: _ClassVar[GrantTTL]
    GRANT_TTL_3D: _ClassVar[GrantTTL]
    GRANT_TTL_7D: _ClassVar[GrantTTL]
    GRANT_TTL_30D: _ClassVar[GrantTTL]
    GRANT_TTL_PERMANENT: _ClassVar[GrantTTL]

class ActivitySort(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    ACTIVITY_SORT_UNSPECIFIED: _ClassVar[ActivitySort]
    ACTIVITY_SORT_LAST_EVENT_DESC: _ClassVar[ActivitySort]
    ACTIVITY_SORT_SESSIONS_DESC: _ClassVar[ActivitySort]
    ACTIVITY_SORT_ACTIVE_TIME_DESC: _ClassVar[ActivitySort]
    ACTIVITY_SORT_ASK_ROGER_DESC: _ClassVar[ActivitySort]
REVIEW_KIND_UNSPECIFIED: ReviewKind
REVIEW_KIND_NEGATIVE_FEEDBACK: ReviewKind
REVIEW_KIND_NO_SUPPORT: ReviewKind
REVIEW_KIND_ESCALATION: ReviewKind
REVIEW_KIND_OUT_OF_SCOPE: ReviewKind
REVIEW_STATUS_UNSPECIFIED: ReviewStatus
REVIEW_STATUS_OPEN: ReviewStatus
REVIEW_STATUS_RESOLVED: ReviewStatus
REVIEW_STATUS_CONVERTED_TO_QA: ReviewStatus
REVIEW_STATUS_REPLIED: ReviewStatus
JOB_KIND_UNSPECIFIED: JobKind
JOB_KIND_CONTENT_VALIDATE: JobKind
JOB_KIND_CONTENT_INDEX: JobKind
JOB_KIND_INGEST_CHANGED: JobKind
JOB_KIND_INGEST_FULL: JobKind
JOB_KIND_RESUME_BUILD: JobKind
JOB_KIND_EVAL_QUICK: JobKind
JOB_KIND_GITHUB_SYNC: JobKind
JOB_STATUS_UNSPECIFIED: JobStatus
JOB_STATUS_QUEUED: JobStatus
JOB_STATUS_RUNNING: JobStatus
JOB_STATUS_SUCCEEDED: JobStatus
JOB_STATUS_FAILED: JobStatus
SUPPORT_STATUS_UNSPECIFIED: SupportStatus
SUPPORT_STATUS_OPEN: SupportStatus
SUPPORT_STATUS_RESOLVED: SupportStatus
GRANT_TTL_UNSPECIFIED: GrantTTL
GRANT_TTL_1D: GrantTTL
GRANT_TTL_3D: GrantTTL
GRANT_TTL_7D: GrantTTL
GRANT_TTL_30D: GrantTTL
GRANT_TTL_PERMANENT: GrantTTL
ACTIVITY_SORT_UNSPECIFIED: ActivitySort
ACTIVITY_SORT_LAST_EVENT_DESC: ActivitySort
ACTIVITY_SORT_SESSIONS_DESC: ActivitySort
ACTIVITY_SORT_ACTIVE_TIME_DESC: ActivitySort
ACTIVITY_SORT_ASK_ROGER_DESC: ActivitySort

class MemberCounts(_message.Message):
    __slots__ = ("views", "downloads", "chat_messages", "escalations", "saved")
    VIEWS_FIELD_NUMBER: _ClassVar[int]
    DOWNLOADS_FIELD_NUMBER: _ClassVar[int]
    CHAT_MESSAGES_FIELD_NUMBER: _ClassVar[int]
    ESCALATIONS_FIELD_NUMBER: _ClassVar[int]
    SAVED_FIELD_NUMBER: _ClassVar[int]
    views: int
    downloads: int
    chat_messages: int
    escalations: int
    saved: int
    def __init__(self, views: _Optional[int] = ..., downloads: _Optional[int] = ..., chat_messages: _Optional[int] = ..., escalations: _Optional[int] = ..., saved: _Optional[int] = ...) -> None: ...

class MemberRecord(_message.Message):
    __slots__ = ("me", "counts", "first_seen_at", "note_count")
    ME_FIELD_NUMBER: _ClassVar[int]
    COUNTS_FIELD_NUMBER: _ClassVar[int]
    FIRST_SEEN_AT_FIELD_NUMBER: _ClassVar[int]
    NOTE_COUNT_FIELD_NUMBER: _ClassVar[int]
    me: _common_pb2.Me
    counts: MemberCounts
    first_seen_at: _timestamp_pb2.Timestamp
    note_count: int
    def __init__(self, me: _Optional[_Union[_common_pb2.Me, _Mapping]] = ..., counts: _Optional[_Union[MemberCounts, _Mapping]] = ..., first_seen_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., note_count: _Optional[int] = ...) -> None: ...

class ListMembersRequest(_message.Message):
    __slots__ = ("query", "status", "track_id", "sort", "page")
    class Sort(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        SORT_UNSPECIFIED: _ClassVar[ListMembersRequest.Sort]
        SORT_NEWEST: _ClassVar[ListMembersRequest.Sort]
        SORT_MOST_ACTIVE: _ClassVar[ListMembersRequest.Sort]
        SORT_NAME: _ClassVar[ListMembersRequest.Sort]
    SORT_UNSPECIFIED: ListMembersRequest.Sort
    SORT_NEWEST: ListMembersRequest.Sort
    SORT_MOST_ACTIVE: ListMembersRequest.Sort
    SORT_NAME: ListMembersRequest.Sort
    QUERY_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    TRACK_ID_FIELD_NUMBER: _ClassVar[int]
    SORT_FIELD_NUMBER: _ClassVar[int]
    PAGE_FIELD_NUMBER: _ClassVar[int]
    query: str
    status: _common_pb2.MemberStatus
    track_id: str
    sort: ListMembersRequest.Sort
    page: _common_pb2.PageRequest
    def __init__(self, query: _Optional[str] = ..., status: _Optional[_Union[_common_pb2.MemberStatus, str]] = ..., track_id: _Optional[str] = ..., sort: _Optional[_Union[ListMembersRequest.Sort, str]] = ..., page: _Optional[_Union[_common_pb2.PageRequest, _Mapping]] = ...) -> None: ...

class ListMembersResponse(_message.Message):
    __slots__ = ("members", "page")
    MEMBERS_FIELD_NUMBER: _ClassVar[int]
    PAGE_FIELD_NUMBER: _ClassVar[int]
    members: _containers.RepeatedCompositeFieldContainer[MemberRecord]
    page: _common_pb2.PageResponse
    def __init__(self, members: _Optional[_Iterable[_Union[MemberRecord, _Mapping]]] = ..., page: _Optional[_Union[_common_pb2.PageResponse, _Mapping]] = ...) -> None: ...

class GetMemberRequest(_message.Message):
    __slots__ = ("member_id",)
    MEMBER_ID_FIELD_NUMBER: _ClassVar[int]
    member_id: str
    def __init__(self, member_id: _Optional[str] = ...) -> None: ...

class AdminNote(_message.Message):
    __slots__ = ("id", "text", "created_at")
    ID_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    id: str
    text: str
    created_at: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[str] = ..., text: _Optional[str] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class NotificationDelivery(_message.Message):
    __slots__ = ("id", "kind", "recipient", "provider", "triggered_by", "duration_ms", "ok", "error", "sent_at")
    ID_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    RECIPIENT_FIELD_NUMBER: _ClassVar[int]
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    TRIGGERED_BY_FIELD_NUMBER: _ClassVar[int]
    DURATION_MS_FIELD_NUMBER: _ClassVar[int]
    OK_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    SENT_AT_FIELD_NUMBER: _ClassVar[int]
    id: str
    kind: str
    recipient: str
    provider: str
    triggered_by: str
    duration_ms: int
    ok: bool
    error: str
    sent_at: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[str] = ..., kind: _Optional[str] = ..., recipient: _Optional[str] = ..., provider: _Optional[str] = ..., triggered_by: _Optional[str] = ..., duration_ms: _Optional[int] = ..., ok: _Optional[bool] = ..., error: _Optional[str] = ..., sent_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class GetMemberResponse(_message.Message):
    __slots__ = ("member", "recent_activity", "conversations", "notes", "deliveries")
    MEMBER_FIELD_NUMBER: _ClassVar[int]
    RECENT_ACTIVITY_FIELD_NUMBER: _ClassVar[int]
    CONVERSATIONS_FIELD_NUMBER: _ClassVar[int]
    NOTES_FIELD_NUMBER: _ClassVar[int]
    DELIVERIES_FIELD_NUMBER: _ClassVar[int]
    member: MemberRecord
    recent_activity: _containers.RepeatedCompositeFieldContainer[_common_pb2.ActivityEvent]
    conversations: _containers.RepeatedCompositeFieldContainer[_chat_pb2.Conversation]
    notes: _containers.RepeatedCompositeFieldContainer[AdminNote]
    deliveries: _containers.RepeatedCompositeFieldContainer[NotificationDelivery]
    def __init__(self, member: _Optional[_Union[MemberRecord, _Mapping]] = ..., recent_activity: _Optional[_Iterable[_Union[_common_pb2.ActivityEvent, _Mapping]]] = ..., conversations: _Optional[_Iterable[_Union[_chat_pb2.Conversation, _Mapping]]] = ..., notes: _Optional[_Iterable[_Union[AdminNote, _Mapping]]] = ..., deliveries: _Optional[_Iterable[_Union[NotificationDelivery, _Mapping]]] = ...) -> None: ...

class ResendNotificationRequest(_message.Message):
    __slots__ = ("member_id", "kind")
    MEMBER_ID_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    member_id: str
    kind: str
    def __init__(self, member_id: _Optional[str] = ..., kind: _Optional[str] = ...) -> None: ...

class ResendNotificationResponse(_message.Message):
    __slots__ = ("delivery",)
    DELIVERY_FIELD_NUMBER: _ClassVar[int]
    delivery: NotificationDelivery
    def __init__(self, delivery: _Optional[_Union[NotificationDelivery, _Mapping]] = ...) -> None: ...

class AddMemberNoteRequest(_message.Message):
    __slots__ = ("member_id", "text")
    MEMBER_ID_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    member_id: str
    text: str
    def __init__(self, member_id: _Optional[str] = ..., text: _Optional[str] = ...) -> None: ...

class AddMemberNoteResponse(_message.Message):
    __slots__ = ("note",)
    NOTE_FIELD_NUMBER: _ClassVar[int]
    note: AdminNote
    def __init__(self, note: _Optional[_Union[AdminNote, _Mapping]] = ...) -> None: ...

class SetMemberStatusRequest(_message.Message):
    __slots__ = ("member_id", "status", "reason")
    MEMBER_ID_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    member_id: str
    status: _common_pb2.MemberStatus
    reason: str
    def __init__(self, member_id: _Optional[str] = ..., status: _Optional[_Union[_common_pb2.MemberStatus, str]] = ..., reason: _Optional[str] = ...) -> None: ...

class SetMemberStatusResponse(_message.Message):
    __slots__ = ("member",)
    MEMBER_FIELD_NUMBER: _ClassVar[int]
    member: MemberRecord
    def __init__(self, member: _Optional[_Union[MemberRecord, _Mapping]] = ...) -> None: ...

class GetReviewQueueRequest(_message.Message):
    __slots__ = ("kind", "status", "page")
    KIND_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    PAGE_FIELD_NUMBER: _ClassVar[int]
    kind: ReviewKind
    status: ReviewStatus
    page: _common_pb2.PageRequest
    def __init__(self, kind: _Optional[_Union[ReviewKind, str]] = ..., status: _Optional[_Union[ReviewStatus, str]] = ..., page: _Optional[_Union[_common_pb2.PageRequest, _Mapping]] = ...) -> None: ...

class ReviewItem(_message.Message):
    __slots__ = ("id", "kind", "status", "member", "conversation_id", "message_id", "question", "answer_excerpt", "comment", "escalation_id", "created_at")
    ID_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    MEMBER_FIELD_NUMBER: _ClassVar[int]
    CONVERSATION_ID_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_ID_FIELD_NUMBER: _ClassVar[int]
    QUESTION_FIELD_NUMBER: _ClassVar[int]
    ANSWER_EXCERPT_FIELD_NUMBER: _ClassVar[int]
    COMMENT_FIELD_NUMBER: _ClassVar[int]
    ESCALATION_ID_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    id: str
    kind: ReviewKind
    status: ReviewStatus
    member: _common_pb2.Me
    conversation_id: str
    message_id: str
    question: str
    answer_excerpt: str
    comment: str
    escalation_id: str
    created_at: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[str] = ..., kind: _Optional[_Union[ReviewKind, str]] = ..., status: _Optional[_Union[ReviewStatus, str]] = ..., member: _Optional[_Union[_common_pb2.Me, _Mapping]] = ..., conversation_id: _Optional[str] = ..., message_id: _Optional[str] = ..., question: _Optional[str] = ..., answer_excerpt: _Optional[str] = ..., comment: _Optional[str] = ..., escalation_id: _Optional[str] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class GetReviewQueueResponse(_message.Message):
    __slots__ = ("items", "page", "open_count")
    ITEMS_FIELD_NUMBER: _ClassVar[int]
    PAGE_FIELD_NUMBER: _ClassVar[int]
    OPEN_COUNT_FIELD_NUMBER: _ClassVar[int]
    items: _containers.RepeatedCompositeFieldContainer[ReviewItem]
    page: _common_pb2.PageResponse
    open_count: int
    def __init__(self, items: _Optional[_Iterable[_Union[ReviewItem, _Mapping]]] = ..., page: _Optional[_Union[_common_pb2.PageResponse, _Mapping]] = ..., open_count: _Optional[int] = ...) -> None: ...

class ResolveReviewItemRequest(_message.Message):
    __slots__ = ("review_item_id", "resolution", "note")
    REVIEW_ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    RESOLUTION_FIELD_NUMBER: _ClassVar[int]
    NOTE_FIELD_NUMBER: _ClassVar[int]
    review_item_id: str
    resolution: ReviewStatus
    note: str
    def __init__(self, review_item_id: _Optional[str] = ..., resolution: _Optional[_Union[ReviewStatus, str]] = ..., note: _Optional[str] = ...) -> None: ...

class ResolveReviewItemResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ReplyEscalationRequest(_message.Message):
    __slots__ = ("escalation_id", "text", "notify_member")
    ESCALATION_ID_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    NOTIFY_MEMBER_FIELD_NUMBER: _ClassVar[int]
    escalation_id: str
    text: str
    notify_member: bool
    def __init__(self, escalation_id: _Optional[str] = ..., text: _Optional[str] = ..., notify_member: _Optional[bool] = ...) -> None: ...

class ReplyEscalationResponse(_message.Message):
    __slots__ = ("message",)
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    message: _chat_pb2.Message
    def __init__(self, message: _Optional[_Union[_chat_pb2.Message, _Mapping]] = ...) -> None: ...

class GetMemberConversationRequest(_message.Message):
    __slots__ = ("conversation_id",)
    CONVERSATION_ID_FIELD_NUMBER: _ClassVar[int]
    conversation_id: str
    def __init__(self, conversation_id: _Optional[str] = ...) -> None: ...

class GetMemberConversationResponse(_message.Message):
    __slots__ = ("member", "conversation", "messages")
    MEMBER_FIELD_NUMBER: _ClassVar[int]
    CONVERSATION_FIELD_NUMBER: _ClassVar[int]
    MESSAGES_FIELD_NUMBER: _ClassVar[int]
    member: _common_pb2.Me
    conversation: _chat_pb2.Conversation
    messages: _containers.RepeatedCompositeFieldContainer[_chat_pb2.Message]
    def __init__(self, member: _Optional[_Union[_common_pb2.Me, _Mapping]] = ..., conversation: _Optional[_Union[_chat_pb2.Conversation, _Mapping]] = ..., messages: _Optional[_Iterable[_Union[_chat_pb2.Message, _Mapping]]] = ...) -> None: ...

class GetCorpusStatusRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetCorpusStatusResponse(_message.Message):
    __slots__ = ("documents", "chunks", "qa_entries", "documents_by_type", "private_documents", "last_ingest_at", "embedding_model", "embedding_dimensions", "persona_version")
    class DocumentsByTypeEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: int
        def __init__(self, key: _Optional[str] = ..., value: _Optional[int] = ...) -> None: ...
    DOCUMENTS_FIELD_NUMBER: _ClassVar[int]
    CHUNKS_FIELD_NUMBER: _ClassVar[int]
    QA_ENTRIES_FIELD_NUMBER: _ClassVar[int]
    DOCUMENTS_BY_TYPE_FIELD_NUMBER: _ClassVar[int]
    PRIVATE_DOCUMENTS_FIELD_NUMBER: _ClassVar[int]
    LAST_INGEST_AT_FIELD_NUMBER: _ClassVar[int]
    EMBEDDING_MODEL_FIELD_NUMBER: _ClassVar[int]
    EMBEDDING_DIMENSIONS_FIELD_NUMBER: _ClassVar[int]
    PERSONA_VERSION_FIELD_NUMBER: _ClassVar[int]
    documents: int
    chunks: int
    qa_entries: int
    documents_by_type: _containers.ScalarMap[str, int]
    private_documents: int
    last_ingest_at: _timestamp_pb2.Timestamp
    embedding_model: str
    embedding_dimensions: int
    persona_version: str
    def __init__(self, documents: _Optional[int] = ..., chunks: _Optional[int] = ..., qa_entries: _Optional[int] = ..., documents_by_type: _Optional[_Mapping[str, int]] = ..., private_documents: _Optional[int] = ..., last_ingest_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., embedding_model: _Optional[str] = ..., embedding_dimensions: _Optional[int] = ..., persona_version: _Optional[str] = ...) -> None: ...

class TestRetrievalRequest(_message.Message):
    __slots__ = ("query", "top_k", "rerank")
    QUERY_FIELD_NUMBER: _ClassVar[int]
    TOP_K_FIELD_NUMBER: _ClassVar[int]
    RERANK_FIELD_NUMBER: _ClassVar[int]
    query: str
    top_k: int
    rerank: bool
    def __init__(self, query: _Optional[str] = ..., top_k: _Optional[int] = ..., rerank: _Optional[bool] = ...) -> None: ...

class RetrievalHit(_message.Message):
    __slots__ = ("chunk_id", "content_id", "title", "heading", "score", "text", "pre_rerank_rank")
    CHUNK_ID_FIELD_NUMBER: _ClassVar[int]
    CONTENT_ID_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    HEADING_FIELD_NUMBER: _ClassVar[int]
    SCORE_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    PRE_RERANK_RANK_FIELD_NUMBER: _ClassVar[int]
    chunk_id: str
    content_id: str
    title: str
    heading: str
    score: float
    text: str
    pre_rerank_rank: int
    def __init__(self, chunk_id: _Optional[str] = ..., content_id: _Optional[str] = ..., title: _Optional[str] = ..., heading: _Optional[str] = ..., score: _Optional[float] = ..., text: _Optional[str] = ..., pre_rerank_rank: _Optional[int] = ...) -> None: ...

class TestRetrievalResponse(_message.Message):
    __slots__ = ("hits", "qa_match", "qa_similarity", "latency_ms")
    HITS_FIELD_NUMBER: _ClassVar[int]
    QA_MATCH_FIELD_NUMBER: _ClassVar[int]
    QA_SIMILARITY_FIELD_NUMBER: _ClassVar[int]
    LATENCY_MS_FIELD_NUMBER: _ClassVar[int]
    hits: _containers.RepeatedCompositeFieldContainer[RetrievalHit]
    qa_match: _common_pb2.ContentSummary
    qa_similarity: float
    latency_ms: int
    def __init__(self, hits: _Optional[_Iterable[_Union[RetrievalHit, _Mapping]]] = ..., qa_match: _Optional[_Union[_common_pb2.ContentSummary, _Mapping]] = ..., qa_similarity: _Optional[float] = ..., latency_ms: _Optional[int] = ...) -> None: ...

class RunJobRequest(_message.Message):
    __slots__ = ("kind",)
    KIND_FIELD_NUMBER: _ClassVar[int]
    kind: JobKind
    def __init__(self, kind: _Optional[_Union[JobKind, str]] = ...) -> None: ...

class RunJobResponse(_message.Message):
    __slots__ = ("job_id",)
    JOB_ID_FIELD_NUMBER: _ClassVar[int]
    job_id: str
    def __init__(self, job_id: _Optional[str] = ...) -> None: ...

class GetJobRequest(_message.Message):
    __slots__ = ("job_id",)
    JOB_ID_FIELD_NUMBER: _ClassVar[int]
    job_id: str
    def __init__(self, job_id: _Optional[str] = ...) -> None: ...

class GetJobResponse(_message.Message):
    __slots__ = ("job_id", "kind", "status", "progress_pct", "started_at", "finished_at", "summary")
    JOB_ID_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    PROGRESS_PCT_FIELD_NUMBER: _ClassVar[int]
    STARTED_AT_FIELD_NUMBER: _ClassVar[int]
    FINISHED_AT_FIELD_NUMBER: _ClassVar[int]
    SUMMARY_FIELD_NUMBER: _ClassVar[int]
    job_id: str
    kind: JobKind
    status: JobStatus
    progress_pct: int
    started_at: _timestamp_pb2.Timestamp
    finished_at: _timestamp_pb2.Timestamp
    summary: str
    def __init__(self, job_id: _Optional[str] = ..., kind: _Optional[_Union[JobKind, str]] = ..., status: _Optional[_Union[JobStatus, str]] = ..., progress_pct: _Optional[int] = ..., started_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., finished_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., summary: _Optional[str] = ...) -> None: ...

class GetPersonaRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class PersonaVersion(_message.Message):
    __slots__ = ("version", "prompt_sha256", "style_notes", "active", "activated_at", "eval_pass_rate")
    VERSION_FIELD_NUMBER: _ClassVar[int]
    PROMPT_SHA256_FIELD_NUMBER: _ClassVar[int]
    STYLE_NOTES_FIELD_NUMBER: _ClassVar[int]
    ACTIVE_FIELD_NUMBER: _ClassVar[int]
    ACTIVATED_AT_FIELD_NUMBER: _ClassVar[int]
    EVAL_PASS_RATE_FIELD_NUMBER: _ClassVar[int]
    version: str
    prompt_sha256: str
    style_notes: str
    active: bool
    activated_at: _timestamp_pb2.Timestamp
    eval_pass_rate: float
    def __init__(self, version: _Optional[str] = ..., prompt_sha256: _Optional[str] = ..., style_notes: _Optional[str] = ..., active: _Optional[bool] = ..., activated_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., eval_pass_rate: _Optional[float] = ...) -> None: ...

class GetPersonaResponse(_message.Message):
    __slots__ = ("active", "history")
    ACTIVE_FIELD_NUMBER: _ClassVar[int]
    HISTORY_FIELD_NUMBER: _ClassVar[int]
    active: PersonaVersion
    history: _containers.RepeatedCompositeFieldContainer[PersonaVersion]
    def __init__(self, active: _Optional[_Union[PersonaVersion, _Mapping]] = ..., history: _Optional[_Iterable[_Union[PersonaVersion, _Mapping]]] = ...) -> None: ...

class GetAnalyticsRequest(_message.Message):
    __slots__ = ("to",)
    FROM_FIELD_NUMBER: _ClassVar[int]
    TO_FIELD_NUMBER: _ClassVar[int]
    to: _timestamp_pb2.Timestamp
    def __init__(self, to: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., **kwargs) -> None: ...

class DailyCount(_message.Message):
    __slots__ = ("day", "count")
    DAY_FIELD_NUMBER: _ClassVar[int]
    COUNT_FIELD_NUMBER: _ClassVar[int]
    day: _timestamp_pb2.Timestamp
    count: int
    def __init__(self, day: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., count: _Optional[int] = ...) -> None: ...

class RankedContent(_message.Message):
    __slots__ = ("item", "count")
    ITEM_FIELD_NUMBER: _ClassVar[int]
    COUNT_FIELD_NUMBER: _ClassVar[int]
    item: _common_pb2.ContentSummary
    count: int
    def __init__(self, item: _Optional[_Union[_common_pb2.ContentSummary, _Mapping]] = ..., count: _Optional[int] = ...) -> None: ...

class RankedQuestion(_message.Message):
    __slots__ = ("question", "count", "no_support_pct")
    QUESTION_FIELD_NUMBER: _ClassVar[int]
    COUNT_FIELD_NUMBER: _ClassVar[int]
    NO_SUPPORT_PCT_FIELD_NUMBER: _ClassVar[int]
    question: str
    count: int
    no_support_pct: float
    def __init__(self, question: _Optional[str] = ..., count: _Optional[int] = ..., no_support_pct: _Optional[float] = ...) -> None: ...

class AssistantUsage(_message.Message):
    __slots__ = ("messages", "conversations", "input_tokens", "output_tokens", "cost_usd", "budget_used_pct", "ratings_up", "ratings_down")
    MESSAGES_FIELD_NUMBER: _ClassVar[int]
    CONVERSATIONS_FIELD_NUMBER: _ClassVar[int]
    INPUT_TOKENS_FIELD_NUMBER: _ClassVar[int]
    OUTPUT_TOKENS_FIELD_NUMBER: _ClassVar[int]
    COST_USD_FIELD_NUMBER: _ClassVar[int]
    BUDGET_USED_PCT_FIELD_NUMBER: _ClassVar[int]
    RATINGS_UP_FIELD_NUMBER: _ClassVar[int]
    RATINGS_DOWN_FIELD_NUMBER: _ClassVar[int]
    messages: int
    conversations: int
    input_tokens: int
    output_tokens: int
    cost_usd: float
    budget_used_pct: float
    ratings_up: int
    ratings_down: int
    def __init__(self, messages: _Optional[int] = ..., conversations: _Optional[int] = ..., input_tokens: _Optional[int] = ..., output_tokens: _Optional[int] = ..., cost_usd: _Optional[float] = ..., budget_used_pct: _Optional[float] = ..., ratings_up: _Optional[int] = ..., ratings_down: _Optional[int] = ...) -> None: ...

class GetAnalyticsResponse(_message.Message):
    __slots__ = ("registrations", "verifications", "active_members", "questionnaires_completed", "top_content", "top_downloads", "top_questions", "assistant", "members_by_track")
    class MembersByTrackEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: int
        def __init__(self, key: _Optional[str] = ..., value: _Optional[int] = ...) -> None: ...
    REGISTRATIONS_FIELD_NUMBER: _ClassVar[int]
    VERIFICATIONS_FIELD_NUMBER: _ClassVar[int]
    ACTIVE_MEMBERS_FIELD_NUMBER: _ClassVar[int]
    QUESTIONNAIRES_COMPLETED_FIELD_NUMBER: _ClassVar[int]
    TOP_CONTENT_FIELD_NUMBER: _ClassVar[int]
    TOP_DOWNLOADS_FIELD_NUMBER: _ClassVar[int]
    TOP_QUESTIONS_FIELD_NUMBER: _ClassVar[int]
    ASSISTANT_FIELD_NUMBER: _ClassVar[int]
    MEMBERS_BY_TRACK_FIELD_NUMBER: _ClassVar[int]
    registrations: _containers.RepeatedCompositeFieldContainer[DailyCount]
    verifications: _containers.RepeatedCompositeFieldContainer[DailyCount]
    active_members: _containers.RepeatedCompositeFieldContainer[DailyCount]
    questionnaires_completed: _containers.RepeatedCompositeFieldContainer[DailyCount]
    top_content: _containers.RepeatedCompositeFieldContainer[RankedContent]
    top_downloads: _containers.RepeatedCompositeFieldContainer[RankedContent]
    top_questions: _containers.RepeatedCompositeFieldContainer[RankedQuestion]
    assistant: AssistantUsage
    members_by_track: _containers.ScalarMap[str, int]
    def __init__(self, registrations: _Optional[_Iterable[_Union[DailyCount, _Mapping]]] = ..., verifications: _Optional[_Iterable[_Union[DailyCount, _Mapping]]] = ..., active_members: _Optional[_Iterable[_Union[DailyCount, _Mapping]]] = ..., questionnaires_completed: _Optional[_Iterable[_Union[DailyCount, _Mapping]]] = ..., top_content: _Optional[_Iterable[_Union[RankedContent, _Mapping]]] = ..., top_downloads: _Optional[_Iterable[_Union[RankedContent, _Mapping]]] = ..., top_questions: _Optional[_Iterable[_Union[RankedQuestion, _Mapping]]] = ..., assistant: _Optional[_Union[AssistantUsage, _Mapping]] = ..., members_by_track: _Optional[_Mapping[str, int]] = ...) -> None: ...

class GetAuditRequest(_message.Message):
    __slots__ = ("actor_id", "action_prefix", "to", "page")
    ACTOR_ID_FIELD_NUMBER: _ClassVar[int]
    ACTION_PREFIX_FIELD_NUMBER: _ClassVar[int]
    FROM_FIELD_NUMBER: _ClassVar[int]
    TO_FIELD_NUMBER: _ClassVar[int]
    PAGE_FIELD_NUMBER: _ClassVar[int]
    actor_id: str
    action_prefix: str
    to: _timestamp_pb2.Timestamp
    page: _common_pb2.PageRequest
    def __init__(self, actor_id: _Optional[str] = ..., action_prefix: _Optional[str] = ..., to: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., page: _Optional[_Union[_common_pb2.PageRequest, _Mapping]] = ..., **kwargs) -> None: ...

class AuditEntry(_message.Message):
    __slots__ = ("id", "actor_id", "action", "target_type", "target_id", "payload_json", "occurred_at")
    ID_FIELD_NUMBER: _ClassVar[int]
    ACTOR_ID_FIELD_NUMBER: _ClassVar[int]
    ACTION_FIELD_NUMBER: _ClassVar[int]
    TARGET_TYPE_FIELD_NUMBER: _ClassVar[int]
    TARGET_ID_FIELD_NUMBER: _ClassVar[int]
    PAYLOAD_JSON_FIELD_NUMBER: _ClassVar[int]
    OCCURRED_AT_FIELD_NUMBER: _ClassVar[int]
    id: str
    actor_id: str
    action: str
    target_type: str
    target_id: str
    payload_json: str
    occurred_at: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[str] = ..., actor_id: _Optional[str] = ..., action: _Optional[str] = ..., target_type: _Optional[str] = ..., target_id: _Optional[str] = ..., payload_json: _Optional[str] = ..., occurred_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class GetAuditResponse(_message.Message):
    __slots__ = ("entries", "page")
    ENTRIES_FIELD_NUMBER: _ClassVar[int]
    PAGE_FIELD_NUMBER: _ClassVar[int]
    entries: _containers.RepeatedCompositeFieldContainer[AuditEntry]
    page: _common_pb2.PageResponse
    def __init__(self, entries: _Optional[_Iterable[_Union[AuditEntry, _Mapping]]] = ..., page: _Optional[_Union[_common_pb2.PageResponse, _Mapping]] = ...) -> None: ...

class SupportMessage(_message.Message):
    __slots__ = ("id", "ticket_id", "category", "status", "subject", "body", "sender_name", "sender_email", "user_id", "created_at", "updated_at", "resolved_at", "hiring_role", "hiring_jd_url", "hiring_target_start")
    ID_FIELD_NUMBER: _ClassVar[int]
    TICKET_ID_FIELD_NUMBER: _ClassVar[int]
    CATEGORY_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    SUBJECT_FIELD_NUMBER: _ClassVar[int]
    BODY_FIELD_NUMBER: _ClassVar[int]
    SENDER_NAME_FIELD_NUMBER: _ClassVar[int]
    SENDER_EMAIL_FIELD_NUMBER: _ClassVar[int]
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    UPDATED_AT_FIELD_NUMBER: _ClassVar[int]
    RESOLVED_AT_FIELD_NUMBER: _ClassVar[int]
    HIRING_ROLE_FIELD_NUMBER: _ClassVar[int]
    HIRING_JD_URL_FIELD_NUMBER: _ClassVar[int]
    HIRING_TARGET_START_FIELD_NUMBER: _ClassVar[int]
    id: str
    ticket_id: str
    category: _contact_pb2.SupportCategory
    status: SupportStatus
    subject: str
    body: str
    sender_name: str
    sender_email: str
    user_id: str
    created_at: _timestamp_pb2.Timestamp
    updated_at: _timestamp_pb2.Timestamp
    resolved_at: _timestamp_pb2.Timestamp
    hiring_role: str
    hiring_jd_url: str
    hiring_target_start: str
    def __init__(self, id: _Optional[str] = ..., ticket_id: _Optional[str] = ..., category: _Optional[_Union[_contact_pb2.SupportCategory, str]] = ..., status: _Optional[_Union[SupportStatus, str]] = ..., subject: _Optional[str] = ..., body: _Optional[str] = ..., sender_name: _Optional[str] = ..., sender_email: _Optional[str] = ..., user_id: _Optional[str] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., updated_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., resolved_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., hiring_role: _Optional[str] = ..., hiring_jd_url: _Optional[str] = ..., hiring_target_start: _Optional[str] = ...) -> None: ...

class ListContactMessagesRequest(_message.Message):
    __slots__ = ("query", "status", "category", "page")
    QUERY_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    CATEGORY_FIELD_NUMBER: _ClassVar[int]
    PAGE_FIELD_NUMBER: _ClassVar[int]
    query: str
    status: SupportStatus
    category: _contact_pb2.SupportCategory
    page: _common_pb2.PageRequest
    def __init__(self, query: _Optional[str] = ..., status: _Optional[_Union[SupportStatus, str]] = ..., category: _Optional[_Union[_contact_pb2.SupportCategory, str]] = ..., page: _Optional[_Union[_common_pb2.PageRequest, _Mapping]] = ...) -> None: ...

class ListContactMessagesResponse(_message.Message):
    __slots__ = ("messages", "page", "open_count", "resolved_count")
    MESSAGES_FIELD_NUMBER: _ClassVar[int]
    PAGE_FIELD_NUMBER: _ClassVar[int]
    OPEN_COUNT_FIELD_NUMBER: _ClassVar[int]
    RESOLVED_COUNT_FIELD_NUMBER: _ClassVar[int]
    messages: _containers.RepeatedCompositeFieldContainer[SupportMessage]
    page: _common_pb2.PageResponse
    open_count: int
    resolved_count: int
    def __init__(self, messages: _Optional[_Iterable[_Union[SupportMessage, _Mapping]]] = ..., page: _Optional[_Union[_common_pb2.PageResponse, _Mapping]] = ..., open_count: _Optional[int] = ..., resolved_count: _Optional[int] = ...) -> None: ...

class ResolveContactMessageRequest(_message.Message):
    __slots__ = ("id", "status")
    ID_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    id: str
    status: SupportStatus
    def __init__(self, id: _Optional[str] = ..., status: _Optional[_Union[SupportStatus, str]] = ...) -> None: ...

class ResolveContactMessageResponse(_message.Message):
    __slots__ = ("message",)
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    message: SupportMessage
    def __init__(self, message: _Optional[_Union[SupportMessage, _Mapping]] = ...) -> None: ...

class ApproveRegistrationRequest(_message.Message):
    __slots__ = ("member_id", "reason")
    MEMBER_ID_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    member_id: str
    reason: str
    def __init__(self, member_id: _Optional[str] = ..., reason: _Optional[str] = ...) -> None: ...

class ApproveRegistrationResponse(_message.Message):
    __slots__ = ("member",)
    MEMBER_FIELD_NUMBER: _ClassVar[int]
    member: MemberRecord
    def __init__(self, member: _Optional[_Union[MemberRecord, _Mapping]] = ...) -> None: ...

class DeclineRegistrationRequest(_message.Message):
    __slots__ = ("member_id", "reason")
    MEMBER_ID_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    member_id: str
    reason: str
    def __init__(self, member_id: _Optional[str] = ..., reason: _Optional[str] = ...) -> None: ...

class DeclineRegistrationResponse(_message.Message):
    __slots__ = ("member",)
    MEMBER_FIELD_NUMBER: _ClassVar[int]
    member: MemberRecord
    def __init__(self, member: _Optional[_Union[MemberRecord, _Mapping]] = ...) -> None: ...

class ExtendAccessRequest(_message.Message):
    __slots__ = ("member_id", "extend_days", "new_expires_at", "permanent", "reason")
    MEMBER_ID_FIELD_NUMBER: _ClassVar[int]
    EXTEND_DAYS_FIELD_NUMBER: _ClassVar[int]
    NEW_EXPIRES_AT_FIELD_NUMBER: _ClassVar[int]
    PERMANENT_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    member_id: str
    extend_days: int
    new_expires_at: _timestamp_pb2.Timestamp
    permanent: bool
    reason: str
    def __init__(self, member_id: _Optional[str] = ..., extend_days: _Optional[int] = ..., new_expires_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., permanent: _Optional[bool] = ..., reason: _Optional[str] = ...) -> None: ...

class ExtendAccessResponse(_message.Message):
    __slots__ = ("member",)
    MEMBER_FIELD_NUMBER: _ClassVar[int]
    member: MemberRecord
    def __init__(self, member: _Optional[_Union[MemberRecord, _Mapping]] = ...) -> None: ...

class DbColumn(_message.Message):
    __slots__ = ("name", "data_type", "nullable")
    NAME_FIELD_NUMBER: _ClassVar[int]
    DATA_TYPE_FIELD_NUMBER: _ClassVar[int]
    NULLABLE_FIELD_NUMBER: _ClassVar[int]
    name: str
    data_type: str
    nullable: bool
    def __init__(self, name: _Optional[str] = ..., data_type: _Optional[str] = ..., nullable: _Optional[bool] = ...) -> None: ...

class DbTable(_message.Message):
    __slots__ = ("name", "columns", "approx_row_count")
    NAME_FIELD_NUMBER: _ClassVar[int]
    COLUMNS_FIELD_NUMBER: _ClassVar[int]
    APPROX_ROW_COUNT_FIELD_NUMBER: _ClassVar[int]
    name: str
    columns: _containers.RepeatedCompositeFieldContainer[DbColumn]
    approx_row_count: int
    def __init__(self, name: _Optional[str] = ..., columns: _Optional[_Iterable[_Union[DbColumn, _Mapping]]] = ..., approx_row_count: _Optional[int] = ...) -> None: ...

class ListDbTablesRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListDbTablesResponse(_message.Message):
    __slots__ = ("tables",)
    TABLES_FIELD_NUMBER: _ClassVar[int]
    tables: _containers.RepeatedCompositeFieldContainer[DbTable]
    def __init__(self, tables: _Optional[_Iterable[_Union[DbTable, _Mapping]]] = ...) -> None: ...

class RunDbQueryRequest(_message.Message):
    __slots__ = ("sql", "timeout_ms", "sort")
    SQL_FIELD_NUMBER: _ClassVar[int]
    TIMEOUT_MS_FIELD_NUMBER: _ClassVar[int]
    SORT_FIELD_NUMBER: _ClassVar[int]
    sql: str
    timeout_ms: int
    sort: str
    def __init__(self, sql: _Optional[str] = ..., timeout_ms: _Optional[int] = ..., sort: _Optional[str] = ...) -> None: ...

class DbRow(_message.Message):
    __slots__ = ("cells", "null_mask")
    CELLS_FIELD_NUMBER: _ClassVar[int]
    NULL_MASK_FIELD_NUMBER: _ClassVar[int]
    cells: _containers.RepeatedScalarFieldContainer[str]
    null_mask: int
    def __init__(self, cells: _Optional[_Iterable[str]] = ..., null_mask: _Optional[int] = ...) -> None: ...

class RunDbQueryResponse(_message.Message):
    __slots__ = ("columns", "column_types", "rows", "truncated", "row_count", "elapsed_ms")
    COLUMNS_FIELD_NUMBER: _ClassVar[int]
    COLUMN_TYPES_FIELD_NUMBER: _ClassVar[int]
    ROWS_FIELD_NUMBER: _ClassVar[int]
    TRUNCATED_FIELD_NUMBER: _ClassVar[int]
    ROW_COUNT_FIELD_NUMBER: _ClassVar[int]
    ELAPSED_MS_FIELD_NUMBER: _ClassVar[int]
    columns: _containers.RepeatedScalarFieldContainer[str]
    column_types: _containers.RepeatedScalarFieldContainer[str]
    rows: _containers.RepeatedCompositeFieldContainer[DbRow]
    truncated: bool
    row_count: int
    elapsed_ms: int
    def __init__(self, columns: _Optional[_Iterable[str]] = ..., column_types: _Optional[_Iterable[str]] = ..., rows: _Optional[_Iterable[_Union[DbRow, _Mapping]]] = ..., truncated: _Optional[bool] = ..., row_count: _Optional[int] = ..., elapsed_ms: _Optional[int] = ...) -> None: ...

class AccessGrant(_message.Message):
    __slots__ = ("id", "email", "default_ttl", "notes", "entry_expires_at", "created_by", "created_at", "updated_at")
    ID_FIELD_NUMBER: _ClassVar[int]
    EMAIL_FIELD_NUMBER: _ClassVar[int]
    DEFAULT_TTL_FIELD_NUMBER: _ClassVar[int]
    NOTES_FIELD_NUMBER: _ClassVar[int]
    ENTRY_EXPIRES_AT_FIELD_NUMBER: _ClassVar[int]
    CREATED_BY_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    UPDATED_AT_FIELD_NUMBER: _ClassVar[int]
    id: str
    email: str
    default_ttl: GrantTTL
    notes: str
    entry_expires_at: _timestamp_pb2.Timestamp
    created_by: str
    created_at: _timestamp_pb2.Timestamp
    updated_at: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[str] = ..., email: _Optional[str] = ..., default_ttl: _Optional[_Union[GrantTTL, str]] = ..., notes: _Optional[str] = ..., entry_expires_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., created_by: _Optional[str] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., updated_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ListAccessGrantsRequest(_message.Message):
    __slots__ = ("query",)
    QUERY_FIELD_NUMBER: _ClassVar[int]
    query: str
    def __init__(self, query: _Optional[str] = ...) -> None: ...

class ListAccessGrantsResponse(_message.Message):
    __slots__ = ("grants", "active_count", "expired_count")
    GRANTS_FIELD_NUMBER: _ClassVar[int]
    ACTIVE_COUNT_FIELD_NUMBER: _ClassVar[int]
    EXPIRED_COUNT_FIELD_NUMBER: _ClassVar[int]
    grants: _containers.RepeatedCompositeFieldContainer[AccessGrant]
    active_count: int
    expired_count: int
    def __init__(self, grants: _Optional[_Iterable[_Union[AccessGrant, _Mapping]]] = ..., active_count: _Optional[int] = ..., expired_count: _Optional[int] = ...) -> None: ...

class UpsertAccessGrantRequest(_message.Message):
    __slots__ = ("email", "default_ttl", "notes", "entry_expires_at")
    EMAIL_FIELD_NUMBER: _ClassVar[int]
    DEFAULT_TTL_FIELD_NUMBER: _ClassVar[int]
    NOTES_FIELD_NUMBER: _ClassVar[int]
    ENTRY_EXPIRES_AT_FIELD_NUMBER: _ClassVar[int]
    email: str
    default_ttl: GrantTTL
    notes: str
    entry_expires_at: _timestamp_pb2.Timestamp
    def __init__(self, email: _Optional[str] = ..., default_ttl: _Optional[_Union[GrantTTL, str]] = ..., notes: _Optional[str] = ..., entry_expires_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class UpsertAccessGrantResponse(_message.Message):
    __slots__ = ("grant", "created")
    GRANT_FIELD_NUMBER: _ClassVar[int]
    CREATED_FIELD_NUMBER: _ClassVar[int]
    grant: AccessGrant
    created: bool
    def __init__(self, grant: _Optional[_Union[AccessGrant, _Mapping]] = ..., created: _Optional[bool] = ...) -> None: ...

class DeleteAccessGrantRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class DeleteAccessGrantResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class SavedQuery(_message.Message):
    __slots__ = ("id", "name", "sql", "created_at", "updated_at")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    SQL_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    UPDATED_AT_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    sql: str
    created_at: _timestamp_pb2.Timestamp
    updated_at: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., sql: _Optional[str] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., updated_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ListSavedQueriesRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListSavedQueriesResponse(_message.Message):
    __slots__ = ("queries",)
    QUERIES_FIELD_NUMBER: _ClassVar[int]
    queries: _containers.RepeatedCompositeFieldContainer[SavedQuery]
    def __init__(self, queries: _Optional[_Iterable[_Union[SavedQuery, _Mapping]]] = ...) -> None: ...

class UpsertSavedQueryRequest(_message.Message):
    __slots__ = ("name", "sql")
    NAME_FIELD_NUMBER: _ClassVar[int]
    SQL_FIELD_NUMBER: _ClassVar[int]
    name: str
    sql: str
    def __init__(self, name: _Optional[str] = ..., sql: _Optional[str] = ...) -> None: ...

class UpsertSavedQueryResponse(_message.Message):
    __slots__ = ("query", "created")
    QUERY_FIELD_NUMBER: _ClassVar[int]
    CREATED_FIELD_NUMBER: _ClassVar[int]
    query: SavedQuery
    created: bool
    def __init__(self, query: _Optional[_Union[SavedQuery, _Mapping]] = ..., created: _Optional[bool] = ...) -> None: ...

class DeleteSavedQueryRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class DeleteSavedQueryResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class MemberActivitySummary(_message.Message):
    __slots__ = ("user_id", "name", "email", "status", "total_sessions", "total_active_secs", "ask_roger_count", "total_events", "last_kind", "last_event_at")
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    EMAIL_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    TOTAL_SESSIONS_FIELD_NUMBER: _ClassVar[int]
    TOTAL_ACTIVE_SECS_FIELD_NUMBER: _ClassVar[int]
    ASK_ROGER_COUNT_FIELD_NUMBER: _ClassVar[int]
    TOTAL_EVENTS_FIELD_NUMBER: _ClassVar[int]
    LAST_KIND_FIELD_NUMBER: _ClassVar[int]
    LAST_EVENT_AT_FIELD_NUMBER: _ClassVar[int]
    user_id: str
    name: str
    email: str
    status: _common_pb2.MemberStatus
    total_sessions: int
    total_active_secs: int
    ask_roger_count: int
    total_events: int
    last_kind: str
    last_event_at: _timestamp_pb2.Timestamp
    def __init__(self, user_id: _Optional[str] = ..., name: _Optional[str] = ..., email: _Optional[str] = ..., status: _Optional[_Union[_common_pb2.MemberStatus, str]] = ..., total_sessions: _Optional[int] = ..., total_active_secs: _Optional[int] = ..., ask_roger_count: _Optional[int] = ..., total_events: _Optional[int] = ..., last_kind: _Optional[str] = ..., last_event_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ListMemberActivityRequest(_message.Message):
    __slots__ = ("sort",)
    SORT_FIELD_NUMBER: _ClassVar[int]
    sort: ActivitySort
    def __init__(self, sort: _Optional[_Union[ActivitySort, str]] = ...) -> None: ...

class ListMemberActivityResponse(_message.Message):
    __slots__ = ("members",)
    MEMBERS_FIELD_NUMBER: _ClassVar[int]
    members: _containers.RepeatedCompositeFieldContainer[MemberActivitySummary]
    def __init__(self, members: _Optional[_Iterable[_Union[MemberActivitySummary, _Mapping]]] = ...) -> None: ...

class IngestCorpusTextRequest(_message.Message):
    __slots__ = ("source_kind", "source_path", "title", "body", "visibility")
    SOURCE_KIND_FIELD_NUMBER: _ClassVar[int]
    SOURCE_PATH_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    BODY_FIELD_NUMBER: _ClassVar[int]
    VISIBILITY_FIELD_NUMBER: _ClassVar[int]
    source_kind: str
    source_path: str
    title: str
    body: str
    visibility: str
    def __init__(self, source_kind: _Optional[str] = ..., source_path: _Optional[str] = ..., title: _Optional[str] = ..., body: _Optional[str] = ..., visibility: _Optional[str] = ...) -> None: ...

class IngestCorpusTextResponse(_message.Message):
    __slots__ = ("document_id", "chunks_inserted", "chunks_embedded", "skipped", "chunker_name", "embedder_model")
    DOCUMENT_ID_FIELD_NUMBER: _ClassVar[int]
    CHUNKS_INSERTED_FIELD_NUMBER: _ClassVar[int]
    CHUNKS_EMBEDDED_FIELD_NUMBER: _ClassVar[int]
    SKIPPED_FIELD_NUMBER: _ClassVar[int]
    CHUNKER_NAME_FIELD_NUMBER: _ClassVar[int]
    EMBEDDER_MODEL_FIELD_NUMBER: _ClassVar[int]
    document_id: str
    chunks_inserted: int
    chunks_embedded: int
    skipped: bool
    chunker_name: str
    embedder_model: str
    def __init__(self, document_id: _Optional[str] = ..., chunks_inserted: _Optional[int] = ..., chunks_embedded: _Optional[int] = ..., skipped: _Optional[bool] = ..., chunker_name: _Optional[str] = ..., embedder_model: _Optional[str] = ...) -> None: ...

class ListCorpusDocumentsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class CorpusDocumentRow(_message.Message):
    __slots__ = ("id", "source_kind", "source_path", "title", "chunk_count", "embedded_count", "ingested_at", "updated_at", "visibility")
    ID_FIELD_NUMBER: _ClassVar[int]
    SOURCE_KIND_FIELD_NUMBER: _ClassVar[int]
    SOURCE_PATH_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    CHUNK_COUNT_FIELD_NUMBER: _ClassVar[int]
    EMBEDDED_COUNT_FIELD_NUMBER: _ClassVar[int]
    INGESTED_AT_FIELD_NUMBER: _ClassVar[int]
    UPDATED_AT_FIELD_NUMBER: _ClassVar[int]
    VISIBILITY_FIELD_NUMBER: _ClassVar[int]
    id: str
    source_kind: str
    source_path: str
    title: str
    chunk_count: int
    embedded_count: int
    ingested_at: _timestamp_pb2.Timestamp
    updated_at: _timestamp_pb2.Timestamp
    visibility: str
    def __init__(self, id: _Optional[str] = ..., source_kind: _Optional[str] = ..., source_path: _Optional[str] = ..., title: _Optional[str] = ..., chunk_count: _Optional[int] = ..., embedded_count: _Optional[int] = ..., ingested_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., updated_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., visibility: _Optional[str] = ...) -> None: ...

class ListCorpusDocumentsResponse(_message.Message):
    __slots__ = ("documents", "total_documents", "total_chunks", "total_embedded", "total_public", "total_corpus_only", "embedder_counts")
    DOCUMENTS_FIELD_NUMBER: _ClassVar[int]
    TOTAL_DOCUMENTS_FIELD_NUMBER: _ClassVar[int]
    TOTAL_CHUNKS_FIELD_NUMBER: _ClassVar[int]
    TOTAL_EMBEDDED_FIELD_NUMBER: _ClassVar[int]
    TOTAL_PUBLIC_FIELD_NUMBER: _ClassVar[int]
    TOTAL_CORPUS_ONLY_FIELD_NUMBER: _ClassVar[int]
    EMBEDDER_COUNTS_FIELD_NUMBER: _ClassVar[int]
    documents: _containers.RepeatedCompositeFieldContainer[CorpusDocumentRow]
    total_documents: int
    total_chunks: int
    total_embedded: int
    total_public: int
    total_corpus_only: int
    embedder_counts: _containers.RepeatedCompositeFieldContainer[EmbedderCount]
    def __init__(self, documents: _Optional[_Iterable[_Union[CorpusDocumentRow, _Mapping]]] = ..., total_documents: _Optional[int] = ..., total_chunks: _Optional[int] = ..., total_embedded: _Optional[int] = ..., total_public: _Optional[int] = ..., total_corpus_only: _Optional[int] = ..., embedder_counts: _Optional[_Iterable[_Union[EmbedderCount, _Mapping]]] = ...) -> None: ...

class EmbedderCount(_message.Message):
    __slots__ = ("model", "count")
    MODEL_FIELD_NUMBER: _ClassVar[int]
    COUNT_FIELD_NUMBER: _ClassVar[int]
    model: str
    count: int
    def __init__(self, model: _Optional[str] = ..., count: _Optional[int] = ...) -> None: ...

class SweepCorpusEmbeddingsRequest(_message.Message):
    __slots__ = ("max_chunks",)
    MAX_CHUNKS_FIELD_NUMBER: _ClassVar[int]
    max_chunks: int
    def __init__(self, max_chunks: _Optional[int] = ...) -> None: ...

class SweepCorpusEmbeddingsResponse(_message.Message):
    __slots__ = ("model", "considered", "embedded", "failed", "remaining")
    MODEL_FIELD_NUMBER: _ClassVar[int]
    CONSIDERED_FIELD_NUMBER: _ClassVar[int]
    EMBEDDED_FIELD_NUMBER: _ClassVar[int]
    FAILED_FIELD_NUMBER: _ClassVar[int]
    REMAINING_FIELD_NUMBER: _ClassVar[int]
    model: str
    considered: int
    embedded: int
    failed: int
    remaining: int
    def __init__(self, model: _Optional[str] = ..., considered: _Optional[int] = ..., embedded: _Optional[int] = ..., failed: _Optional[int] = ..., remaining: _Optional[int] = ...) -> None: ...

class GetJdSubmissionRequest(_message.Message):
    __slots__ = ("submission_id",)
    SUBMISSION_ID_FIELD_NUMBER: _ClassVar[int]
    submission_id: str
    def __init__(self, submission_id: _Optional[str] = ...) -> None: ...

class GetJdSubmissionResponse(_message.Message):
    __slots__ = ("row", "jd_text", "assessment_json", "resume_markdown", "llm_model", "prompt_id", "prompt_version", "download_url")
    ROW_FIELD_NUMBER: _ClassVar[int]
    JD_TEXT_FIELD_NUMBER: _ClassVar[int]
    ASSESSMENT_JSON_FIELD_NUMBER: _ClassVar[int]
    RESUME_MARKDOWN_FIELD_NUMBER: _ClassVar[int]
    LLM_MODEL_FIELD_NUMBER: _ClassVar[int]
    PROMPT_ID_FIELD_NUMBER: _ClassVar[int]
    PROMPT_VERSION_FIELD_NUMBER: _ClassVar[int]
    DOWNLOAD_URL_FIELD_NUMBER: _ClassVar[int]
    row: JdSubmissionRow
    jd_text: str
    assessment_json: str
    resume_markdown: str
    llm_model: str
    prompt_id: str
    prompt_version: int
    download_url: str
    def __init__(self, row: _Optional[_Union[JdSubmissionRow, _Mapping]] = ..., jd_text: _Optional[str] = ..., assessment_json: _Optional[str] = ..., resume_markdown: _Optional[str] = ..., llm_model: _Optional[str] = ..., prompt_id: _Optional[str] = ..., prompt_version: _Optional[int] = ..., download_url: _Optional[str] = ...) -> None: ...

class RescoreJdRequest(_message.Message):
    __slots__ = ("submission_id",)
    SUBMISSION_ID_FIELD_NUMBER: _ClassVar[int]
    submission_id: str
    def __init__(self, submission_id: _Optional[str] = ...) -> None: ...

class RescoreJdResponse(_message.Message):
    __slots__ = ("status",)
    STATUS_FIELD_NUMBER: _ClassVar[int]
    status: _jd_pb2.JdStatus
    def __init__(self, status: _Optional[_Union[_jd_pb2.JdStatus, str]] = ...) -> None: ...

class ReindexCorpusRequest(_message.Message):
    __slots__ = ("source_kind", "scope")
    SOURCE_KIND_FIELD_NUMBER: _ClassVar[int]
    SCOPE_FIELD_NUMBER: _ClassVar[int]
    source_kind: str
    scope: str
    def __init__(self, source_kind: _Optional[str] = ..., scope: _Optional[str] = ...) -> None: ...

class ReindexCorpusResponse(_message.Message):
    __slots__ = ("root", "visibility", "kinds_walked", "files_scanned", "docs_ingested", "docs_skipped", "chunks_inserted", "chunks_embedded", "errors")
    ROOT_FIELD_NUMBER: _ClassVar[int]
    VISIBILITY_FIELD_NUMBER: _ClassVar[int]
    KINDS_WALKED_FIELD_NUMBER: _ClassVar[int]
    FILES_SCANNED_FIELD_NUMBER: _ClassVar[int]
    DOCS_INGESTED_FIELD_NUMBER: _ClassVar[int]
    DOCS_SKIPPED_FIELD_NUMBER: _ClassVar[int]
    CHUNKS_INSERTED_FIELD_NUMBER: _ClassVar[int]
    CHUNKS_EMBEDDED_FIELD_NUMBER: _ClassVar[int]
    ERRORS_FIELD_NUMBER: _ClassVar[int]
    root: str
    visibility: str
    kinds_walked: _containers.RepeatedScalarFieldContainer[str]
    files_scanned: int
    docs_ingested: int
    docs_skipped: int
    chunks_inserted: int
    chunks_embedded: int
    errors: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, root: _Optional[str] = ..., visibility: _Optional[str] = ..., kinds_walked: _Optional[_Iterable[str]] = ..., files_scanned: _Optional[int] = ..., docs_ingested: _Optional[int] = ..., docs_skipped: _Optional[int] = ..., chunks_inserted: _Optional[int] = ..., chunks_embedded: _Optional[int] = ..., errors: _Optional[_Iterable[str]] = ...) -> None: ...

class ListJdSubmissionsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class JdSubmissionRow(_message.Message):
    __slots__ = ("id", "status", "match_score", "text_head", "role_hint", "employer_hint", "contact_email", "source", "error_message", "generated_resume_url", "created_at", "completed_at", "retrieval_score", "submitter_email")
    ID_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    MATCH_SCORE_FIELD_NUMBER: _ClassVar[int]
    TEXT_HEAD_FIELD_NUMBER: _ClassVar[int]
    ROLE_HINT_FIELD_NUMBER: _ClassVar[int]
    EMPLOYER_HINT_FIELD_NUMBER: _ClassVar[int]
    CONTACT_EMAIL_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    ERROR_MESSAGE_FIELD_NUMBER: _ClassVar[int]
    GENERATED_RESUME_URL_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    COMPLETED_AT_FIELD_NUMBER: _ClassVar[int]
    RETRIEVAL_SCORE_FIELD_NUMBER: _ClassVar[int]
    SUBMITTER_EMAIL_FIELD_NUMBER: _ClassVar[int]
    id: str
    status: _jd_pb2.JdStatus
    match_score: float
    text_head: str
    role_hint: str
    employer_hint: str
    contact_email: str
    source: _jd_pb2.JdSource
    error_message: str
    generated_resume_url: str
    created_at: _timestamp_pb2.Timestamp
    completed_at: _timestamp_pb2.Timestamp
    retrieval_score: float
    submitter_email: str
    def __init__(self, id: _Optional[str] = ..., status: _Optional[_Union[_jd_pb2.JdStatus, str]] = ..., match_score: _Optional[float] = ..., text_head: _Optional[str] = ..., role_hint: _Optional[str] = ..., employer_hint: _Optional[str] = ..., contact_email: _Optional[str] = ..., source: _Optional[_Union[_jd_pb2.JdSource, str]] = ..., error_message: _Optional[str] = ..., generated_resume_url: _Optional[str] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., completed_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., retrieval_score: _Optional[float] = ..., submitter_email: _Optional[str] = ...) -> None: ...

class ListJdSubmissionsResponse(_message.Message):
    __slots__ = ("submissions", "ready_count", "below_threshold_count", "failed_count", "in_flight_count")
    SUBMISSIONS_FIELD_NUMBER: _ClassVar[int]
    READY_COUNT_FIELD_NUMBER: _ClassVar[int]
    BELOW_THRESHOLD_COUNT_FIELD_NUMBER: _ClassVar[int]
    FAILED_COUNT_FIELD_NUMBER: _ClassVar[int]
    IN_FLIGHT_COUNT_FIELD_NUMBER: _ClassVar[int]
    submissions: _containers.RepeatedCompositeFieldContainer[JdSubmissionRow]
    ready_count: int
    below_threshold_count: int
    failed_count: int
    in_flight_count: int
    def __init__(self, submissions: _Optional[_Iterable[_Union[JdSubmissionRow, _Mapping]]] = ..., ready_count: _Optional[int] = ..., below_threshold_count: _Optional[int] = ..., failed_count: _Optional[int] = ..., in_flight_count: _Optional[int] = ...) -> None: ...
