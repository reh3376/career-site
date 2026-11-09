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

class PilotSession(_message.Message):
    __slots__ = ("id", "status", "role_branch", "instrument_version", "consent_at", "recording_ok", "survey_finished_at", "record_finished_at", "workflow_finished_at", "finished_at", "sections_done", "next_section")
    ID_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    ROLE_BRANCH_FIELD_NUMBER: _ClassVar[int]
    INSTRUMENT_VERSION_FIELD_NUMBER: _ClassVar[int]
    CONSENT_AT_FIELD_NUMBER: _ClassVar[int]
    RECORDING_OK_FIELD_NUMBER: _ClassVar[int]
    SURVEY_FINISHED_AT_FIELD_NUMBER: _ClassVar[int]
    RECORD_FINISHED_AT_FIELD_NUMBER: _ClassVar[int]
    WORKFLOW_FINISHED_AT_FIELD_NUMBER: _ClassVar[int]
    FINISHED_AT_FIELD_NUMBER: _ClassVar[int]
    SECTIONS_DONE_FIELD_NUMBER: _ClassVar[int]
    NEXT_SECTION_FIELD_NUMBER: _ClassVar[int]
    id: str
    status: str
    role_branch: str
    instrument_version: str
    consent_at: _timestamp_pb2.Timestamp
    recording_ok: bool
    survey_finished_at: _timestamp_pb2.Timestamp
    record_finished_at: _timestamp_pb2.Timestamp
    workflow_finished_at: _timestamp_pb2.Timestamp
    finished_at: _timestamp_pb2.Timestamp
    sections_done: _containers.RepeatedScalarFieldContainer[str]
    next_section: str
    def __init__(self, id: _Optional[str] = ..., status: _Optional[str] = ..., role_branch: _Optional[str] = ..., instrument_version: _Optional[str] = ..., consent_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., recording_ok: _Optional[bool] = ..., survey_finished_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., record_finished_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., workflow_finished_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., finished_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., sections_done: _Optional[_Iterable[str]] = ..., next_section: _Optional[str] = ...) -> None: ...

class GetPilotRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetPilotResponse(_message.Message):
    __slots__ = ("allowed", "admin", "instrument_version", "session")
    ALLOWED_FIELD_NUMBER: _ClassVar[int]
    ADMIN_FIELD_NUMBER: _ClassVar[int]
    INSTRUMENT_VERSION_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    allowed: bool
    admin: bool
    instrument_version: str
    session: PilotSession
    def __init__(self, allowed: _Optional[bool] = ..., admin: _Optional[bool] = ..., instrument_version: _Optional[str] = ..., session: _Optional[_Union[PilotSession, _Mapping]] = ...) -> None: ...

class StartPilotRequest(_message.Message):
    __slots__ = ("consent", "recording_ok")
    CONSENT_FIELD_NUMBER: _ClassVar[int]
    RECORDING_OK_FIELD_NUMBER: _ClassVar[int]
    consent: bool
    recording_ok: bool
    def __init__(self, consent: _Optional[bool] = ..., recording_ok: _Optional[bool] = ...) -> None: ...

class StartPilotResponse(_message.Message):
    __slots__ = ("session",)
    SESSION_FIELD_NUMBER: _ClassVar[int]
    session: PilotSession
    def __init__(self, session: _Optional[_Union[PilotSession, _Mapping]] = ...) -> None: ...

class PilotScale(_message.Message):
    __slots__ = ("min", "max", "min_label", "max_label")
    MIN_FIELD_NUMBER: _ClassVar[int]
    MAX_FIELD_NUMBER: _ClassVar[int]
    MIN_LABEL_FIELD_NUMBER: _ClassVar[int]
    MAX_LABEL_FIELD_NUMBER: _ClassVar[int]
    min: int
    max: int
    min_label: str
    max_label: str
    def __init__(self, min: _Optional[int] = ..., max: _Optional[int] = ..., min_label: _Optional[str] = ..., max_label: _Optional[str] = ...) -> None: ...

class PilotDimension(_message.Message):
    __slots__ = ("id", "label", "options")
    ID_FIELD_NUMBER: _ClassVar[int]
    LABEL_FIELD_NUMBER: _ClassVar[int]
    OPTIONS_FIELD_NUMBER: _ClassVar[int]
    id: str
    label: str
    options: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, id: _Optional[str] = ..., label: _Optional[str] = ..., options: _Optional[_Iterable[str]] = ...) -> None: ...

class PilotItem(_message.Message):
    __slots__ = ("id", "type", "prompt", "help", "options", "max_select", "required", "scale", "dimensions", "pilot_only", "source_item")
    ID_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    PROMPT_FIELD_NUMBER: _ClassVar[int]
    HELP_FIELD_NUMBER: _ClassVar[int]
    OPTIONS_FIELD_NUMBER: _ClassVar[int]
    MAX_SELECT_FIELD_NUMBER: _ClassVar[int]
    REQUIRED_FIELD_NUMBER: _ClassVar[int]
    SCALE_FIELD_NUMBER: _ClassVar[int]
    DIMENSIONS_FIELD_NUMBER: _ClassVar[int]
    PILOT_ONLY_FIELD_NUMBER: _ClassVar[int]
    SOURCE_ITEM_FIELD_NUMBER: _ClassVar[int]
    id: str
    type: str
    prompt: str
    help: str
    options: _containers.RepeatedScalarFieldContainer[str]
    max_select: int
    required: bool
    scale: PilotScale
    dimensions: _containers.RepeatedCompositeFieldContainer[PilotDimension]
    pilot_only: bool
    source_item: str
    def __init__(self, id: _Optional[str] = ..., type: _Optional[str] = ..., prompt: _Optional[str] = ..., help: _Optional[str] = ..., options: _Optional[_Iterable[str]] = ..., max_select: _Optional[int] = ..., required: _Optional[bool] = ..., scale: _Optional[_Union[PilotScale, _Mapping]] = ..., dimensions: _Optional[_Iterable[_Union[PilotDimension, _Mapping]]] = ..., pilot_only: _Optional[bool] = ..., source_item: _Optional[str] = ...) -> None: ...

class PilotAnswer(_message.Message):
    __slots__ = ("item_id", "value_json", "flagged", "prompt", "section", "note")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    VALUE_JSON_FIELD_NUMBER: _ClassVar[int]
    FLAGGED_FIELD_NUMBER: _ClassVar[int]
    PROMPT_FIELD_NUMBER: _ClassVar[int]
    SECTION_FIELD_NUMBER: _ClassVar[int]
    NOTE_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    value_json: str
    flagged: bool
    prompt: str
    section: str
    note: str
    def __init__(self, item_id: _Optional[str] = ..., value_json: _Optional[str] = ..., flagged: _Optional[bool] = ..., prompt: _Optional[str] = ..., section: _Optional[str] = ..., note: _Optional[str] = ...) -> None: ...

class GetPilotSectionRequest(_message.Message):
    __slots__ = ("section_id",)
    SECTION_ID_FIELD_NUMBER: _ClassVar[int]
    section_id: str
    def __init__(self, section_id: _Optional[str] = ...) -> None: ...

class GetPilotSectionResponse(_message.Message):
    __slots__ = ("id", "title", "intro", "items", "answers", "index", "total", "session")
    ID_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    INTRO_FIELD_NUMBER: _ClassVar[int]
    ITEMS_FIELD_NUMBER: _ClassVar[int]
    ANSWERS_FIELD_NUMBER: _ClassVar[int]
    INDEX_FIELD_NUMBER: _ClassVar[int]
    TOTAL_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    id: str
    title: str
    intro: str
    items: _containers.RepeatedCompositeFieldContainer[PilotItem]
    answers: _containers.RepeatedCompositeFieldContainer[PilotAnswer]
    index: int
    total: int
    session: PilotSession
    def __init__(self, id: _Optional[str] = ..., title: _Optional[str] = ..., intro: _Optional[str] = ..., items: _Optional[_Iterable[_Union[PilotItem, _Mapping]]] = ..., answers: _Optional[_Iterable[_Union[PilotAnswer, _Mapping]]] = ..., index: _Optional[int] = ..., total: _Optional[int] = ..., session: _Optional[_Union[PilotSession, _Mapping]] = ...) -> None: ...

class SubmitPilotSectionRequest(_message.Message):
    __slots__ = ("section_id", "answers")
    SECTION_ID_FIELD_NUMBER: _ClassVar[int]
    ANSWERS_FIELD_NUMBER: _ClassVar[int]
    section_id: str
    answers: _containers.RepeatedCompositeFieldContainer[PilotAnswer]
    def __init__(self, section_id: _Optional[str] = ..., answers: _Optional[_Iterable[_Union[PilotAnswer, _Mapping]]] = ...) -> None: ...

class SubmitPilotSectionResponse(_message.Message):
    __slots__ = ("session", "ended")
    SESSION_FIELD_NUMBER: _ClassVar[int]
    ENDED_FIELD_NUMBER: _ClassVar[int]
    session: PilotSession
    ended: bool
    def __init__(self, session: _Optional[_Union[PilotSession, _Mapping]] = ..., ended: _Optional[bool] = ...) -> None: ...

class GetPilotRecordRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetPilotRecordResponse(_message.Message):
    __slots__ = ("record_json", "comments", "record_version")
    RECORD_JSON_FIELD_NUMBER: _ClassVar[int]
    COMMENTS_FIELD_NUMBER: _ClassVar[int]
    RECORD_VERSION_FIELD_NUMBER: _ClassVar[int]
    record_json: str
    comments: _containers.RepeatedCompositeFieldContainer[PilotComment]
    record_version: str
    def __init__(self, record_json: _Optional[str] = ..., comments: _Optional[_Iterable[_Union[PilotComment, _Mapping]]] = ..., record_version: _Optional[str] = ...) -> None: ...

class GetPilotWorkflowRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class PilotWorkflowNode(_message.Message):
    __slots__ = ("id", "phase", "name", "owner", "summary", "decision", "open")
    ID_FIELD_NUMBER: _ClassVar[int]
    PHASE_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    OWNER_FIELD_NUMBER: _ClassVar[int]
    SUMMARY_FIELD_NUMBER: _ClassVar[int]
    DECISION_FIELD_NUMBER: _ClassVar[int]
    OPEN_FIELD_NUMBER: _ClassVar[int]
    id: str
    phase: str
    name: str
    owner: str
    summary: str
    decision: bool
    open: bool
    def __init__(self, id: _Optional[str] = ..., phase: _Optional[str] = ..., name: _Optional[str] = ..., owner: _Optional[str] = ..., summary: _Optional[str] = ..., decision: _Optional[bool] = ..., open: _Optional[bool] = ...) -> None: ...

class PilotWorkflowPhase(_message.Message):
    __slots__ = ("id", "title")
    ID_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    id: str
    title: str
    def __init__(self, id: _Optional[str] = ..., title: _Optional[str] = ...) -> None: ...

class GetPilotWorkflowResponse(_message.Message):
    __slots__ = ("intro", "prompts", "phases", "nodes", "comments", "workflow_version")
    INTRO_FIELD_NUMBER: _ClassVar[int]
    PROMPTS_FIELD_NUMBER: _ClassVar[int]
    PHASES_FIELD_NUMBER: _ClassVar[int]
    NODES_FIELD_NUMBER: _ClassVar[int]
    COMMENTS_FIELD_NUMBER: _ClassVar[int]
    WORKFLOW_VERSION_FIELD_NUMBER: _ClassVar[int]
    intro: str
    prompts: _containers.RepeatedScalarFieldContainer[str]
    phases: _containers.RepeatedCompositeFieldContainer[PilotWorkflowPhase]
    nodes: _containers.RepeatedCompositeFieldContainer[PilotWorkflowNode]
    comments: _containers.RepeatedCompositeFieldContainer[PilotComment]
    workflow_version: str
    def __init__(self, intro: _Optional[str] = ..., prompts: _Optional[_Iterable[str]] = ..., phases: _Optional[_Iterable[_Union[PilotWorkflowPhase, _Mapping]]] = ..., nodes: _Optional[_Iterable[_Union[PilotWorkflowNode, _Mapping]]] = ..., comments: _Optional[_Iterable[_Union[PilotComment, _Mapping]]] = ..., workflow_version: _Optional[str] = ...) -> None: ...

class PilotComment(_message.Message):
    __slots__ = ("target_kind", "target_id", "body", "updated_at")
    TARGET_KIND_FIELD_NUMBER: _ClassVar[int]
    TARGET_ID_FIELD_NUMBER: _ClassVar[int]
    BODY_FIELD_NUMBER: _ClassVar[int]
    UPDATED_AT_FIELD_NUMBER: _ClassVar[int]
    target_kind: str
    target_id: str
    body: str
    updated_at: _timestamp_pb2.Timestamp
    def __init__(self, target_kind: _Optional[str] = ..., target_id: _Optional[str] = ..., body: _Optional[str] = ..., updated_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class UpsertPilotCommentRequest(_message.Message):
    __slots__ = ("target_kind", "target_id", "body")
    TARGET_KIND_FIELD_NUMBER: _ClassVar[int]
    TARGET_ID_FIELD_NUMBER: _ClassVar[int]
    BODY_FIELD_NUMBER: _ClassVar[int]
    target_kind: str
    target_id: str
    body: str
    def __init__(self, target_kind: _Optional[str] = ..., target_id: _Optional[str] = ..., body: _Optional[str] = ...) -> None: ...

class UpsertPilotCommentResponse(_message.Message):
    __slots__ = ("comment",)
    COMMENT_FIELD_NUMBER: _ClassVar[int]
    comment: PilotComment
    def __init__(self, comment: _Optional[_Union[PilotComment, _Mapping]] = ...) -> None: ...

class MarkPilotPartRequest(_message.Message):
    __slots__ = ("part",)
    PART_FIELD_NUMBER: _ClassVar[int]
    part: str
    def __init__(self, part: _Optional[str] = ...) -> None: ...

class MarkPilotPartResponse(_message.Message):
    __slots__ = ("session",)
    SESSION_FIELD_NUMBER: _ClassVar[int]
    session: PilotSession
    def __init__(self, session: _Optional[_Union[PilotSession, _Mapping]] = ...) -> None: ...

class FinishPilotRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class FinishPilotResponse(_message.Message):
    __slots__ = ("session",)
    SESSION_FIELD_NUMBER: _ClassVar[int]
    session: PilotSession
    def __init__(self, session: _Optional[_Union[PilotSession, _Mapping]] = ...) -> None: ...

class ListPilotSessionsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class PilotSessionSummary(_message.Message):
    __slots__ = ("session", "member_id", "name", "email", "measured_minutes", "self_reported_minutes", "comment_count", "flag_count", "review_status", "review_note", "reviewed_at")
    SESSION_FIELD_NUMBER: _ClassVar[int]
    MEMBER_ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    EMAIL_FIELD_NUMBER: _ClassVar[int]
    MEASURED_MINUTES_FIELD_NUMBER: _ClassVar[int]
    SELF_REPORTED_MINUTES_FIELD_NUMBER: _ClassVar[int]
    COMMENT_COUNT_FIELD_NUMBER: _ClassVar[int]
    FLAG_COUNT_FIELD_NUMBER: _ClassVar[int]
    REVIEW_STATUS_FIELD_NUMBER: _ClassVar[int]
    REVIEW_NOTE_FIELD_NUMBER: _ClassVar[int]
    REVIEWED_AT_FIELD_NUMBER: _ClassVar[int]
    session: PilotSession
    member_id: str
    name: str
    email: str
    measured_minutes: int
    self_reported_minutes: int
    comment_count: int
    flag_count: int
    review_status: str
    review_note: str
    reviewed_at: _timestamp_pb2.Timestamp
    def __init__(self, session: _Optional[_Union[PilotSession, _Mapping]] = ..., member_id: _Optional[str] = ..., name: _Optional[str] = ..., email: _Optional[str] = ..., measured_minutes: _Optional[int] = ..., self_reported_minutes: _Optional[int] = ..., comment_count: _Optional[int] = ..., flag_count: _Optional[int] = ..., review_status: _Optional[str] = ..., review_note: _Optional[str] = ..., reviewed_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ListPilotSessionsResponse(_message.Message):
    __slots__ = ("sessions", "instrument_version")
    SESSIONS_FIELD_NUMBER: _ClassVar[int]
    INSTRUMENT_VERSION_FIELD_NUMBER: _ClassVar[int]
    sessions: _containers.RepeatedCompositeFieldContainer[PilotSessionSummary]
    instrument_version: str
    def __init__(self, sessions: _Optional[_Iterable[_Union[PilotSessionSummary, _Mapping]]] = ..., instrument_version: _Optional[str] = ...) -> None: ...

class GetPilotSessionRequest(_message.Message):
    __slots__ = ("session_id",)
    SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    session_id: str
    def __init__(self, session_id: _Optional[str] = ...) -> None: ...

class PilotSectionEvent(_message.Message):
    __slots__ = ("section", "event", "at")
    SECTION_FIELD_NUMBER: _ClassVar[int]
    EVENT_FIELD_NUMBER: _ClassVar[int]
    AT_FIELD_NUMBER: _ClassVar[int]
    section: str
    event: str
    at: _timestamp_pb2.Timestamp
    def __init__(self, section: _Optional[str] = ..., event: _Optional[str] = ..., at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class GetPilotSessionResponse(_message.Message):
    __slots__ = ("summary", "answers", "comments", "events", "notes")
    SUMMARY_FIELD_NUMBER: _ClassVar[int]
    ANSWERS_FIELD_NUMBER: _ClassVar[int]
    COMMENTS_FIELD_NUMBER: _ClassVar[int]
    EVENTS_FIELD_NUMBER: _ClassVar[int]
    NOTES_FIELD_NUMBER: _ClassVar[int]
    summary: PilotSessionSummary
    answers: _containers.RepeatedCompositeFieldContainer[PilotAnswer]
    comments: _containers.RepeatedCompositeFieldContainer[PilotComment]
    events: _containers.RepeatedCompositeFieldContainer[PilotSectionEvent]
    notes: _containers.RepeatedCompositeFieldContainer[PilotNote]
    def __init__(self, summary: _Optional[_Union[PilotSessionSummary, _Mapping]] = ..., answers: _Optional[_Iterable[_Union[PilotAnswer, _Mapping]]] = ..., comments: _Optional[_Iterable[_Union[PilotComment, _Mapping]]] = ..., events: _Optional[_Iterable[_Union[PilotSectionEvent, _Mapping]]] = ..., notes: _Optional[_Iterable[_Union[PilotNote, _Mapping]]] = ...) -> None: ...

class ExportPilotRequest(_message.Message):
    __slots__ = ("table",)
    TABLE_FIELD_NUMBER: _ClassVar[int]
    table: str
    def __init__(self, table: _Optional[str] = ...) -> None: ...

class ExportPilotResponse(_message.Message):
    __slots__ = ("csv", "row_count")
    CSV_FIELD_NUMBER: _ClassVar[int]
    ROW_COUNT_FIELD_NUMBER: _ClassVar[int]
    csv: str
    row_count: int
    def __init__(self, csv: _Optional[str] = ..., row_count: _Optional[int] = ...) -> None: ...

class DeletePilotSessionRequest(_message.Message):
    __slots__ = ("session_id",)
    SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    session_id: str
    def __init__(self, session_id: _Optional[str] = ...) -> None: ...

class DeletePilotSessionResponse(_message.Message):
    __slots__ = ("member_id",)
    MEMBER_ID_FIELD_NUMBER: _ClassVar[int]
    member_id: str
    def __init__(self, member_id: _Optional[str] = ...) -> None: ...

class PilotNote(_message.Message):
    __slots__ = ("target_kind", "target_id", "body", "updated_at", "author")
    TARGET_KIND_FIELD_NUMBER: _ClassVar[int]
    TARGET_ID_FIELD_NUMBER: _ClassVar[int]
    BODY_FIELD_NUMBER: _ClassVar[int]
    UPDATED_AT_FIELD_NUMBER: _ClassVar[int]
    AUTHOR_FIELD_NUMBER: _ClassVar[int]
    target_kind: str
    target_id: str
    body: str
    updated_at: _timestamp_pb2.Timestamp
    author: str
    def __init__(self, target_kind: _Optional[str] = ..., target_id: _Optional[str] = ..., body: _Optional[str] = ..., updated_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., author: _Optional[str] = ...) -> None: ...

class UpsertPilotNoteRequest(_message.Message):
    __slots__ = ("session_id", "target_kind", "target_id", "body")
    SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    TARGET_KIND_FIELD_NUMBER: _ClassVar[int]
    TARGET_ID_FIELD_NUMBER: _ClassVar[int]
    BODY_FIELD_NUMBER: _ClassVar[int]
    session_id: str
    target_kind: str
    target_id: str
    body: str
    def __init__(self, session_id: _Optional[str] = ..., target_kind: _Optional[str] = ..., target_id: _Optional[str] = ..., body: _Optional[str] = ...) -> None: ...

class UpsertPilotNoteResponse(_message.Message):
    __slots__ = ("updated_at", "removed")
    UPDATED_AT_FIELD_NUMBER: _ClassVar[int]
    REMOVED_FIELD_NUMBER: _ClassVar[int]
    updated_at: _timestamp_pb2.Timestamp
    removed: bool
    def __init__(self, updated_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., removed: _Optional[bool] = ...) -> None: ...

class SetPilotSessionReviewRequest(_message.Message):
    __slots__ = ("session_id", "status", "note")
    SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    NOTE_FIELD_NUMBER: _ClassVar[int]
    session_id: str
    status: str
    note: str
    def __init__(self, session_id: _Optional[str] = ..., status: _Optional[str] = ..., note: _Optional[str] = ...) -> None: ...

class SetPilotSessionReviewResponse(_message.Message):
    __slots__ = ("summary",)
    SUMMARY_FIELD_NUMBER: _ClassVar[int]
    summary: PilotSessionSummary
    def __init__(self, summary: _Optional[_Union[PilotSessionSummary, _Mapping]] = ...) -> None: ...
