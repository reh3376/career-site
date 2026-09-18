import datetime

from buf.validate import validate_pb2 as _validate_pb2
from career.v1 import chat_pb2 as _chat_pb2
from career.v1 import common_pb2 as _common_pb2
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

class GetMemberResponse(_message.Message):
    __slots__ = ("member", "recent_activity", "conversations", "notes")
    MEMBER_FIELD_NUMBER: _ClassVar[int]
    RECENT_ACTIVITY_FIELD_NUMBER: _ClassVar[int]
    CONVERSATIONS_FIELD_NUMBER: _ClassVar[int]
    NOTES_FIELD_NUMBER: _ClassVar[int]
    member: MemberRecord
    recent_activity: _containers.RepeatedCompositeFieldContainer[_common_pb2.ActivityEvent]
    conversations: _containers.RepeatedCompositeFieldContainer[_chat_pb2.Conversation]
    notes: _containers.RepeatedCompositeFieldContainer[AdminNote]
    def __init__(self, member: _Optional[_Union[MemberRecord, _Mapping]] = ..., recent_activity: _Optional[_Iterable[_Union[_common_pb2.ActivityEvent, _Mapping]]] = ..., conversations: _Optional[_Iterable[_Union[_chat_pb2.Conversation, _Mapping]]] = ..., notes: _Optional[_Iterable[_Union[AdminNote, _Mapping]]] = ...) -> None: ...

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
