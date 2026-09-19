from buf.validate import validate_pb2 as _validate_pb2
from career.v1 import options_pb2 as _options_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class SupportCategory(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SUPPORT_CATEGORY_UNSPECIFIED: _ClassVar[SupportCategory]
    SUPPORT_CATEGORY_GENERAL_QUESTION: _ClassVar[SupportCategory]
    SUPPORT_CATEGORY_BUG_REPORT: _ClassVar[SupportCategory]
    SUPPORT_CATEGORY_FEATURE_REQUEST: _ClassVar[SupportCategory]
    SUPPORT_CATEGORY_CONTRIBUTOR_ACCESS: _ClassVar[SupportCategory]
    SUPPORT_CATEGORY_PRESS_INQUIRY: _ClassVar[SupportCategory]
    SUPPORT_CATEGORY_OTHER: _ClassVar[SupportCategory]
SUPPORT_CATEGORY_UNSPECIFIED: SupportCategory
SUPPORT_CATEGORY_GENERAL_QUESTION: SupportCategory
SUPPORT_CATEGORY_BUG_REPORT: SupportCategory
SUPPORT_CATEGORY_FEATURE_REQUEST: SupportCategory
SUPPORT_CATEGORY_CONTRIBUTOR_ACCESS: SupportCategory
SUPPORT_CATEGORY_PRESS_INQUIRY: SupportCategory
SUPPORT_CATEGORY_OTHER: SupportCategory

class GetContactOptionsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ContactChannel(_message.Message):
    __slots__ = ("kind", "label", "url")
    KIND_FIELD_NUMBER: _ClassVar[int]
    LABEL_FIELD_NUMBER: _ClassVar[int]
    URL_FIELD_NUMBER: _ClassVar[int]
    kind: str
    label: str
    url: str
    def __init__(self, kind: _Optional[str] = ..., label: _Optional[str] = ..., url: _Optional[str] = ...) -> None: ...

class GetContactOptionsResponse(_message.Message):
    __slots__ = ("availability", "location_preferences", "channels")
    AVAILABILITY_FIELD_NUMBER: _ClassVar[int]
    LOCATION_PREFERENCES_FIELD_NUMBER: _ClassVar[int]
    CHANNELS_FIELD_NUMBER: _ClassVar[int]
    availability: str
    location_preferences: str
    channels: _containers.RepeatedCompositeFieldContainer[ContactChannel]
    def __init__(self, availability: _Optional[str] = ..., location_preferences: _Optional[str] = ..., channels: _Optional[_Iterable[_Union[ContactChannel, _Mapping]]] = ...) -> None: ...

class SubmitContactRequest(_message.Message):
    __slots__ = ("subject", "message", "reply_channel", "conversation_id", "category", "name", "email", "turnstile_token")
    class ReplyChannel(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        REPLY_CHANNEL_UNSPECIFIED: _ClassVar[SubmitContactRequest.ReplyChannel]
        REPLY_CHANNEL_EMAIL: _ClassVar[SubmitContactRequest.ReplyChannel]
        REPLY_CHANNEL_LINKEDIN: _ClassVar[SubmitContactRequest.ReplyChannel]
    REPLY_CHANNEL_UNSPECIFIED: SubmitContactRequest.ReplyChannel
    REPLY_CHANNEL_EMAIL: SubmitContactRequest.ReplyChannel
    REPLY_CHANNEL_LINKEDIN: SubmitContactRequest.ReplyChannel
    SUBJECT_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    REPLY_CHANNEL_FIELD_NUMBER: _ClassVar[int]
    CONVERSATION_ID_FIELD_NUMBER: _ClassVar[int]
    CATEGORY_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    EMAIL_FIELD_NUMBER: _ClassVar[int]
    TURNSTILE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    subject: str
    message: str
    reply_channel: SubmitContactRequest.ReplyChannel
    conversation_id: str
    category: SupportCategory
    name: str
    email: str
    turnstile_token: str
    def __init__(self, subject: _Optional[str] = ..., message: _Optional[str] = ..., reply_channel: _Optional[_Union[SubmitContactRequest.ReplyChannel, str]] = ..., conversation_id: _Optional[str] = ..., category: _Optional[_Union[SupportCategory, str]] = ..., name: _Optional[str] = ..., email: _Optional[str] = ..., turnstile_token: _Optional[str] = ...) -> None: ...

class SubmitContactResponse(_message.Message):
    __slots__ = ("ticket_id",)
    TICKET_ID_FIELD_NUMBER: _ClassVar[int]
    ticket_id: str
    def __init__(self, ticket_id: _Optional[str] = ...) -> None: ...
