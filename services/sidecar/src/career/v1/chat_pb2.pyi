import datetime

from buf.validate import validate_pb2 as _validate_pb2
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

class Rating(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    RATING_UNSPECIFIED: _ClassVar[Rating]
    RATING_UP: _ClassVar[Rating]
    RATING_DOWN: _ClassVar[Rating]
RATING_UNSPECIFIED: Rating
RATING_UP: Rating
RATING_DOWN: Rating

class Conversation(_message.Message):
    __slots__ = ("id", "title", "started_at", "last_message_at", "message_count", "persona_version", "context_content_id")
    ID_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    STARTED_AT_FIELD_NUMBER: _ClassVar[int]
    LAST_MESSAGE_AT_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_COUNT_FIELD_NUMBER: _ClassVar[int]
    PERSONA_VERSION_FIELD_NUMBER: _ClassVar[int]
    CONTEXT_CONTENT_ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    title: str
    started_at: _timestamp_pb2.Timestamp
    last_message_at: _timestamp_pb2.Timestamp
    message_count: int
    persona_version: str
    context_content_id: str
    def __init__(self, id: _Optional[str] = ..., title: _Optional[str] = ..., started_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., last_message_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., message_count: _Optional[int] = ..., persona_version: _Optional[str] = ..., context_content_id: _Optional[str] = ...) -> None: ...

class Citation(_message.Message):
    __slots__ = ("chunk_id", "content_id", "title", "path", "heading", "rank")
    CHUNK_ID_FIELD_NUMBER: _ClassVar[int]
    CONTENT_ID_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    HEADING_FIELD_NUMBER: _ClassVar[int]
    RANK_FIELD_NUMBER: _ClassVar[int]
    chunk_id: str
    content_id: str
    title: str
    path: str
    heading: str
    rank: int
    def __init__(self, chunk_id: _Optional[str] = ..., content_id: _Optional[str] = ..., title: _Optional[str] = ..., path: _Optional[str] = ..., heading: _Optional[str] = ..., rank: _Optional[int] = ...) -> None: ...

class MessageFlags(_message.Message):
    __slots__ = ("out_of_scope", "no_support", "degraded", "qa_match")
    OUT_OF_SCOPE_FIELD_NUMBER: _ClassVar[int]
    NO_SUPPORT_FIELD_NUMBER: _ClassVar[int]
    DEGRADED_FIELD_NUMBER: _ClassVar[int]
    QA_MATCH_FIELD_NUMBER: _ClassVar[int]
    out_of_scope: bool
    no_support: bool
    degraded: bool
    qa_match: bool
    def __init__(self, out_of_scope: _Optional[bool] = ..., no_support: _Optional[bool] = ..., degraded: _Optional[bool] = ..., qa_match: _Optional[bool] = ...) -> None: ...

class Message(_message.Message):
    __slots__ = ("id", "conversation_id", "role", "text", "created_at", "citations", "rating", "flags", "persona_version")
    class Role(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        ROLE_UNSPECIFIED: _ClassVar[Message.Role]
        ROLE_USER: _ClassVar[Message.Role]
        ROLE_ASSISTANT: _ClassVar[Message.Role]
        ROLE_OWNER: _ClassVar[Message.Role]
    ROLE_UNSPECIFIED: Message.Role
    ROLE_USER: Message.Role
    ROLE_ASSISTANT: Message.Role
    ROLE_OWNER: Message.Role
    ID_FIELD_NUMBER: _ClassVar[int]
    CONVERSATION_ID_FIELD_NUMBER: _ClassVar[int]
    ROLE_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    CITATIONS_FIELD_NUMBER: _ClassVar[int]
    RATING_FIELD_NUMBER: _ClassVar[int]
    FLAGS_FIELD_NUMBER: _ClassVar[int]
    PERSONA_VERSION_FIELD_NUMBER: _ClassVar[int]
    id: str
    conversation_id: str
    role: Message.Role
    text: str
    created_at: _timestamp_pb2.Timestamp
    citations: _containers.RepeatedCompositeFieldContainer[Citation]
    rating: Rating
    flags: MessageFlags
    persona_version: str
    def __init__(self, id: _Optional[str] = ..., conversation_id: _Optional[str] = ..., role: _Optional[_Union[Message.Role, str]] = ..., text: _Optional[str] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., citations: _Optional[_Iterable[_Union[Citation, _Mapping]]] = ..., rating: _Optional[_Union[Rating, str]] = ..., flags: _Optional[_Union[MessageFlags, _Mapping]] = ..., persona_version: _Optional[str] = ...) -> None: ...

class CreateConversationRequest(_message.Message):
    __slots__ = ("context_content_id",)
    CONTEXT_CONTENT_ID_FIELD_NUMBER: _ClassVar[int]
    context_content_id: str
    def __init__(self, context_content_id: _Optional[str] = ...) -> None: ...

class CreateConversationResponse(_message.Message):
    __slots__ = ("conversation", "disclosure")
    CONVERSATION_FIELD_NUMBER: _ClassVar[int]
    DISCLOSURE_FIELD_NUMBER: _ClassVar[int]
    conversation: Conversation
    disclosure: Message
    def __init__(self, conversation: _Optional[_Union[Conversation, _Mapping]] = ..., disclosure: _Optional[_Union[Message, _Mapping]] = ...) -> None: ...

class ListConversationsRequest(_message.Message):
    __slots__ = ("page",)
    PAGE_FIELD_NUMBER: _ClassVar[int]
    page: _common_pb2.PageRequest
    def __init__(self, page: _Optional[_Union[_common_pb2.PageRequest, _Mapping]] = ...) -> None: ...

class ListConversationsResponse(_message.Message):
    __slots__ = ("conversations", "page")
    CONVERSATIONS_FIELD_NUMBER: _ClassVar[int]
    PAGE_FIELD_NUMBER: _ClassVar[int]
    conversations: _containers.RepeatedCompositeFieldContainer[Conversation]
    page: _common_pb2.PageResponse
    def __init__(self, conversations: _Optional[_Iterable[_Union[Conversation, _Mapping]]] = ..., page: _Optional[_Union[_common_pb2.PageResponse, _Mapping]] = ...) -> None: ...

class GetConversationRequest(_message.Message):
    __slots__ = ("conversation_id",)
    CONVERSATION_ID_FIELD_NUMBER: _ClassVar[int]
    conversation_id: str
    def __init__(self, conversation_id: _Optional[str] = ...) -> None: ...

class GetConversationResponse(_message.Message):
    __slots__ = ("conversation", "messages")
    CONVERSATION_FIELD_NUMBER: _ClassVar[int]
    MESSAGES_FIELD_NUMBER: _ClassVar[int]
    conversation: Conversation
    messages: _containers.RepeatedCompositeFieldContainer[Message]
    def __init__(self, conversation: _Optional[_Union[Conversation, _Mapping]] = ..., messages: _Optional[_Iterable[_Union[Message, _Mapping]]] = ...) -> None: ...

class SendMessageRequest(_message.Message):
    __slots__ = ("conversation_id", "text", "context_content_id")
    CONVERSATION_ID_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    CONTEXT_CONTENT_ID_FIELD_NUMBER: _ClassVar[int]
    conversation_id: str
    text: str
    context_content_id: str
    def __init__(self, conversation_id: _Optional[str] = ..., text: _Optional[str] = ..., context_content_id: _Optional[str] = ...) -> None: ...

class Usage(_message.Message):
    __slots__ = ("input_tokens", "output_tokens", "remaining_today", "remaining_in_conversation")
    INPUT_TOKENS_FIELD_NUMBER: _ClassVar[int]
    OUTPUT_TOKENS_FIELD_NUMBER: _ClassVar[int]
    REMAINING_TODAY_FIELD_NUMBER: _ClassVar[int]
    REMAINING_IN_CONVERSATION_FIELD_NUMBER: _ClassVar[int]
    input_tokens: int
    output_tokens: int
    remaining_today: int
    remaining_in_conversation: int
    def __init__(self, input_tokens: _Optional[int] = ..., output_tokens: _Optional[int] = ..., remaining_today: _Optional[int] = ..., remaining_in_conversation: _Optional[int] = ...) -> None: ...

class SendMessageResponse(_message.Message):
    __slots__ = ("start", "delta", "citations", "usage", "done")
    class Start(_message.Message):
        __slots__ = ("user_message_id", "assistant_message_id", "persona_version")
        USER_MESSAGE_ID_FIELD_NUMBER: _ClassVar[int]
        ASSISTANT_MESSAGE_ID_FIELD_NUMBER: _ClassVar[int]
        PERSONA_VERSION_FIELD_NUMBER: _ClassVar[int]
        user_message_id: str
        assistant_message_id: str
        persona_version: str
        def __init__(self, user_message_id: _Optional[str] = ..., assistant_message_id: _Optional[str] = ..., persona_version: _Optional[str] = ...) -> None: ...
    class Delta(_message.Message):
        __slots__ = ("text",)
        TEXT_FIELD_NUMBER: _ClassVar[int]
        text: str
        def __init__(self, text: _Optional[str] = ...) -> None: ...
    class Citations(_message.Message):
        __slots__ = ("citations",)
        CITATIONS_FIELD_NUMBER: _ClassVar[int]
        citations: _containers.RepeatedCompositeFieldContainer[Citation]
        def __init__(self, citations: _Optional[_Iterable[_Union[Citation, _Mapping]]] = ...) -> None: ...
    class Done(_message.Message):
        __slots__ = ("message",)
        MESSAGE_FIELD_NUMBER: _ClassVar[int]
        message: Message
        def __init__(self, message: _Optional[_Union[Message, _Mapping]] = ...) -> None: ...
    START_FIELD_NUMBER: _ClassVar[int]
    DELTA_FIELD_NUMBER: _ClassVar[int]
    CITATIONS_FIELD_NUMBER: _ClassVar[int]
    USAGE_FIELD_NUMBER: _ClassVar[int]
    DONE_FIELD_NUMBER: _ClassVar[int]
    start: SendMessageResponse.Start
    delta: SendMessageResponse.Delta
    citations: SendMessageResponse.Citations
    usage: Usage
    done: SendMessageResponse.Done
    def __init__(self, start: _Optional[_Union[SendMessageResponse.Start, _Mapping]] = ..., delta: _Optional[_Union[SendMessageResponse.Delta, _Mapping]] = ..., citations: _Optional[_Union[SendMessageResponse.Citations, _Mapping]] = ..., usage: _Optional[_Union[Usage, _Mapping]] = ..., done: _Optional[_Union[SendMessageResponse.Done, _Mapping]] = ...) -> None: ...

class DeleteConversationRequest(_message.Message):
    __slots__ = ("conversation_id",)
    CONVERSATION_ID_FIELD_NUMBER: _ClassVar[int]
    conversation_id: str
    def __init__(self, conversation_id: _Optional[str] = ...) -> None: ...

class DeleteConversationResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class RateMessageRequest(_message.Message):
    __slots__ = ("message_id", "rating", "comment")
    MESSAGE_ID_FIELD_NUMBER: _ClassVar[int]
    RATING_FIELD_NUMBER: _ClassVar[int]
    COMMENT_FIELD_NUMBER: _ClassVar[int]
    message_id: str
    rating: Rating
    comment: str
    def __init__(self, message_id: _Optional[str] = ..., rating: _Optional[_Union[Rating, str]] = ..., comment: _Optional[str] = ...) -> None: ...

class RateMessageResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class EscalateRequest(_message.Message):
    __slots__ = ("conversation_id", "question", "message_id")
    CONVERSATION_ID_FIELD_NUMBER: _ClassVar[int]
    QUESTION_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_ID_FIELD_NUMBER: _ClassVar[int]
    conversation_id: str
    question: str
    message_id: str
    def __init__(self, conversation_id: _Optional[str] = ..., question: _Optional[str] = ..., message_id: _Optional[str] = ...) -> None: ...

class Escalation(_message.Message):
    __slots__ = ("id", "conversation_id", "status", "created_at", "replied_at")
    class Status(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        STATUS_UNSPECIFIED: _ClassVar[Escalation.Status]
        STATUS_OPEN: _ClassVar[Escalation.Status]
        STATUS_ANSWERED: _ClassVar[Escalation.Status]
        STATUS_CLOSED: _ClassVar[Escalation.Status]
    STATUS_UNSPECIFIED: Escalation.Status
    STATUS_OPEN: Escalation.Status
    STATUS_ANSWERED: Escalation.Status
    STATUS_CLOSED: Escalation.Status
    ID_FIELD_NUMBER: _ClassVar[int]
    CONVERSATION_ID_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    REPLIED_AT_FIELD_NUMBER: _ClassVar[int]
    id: str
    conversation_id: str
    status: Escalation.Status
    created_at: _timestamp_pb2.Timestamp
    replied_at: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[str] = ..., conversation_id: _Optional[str] = ..., status: _Optional[_Union[Escalation.Status, str]] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., replied_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class EscalateResponse(_message.Message):
    __slots__ = ("escalation",)
    ESCALATION_FIELD_NUMBER: _ClassVar[int]
    escalation: Escalation
    def __init__(self, escalation: _Optional[_Union[Escalation, _Mapping]] = ...) -> None: ...

class GetSuggestionsRequest(_message.Message):
    __slots__ = ("path", "content_id")
    PATH_FIELD_NUMBER: _ClassVar[int]
    CONTENT_ID_FIELD_NUMBER: _ClassVar[int]
    path: str
    content_id: str
    def __init__(self, path: _Optional[str] = ..., content_id: _Optional[str] = ...) -> None: ...

class GetSuggestionsResponse(_message.Message):
    __slots__ = ("questions",)
    QUESTIONS_FIELD_NUMBER: _ClassVar[int]
    questions: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, questions: _Optional[_Iterable[str]] = ...) -> None: ...

class GetQuotaRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetQuotaResponse(_message.Message):
    __slots__ = ("daily_limit", "used_today", "conversation_limit", "budget_mode")
    class BudgetMode(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        BUDGET_MODE_UNSPECIFIED: _ClassVar[GetQuotaResponse.BudgetMode]
        BUDGET_MODE_NORMAL: _ClassVar[GetQuotaResponse.BudgetMode]
        BUDGET_MODE_QA_ONLY: _ClassVar[GetQuotaResponse.BudgetMode]
        BUDGET_MODE_DEGRADED: _ClassVar[GetQuotaResponse.BudgetMode]
    BUDGET_MODE_UNSPECIFIED: GetQuotaResponse.BudgetMode
    BUDGET_MODE_NORMAL: GetQuotaResponse.BudgetMode
    BUDGET_MODE_QA_ONLY: GetQuotaResponse.BudgetMode
    BUDGET_MODE_DEGRADED: GetQuotaResponse.BudgetMode
    DAILY_LIMIT_FIELD_NUMBER: _ClassVar[int]
    USED_TODAY_FIELD_NUMBER: _ClassVar[int]
    CONVERSATION_LIMIT_FIELD_NUMBER: _ClassVar[int]
    BUDGET_MODE_FIELD_NUMBER: _ClassVar[int]
    daily_limit: int
    used_today: int
    conversation_limit: int
    budget_mode: GetQuotaResponse.BudgetMode
    def __init__(self, daily_limit: _Optional[int] = ..., used_today: _Optional[int] = ..., conversation_limit: _Optional[int] = ..., budget_mode: _Optional[_Union[GetQuotaResponse.BudgetMode, str]] = ...) -> None: ...
