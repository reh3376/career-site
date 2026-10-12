from google.protobuf import descriptor_pb2 as _descriptor_pb2
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from typing import ClassVar as _ClassVar

DESCRIPTOR: _descriptor.FileDescriptor

class AuthLevel(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    AUTH_LEVEL_UNSPECIFIED: _ClassVar[AuthLevel]
    AUTH_LEVEL_PUBLIC: _ClassVar[AuthLevel]
    AUTH_LEVEL_MEMBER: _ClassVar[AuthLevel]
    AUTH_LEVEL_ADMIN: _ClassVar[AuthLevel]

class AuditLevel(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    AUDIT_LEVEL_UNSPECIFIED: _ClassVar[AuditLevel]
    AUDIT_LEVEL_NONE: _ClassVar[AuditLevel]
    AUDIT_LEVEL_RECORDED: _ClassVar[AuditLevel]
AUTH_LEVEL_UNSPECIFIED: AuthLevel
AUTH_LEVEL_PUBLIC: AuthLevel
AUTH_LEVEL_MEMBER: AuthLevel
AUTH_LEVEL_ADMIN: AuthLevel
AUDIT_LEVEL_UNSPECIFIED: AuditLevel
AUDIT_LEVEL_NONE: AuditLevel
AUDIT_LEVEL_RECORDED: AuditLevel
AUTH_FIELD_NUMBER: _ClassVar[int]
auth: _descriptor.FieldDescriptor
ALLOW_UNVERIFIED_FIELD_NUMBER: _ClassVar[int]
allow_unverified: _descriptor.FieldDescriptor
RATE_LIMIT_PER_MINUTE_FIELD_NUMBER: _ClassVar[int]
rate_limit_per_minute: _descriptor.FieldDescriptor
MFA_FRESH_FIELD_NUMBER: _ClassVar[int]
mfa_fresh: _descriptor.FieldDescriptor
AUDIT_FIELD_NUMBER: _ClassVar[int]
audit: _descriptor.FieldDescriptor
