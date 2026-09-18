from buf.validate import validate_pb2 as _validate_pb2
from career.v1 import common_pb2 as _common_pb2
from career.v1 import options_pb2 as _options_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class RegisterRequest(_message.Message):
    __slots__ = ("name", "email", "password", "organization", "stated_role", "consent_version", "consent_accepted", "turnstile_token")
    NAME_FIELD_NUMBER: _ClassVar[int]
    EMAIL_FIELD_NUMBER: _ClassVar[int]
    PASSWORD_FIELD_NUMBER: _ClassVar[int]
    ORGANIZATION_FIELD_NUMBER: _ClassVar[int]
    STATED_ROLE_FIELD_NUMBER: _ClassVar[int]
    CONSENT_VERSION_FIELD_NUMBER: _ClassVar[int]
    CONSENT_ACCEPTED_FIELD_NUMBER: _ClassVar[int]
    TURNSTILE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    name: str
    email: str
    password: str
    organization: str
    stated_role: str
    consent_version: str
    consent_accepted: bool
    turnstile_token: str
    def __init__(self, name: _Optional[str] = ..., email: _Optional[str] = ..., password: _Optional[str] = ..., organization: _Optional[str] = ..., stated_role: _Optional[str] = ..., consent_version: _Optional[str] = ..., consent_accepted: _Optional[bool] = ..., turnstile_token: _Optional[str] = ...) -> None: ...

class RegisterResponse(_message.Message):
    __slots__ = ("message",)
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    message: str
    def __init__(self, message: _Optional[str] = ...) -> None: ...

class VerifyRequest(_message.Message):
    __slots__ = ("token", "code", "email")
    TOKEN_FIELD_NUMBER: _ClassVar[int]
    CODE_FIELD_NUMBER: _ClassVar[int]
    EMAIL_FIELD_NUMBER: _ClassVar[int]
    token: str
    code: str
    email: str
    def __init__(self, token: _Optional[str] = ..., code: _Optional[str] = ..., email: _Optional[str] = ...) -> None: ...

class VerifyResponse(_message.Message):
    __slots__ = ("me", "questionnaire_pending")
    ME_FIELD_NUMBER: _ClassVar[int]
    QUESTIONNAIRE_PENDING_FIELD_NUMBER: _ClassVar[int]
    me: _common_pb2.Me
    questionnaire_pending: bool
    def __init__(self, me: _Optional[_Union[_common_pb2.Me, _Mapping]] = ..., questionnaire_pending: _Optional[bool] = ...) -> None: ...

class ResendVerificationRequest(_message.Message):
    __slots__ = ("email",)
    EMAIL_FIELD_NUMBER: _ClassVar[int]
    email: str
    def __init__(self, email: _Optional[str] = ...) -> None: ...

class ResendVerificationResponse(_message.Message):
    __slots__ = ("message",)
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    message: str
    def __init__(self, message: _Optional[str] = ...) -> None: ...

class LoginRequest(_message.Message):
    __slots__ = ("email", "password", "turnstile_token")
    EMAIL_FIELD_NUMBER: _ClassVar[int]
    PASSWORD_FIELD_NUMBER: _ClassVar[int]
    TURNSTILE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    email: str
    password: str
    turnstile_token: str
    def __init__(self, email: _Optional[str] = ..., password: _Optional[str] = ..., turnstile_token: _Optional[str] = ...) -> None: ...

class LoginResponse(_message.Message):
    __slots__ = ("me", "mfa_required", "questionnaire_pending")
    ME_FIELD_NUMBER: _ClassVar[int]
    MFA_REQUIRED_FIELD_NUMBER: _ClassVar[int]
    QUESTIONNAIRE_PENDING_FIELD_NUMBER: _ClassVar[int]
    me: _common_pb2.Me
    mfa_required: bool
    questionnaire_pending: bool
    def __init__(self, me: _Optional[_Union[_common_pb2.Me, _Mapping]] = ..., mfa_required: _Optional[bool] = ..., questionnaire_pending: _Optional[bool] = ...) -> None: ...

class LogoutRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class LogoutResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class LogoutAllRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class LogoutAllResponse(_message.Message):
    __slots__ = ("revoked",)
    REVOKED_FIELD_NUMBER: _ClassVar[int]
    revoked: int
    def __init__(self, revoked: _Optional[int] = ...) -> None: ...

class ForgotPasswordRequest(_message.Message):
    __slots__ = ("email",)
    EMAIL_FIELD_NUMBER: _ClassVar[int]
    email: str
    def __init__(self, email: _Optional[str] = ...) -> None: ...

class ForgotPasswordResponse(_message.Message):
    __slots__ = ("message",)
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    message: str
    def __init__(self, message: _Optional[str] = ...) -> None: ...

class ResetPasswordRequest(_message.Message):
    __slots__ = ("token", "new_password")
    TOKEN_FIELD_NUMBER: _ClassVar[int]
    NEW_PASSWORD_FIELD_NUMBER: _ClassVar[int]
    token: str
    new_password: str
    def __init__(self, token: _Optional[str] = ..., new_password: _Optional[str] = ...) -> None: ...

class ResetPasswordResponse(_message.Message):
    __slots__ = ("me",)
    ME_FIELD_NUMBER: _ClassVar[int]
    me: _common_pb2.Me
    def __init__(self, me: _Optional[_Union[_common_pb2.Me, _Mapping]] = ...) -> None: ...

class ChangePasswordRequest(_message.Message):
    __slots__ = ("current_password", "new_password")
    CURRENT_PASSWORD_FIELD_NUMBER: _ClassVar[int]
    NEW_PASSWORD_FIELD_NUMBER: _ClassVar[int]
    current_password: str
    new_password: str
    def __init__(self, current_password: _Optional[str] = ..., new_password: _Optional[str] = ...) -> None: ...

class ChangePasswordResponse(_message.Message):
    __slots__ = ("revoked",)
    REVOKED_FIELD_NUMBER: _ClassVar[int]
    revoked: int
    def __init__(self, revoked: _Optional[int] = ...) -> None: ...

class ChangeEmailRequest(_message.Message):
    __slots__ = ("new_email", "current_password")
    NEW_EMAIL_FIELD_NUMBER: _ClassVar[int]
    CURRENT_PASSWORD_FIELD_NUMBER: _ClassVar[int]
    new_email: str
    current_password: str
    def __init__(self, new_email: _Optional[str] = ..., current_password: _Optional[str] = ...) -> None: ...

class ChangeEmailResponse(_message.Message):
    __slots__ = ("message",)
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    message: str
    def __init__(self, message: _Optional[str] = ...) -> None: ...

class MfaEnrollRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class MfaEnrollResponse(_message.Message):
    __slots__ = ("otpauth_uri", "secret_base32", "recovery_codes")
    OTPAUTH_URI_FIELD_NUMBER: _ClassVar[int]
    SECRET_BASE32_FIELD_NUMBER: _ClassVar[int]
    RECOVERY_CODES_FIELD_NUMBER: _ClassVar[int]
    otpauth_uri: str
    secret_base32: str
    recovery_codes: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, otpauth_uri: _Optional[str] = ..., secret_base32: _Optional[str] = ..., recovery_codes: _Optional[_Iterable[str]] = ...) -> None: ...

class MfaVerifyRequest(_message.Message):
    __slots__ = ("code",)
    CODE_FIELD_NUMBER: _ClassVar[int]
    code: str
    def __init__(self, code: _Optional[str] = ...) -> None: ...

class MfaVerifyResponse(_message.Message):
    __slots__ = ("me", "recovery_codes_remaining")
    ME_FIELD_NUMBER: _ClassVar[int]
    RECOVERY_CODES_REMAINING_FIELD_NUMBER: _ClassVar[int]
    me: _common_pb2.Me
    recovery_codes_remaining: int
    def __init__(self, me: _Optional[_Union[_common_pb2.Me, _Mapping]] = ..., recovery_codes_remaining: _Optional[int] = ...) -> None: ...
