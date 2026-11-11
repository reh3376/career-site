import datetime

from buf.validate import validate_pb2 as _validate_pb2
from career.v1 import options_pb2 as _options_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class RequirementStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    REQUIREMENT_STATUS_UNSPECIFIED: _ClassVar[RequirementStatus]
    REQUIREMENT_STATUS_DRAFT: _ClassVar[RequirementStatus]
    REQUIREMENT_STATUS_ACTIVE: _ClassVar[RequirementStatus]
    REQUIREMENT_STATUS_RETIRED: _ClassVar[RequirementStatus]
REQUIREMENT_STATUS_UNSPECIFIED: RequirementStatus
REQUIREMENT_STATUS_DRAFT: RequirementStatus
REQUIREMENT_STATUS_ACTIVE: RequirementStatus
REQUIREMENT_STATUS_RETIRED: RequirementStatus

class ScopeThreshold(_message.Message):
    __slots__ = ("unit", "value")
    UNIT_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    unit: str
    value: float
    def __init__(self, unit: _Optional[str] = ..., value: _Optional[float] = ...) -> None: ...

class Thresholds(_message.Message):
    __slots__ = ("min_years", "recency_years", "min_scope", "credential_level", "credential_field", "relationship_kind")
    MIN_YEARS_FIELD_NUMBER: _ClassVar[int]
    RECENCY_YEARS_FIELD_NUMBER: _ClassVar[int]
    MIN_SCOPE_FIELD_NUMBER: _ClassVar[int]
    CREDENTIAL_LEVEL_FIELD_NUMBER: _ClassVar[int]
    CREDENTIAL_FIELD_FIELD_NUMBER: _ClassVar[int]
    RELATIONSHIP_KIND_FIELD_NUMBER: _ClassVar[int]
    min_years: float
    recency_years: float
    min_scope: ScopeThreshold
    credential_level: str
    credential_field: str
    relationship_kind: str
    def __init__(self, min_years: _Optional[float] = ..., recency_years: _Optional[float] = ..., min_scope: _Optional[_Union[ScopeThreshold, _Mapping]] = ..., credential_level: _Optional[str] = ..., credential_field: _Optional[str] = ..., relationship_kind: _Optional[str] = ...) -> None: ...

class Requirement(_message.Message):
    __slots__ = ("id", "role_id", "local_id", "version", "supersedes", "schema_version", "vocab_version", "text", "source_quote", "type", "category", "requirement_class", "weight", "evidence_standard", "evidence_standard_template", "alternatives", "equivalence_notes", "equivalency", "thresholds", "named_parties", "included", "inclusion_reason", "edited_by_display", "edited_at", "edit_reason", "status", "released_at", "released_by_person", "created_at", "type_source")
    ID_FIELD_NUMBER: _ClassVar[int]
    ROLE_ID_FIELD_NUMBER: _ClassVar[int]
    LOCAL_ID_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    SUPERSEDES_FIELD_NUMBER: _ClassVar[int]
    SCHEMA_VERSION_FIELD_NUMBER: _ClassVar[int]
    VOCAB_VERSION_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    SOURCE_QUOTE_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    CATEGORY_FIELD_NUMBER: _ClassVar[int]
    REQUIREMENT_CLASS_FIELD_NUMBER: _ClassVar[int]
    WEIGHT_FIELD_NUMBER: _ClassVar[int]
    EVIDENCE_STANDARD_FIELD_NUMBER: _ClassVar[int]
    EVIDENCE_STANDARD_TEMPLATE_FIELD_NUMBER: _ClassVar[int]
    ALTERNATIVES_FIELD_NUMBER: _ClassVar[int]
    EQUIVALENCE_NOTES_FIELD_NUMBER: _ClassVar[int]
    EQUIVALENCY_FIELD_NUMBER: _ClassVar[int]
    THRESHOLDS_FIELD_NUMBER: _ClassVar[int]
    NAMED_PARTIES_FIELD_NUMBER: _ClassVar[int]
    INCLUDED_FIELD_NUMBER: _ClassVar[int]
    INCLUSION_REASON_FIELD_NUMBER: _ClassVar[int]
    EDITED_BY_DISPLAY_FIELD_NUMBER: _ClassVar[int]
    EDITED_AT_FIELD_NUMBER: _ClassVar[int]
    EDIT_REASON_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    RELEASED_AT_FIELD_NUMBER: _ClassVar[int]
    RELEASED_BY_PERSON_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    TYPE_SOURCE_FIELD_NUMBER: _ClassVar[int]
    id: str
    role_id: str
    local_id: str
    version: int
    supersedes: str
    schema_version: str
    vocab_version: str
    text: str
    source_quote: str
    type: str
    category: str
    requirement_class: str
    weight: int
    evidence_standard: str
    evidence_standard_template: str
    alternatives: _containers.RepeatedScalarFieldContainer[str]
    equivalence_notes: str
    equivalency: str
    thresholds: Thresholds
    named_parties: _containers.RepeatedScalarFieldContainer[str]
    included: bool
    inclusion_reason: str
    edited_by_display: str
    edited_at: _timestamp_pb2.Timestamp
    edit_reason: str
    status: RequirementStatus
    released_at: _timestamp_pb2.Timestamp
    released_by_person: bool
    created_at: _timestamp_pb2.Timestamp
    type_source: str
    def __init__(self, id: _Optional[str] = ..., role_id: _Optional[str] = ..., local_id: _Optional[str] = ..., version: _Optional[int] = ..., supersedes: _Optional[str] = ..., schema_version: _Optional[str] = ..., vocab_version: _Optional[str] = ..., text: _Optional[str] = ..., source_quote: _Optional[str] = ..., type: _Optional[str] = ..., category: _Optional[str] = ..., requirement_class: _Optional[str] = ..., weight: _Optional[int] = ..., evidence_standard: _Optional[str] = ..., evidence_standard_template: _Optional[str] = ..., alternatives: _Optional[_Iterable[str]] = ..., equivalence_notes: _Optional[str] = ..., equivalency: _Optional[str] = ..., thresholds: _Optional[_Union[Thresholds, _Mapping]] = ..., named_parties: _Optional[_Iterable[str]] = ..., included: _Optional[bool] = ..., inclusion_reason: _Optional[str] = ..., edited_by_display: _Optional[str] = ..., edited_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., edit_reason: _Optional[str] = ..., status: _Optional[_Union[RequirementStatus, str]] = ..., released_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., released_by_person: _Optional[bool] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., type_source: _Optional[str] = ...) -> None: ...

class StringList(_message.Message):
    __slots__ = ("values",)
    VALUES_FIELD_NUMBER: _ClassVar[int]
    values: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, values: _Optional[_Iterable[str]] = ...) -> None: ...

class RequirementPatch(_message.Message):
    __slots__ = ("text", "type", "requirement_class", "weight", "thresholds", "evidence_standard", "equivalence_notes", "equivalency", "alternatives", "named_parties")
    TEXT_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    REQUIREMENT_CLASS_FIELD_NUMBER: _ClassVar[int]
    WEIGHT_FIELD_NUMBER: _ClassVar[int]
    THRESHOLDS_FIELD_NUMBER: _ClassVar[int]
    EVIDENCE_STANDARD_FIELD_NUMBER: _ClassVar[int]
    EQUIVALENCE_NOTES_FIELD_NUMBER: _ClassVar[int]
    EQUIVALENCY_FIELD_NUMBER: _ClassVar[int]
    ALTERNATIVES_FIELD_NUMBER: _ClassVar[int]
    NAMED_PARTIES_FIELD_NUMBER: _ClassVar[int]
    text: str
    type: str
    requirement_class: str
    weight: int
    thresholds: Thresholds
    evidence_standard: str
    equivalence_notes: str
    equivalency: str
    alternatives: StringList
    named_parties: StringList
    def __init__(self, text: _Optional[str] = ..., type: _Optional[str] = ..., requirement_class: _Optional[str] = ..., weight: _Optional[int] = ..., thresholds: _Optional[_Union[Thresholds, _Mapping]] = ..., evidence_standard: _Optional[str] = ..., equivalence_notes: _Optional[str] = ..., equivalency: _Optional[str] = ..., alternatives: _Optional[_Union[StringList, _Mapping]] = ..., named_parties: _Optional[_Union[StringList, _Mapping]] = ...) -> None: ...

class ListRequirementsRequest(_message.Message):
    __slots__ = ("role_id", "submission_id", "include_retired")
    ROLE_ID_FIELD_NUMBER: _ClassVar[int]
    SUBMISSION_ID_FIELD_NUMBER: _ClassVar[int]
    INCLUDE_RETIRED_FIELD_NUMBER: _ClassVar[int]
    role_id: str
    submission_id: str
    include_retired: bool
    def __init__(self, role_id: _Optional[str] = ..., submission_id: _Optional[str] = ..., include_retired: _Optional[bool] = ...) -> None: ...

class ListRequirementsResponse(_message.Message):
    __slots__ = ("role_id", "requirements", "pending_drafts", "human_edited", "types", "classes", "scope_units", "credential_levels", "relationship_kinds")
    ROLE_ID_FIELD_NUMBER: _ClassVar[int]
    REQUIREMENTS_FIELD_NUMBER: _ClassVar[int]
    PENDING_DRAFTS_FIELD_NUMBER: _ClassVar[int]
    HUMAN_EDITED_FIELD_NUMBER: _ClassVar[int]
    TYPES_FIELD_NUMBER: _ClassVar[int]
    CLASSES_FIELD_NUMBER: _ClassVar[int]
    SCOPE_UNITS_FIELD_NUMBER: _ClassVar[int]
    CREDENTIAL_LEVELS_FIELD_NUMBER: _ClassVar[int]
    RELATIONSHIP_KINDS_FIELD_NUMBER: _ClassVar[int]
    role_id: str
    requirements: _containers.RepeatedCompositeFieldContainer[Requirement]
    pending_drafts: int
    human_edited: bool
    types: _containers.RepeatedCompositeFieldContainer[VocabCode]
    classes: _containers.RepeatedCompositeFieldContainer[VocabCode]
    scope_units: _containers.RepeatedCompositeFieldContainer[VocabCode]
    credential_levels: _containers.RepeatedCompositeFieldContainer[VocabCode]
    relationship_kinds: _containers.RepeatedCompositeFieldContainer[VocabCode]
    def __init__(self, role_id: _Optional[str] = ..., requirements: _Optional[_Iterable[_Union[Requirement, _Mapping]]] = ..., pending_drafts: _Optional[int] = ..., human_edited: _Optional[bool] = ..., types: _Optional[_Iterable[_Union[VocabCode, _Mapping]]] = ..., classes: _Optional[_Iterable[_Union[VocabCode, _Mapping]]] = ..., scope_units: _Optional[_Iterable[_Union[VocabCode, _Mapping]]] = ..., credential_levels: _Optional[_Iterable[_Union[VocabCode, _Mapping]]] = ..., relationship_kinds: _Optional[_Iterable[_Union[VocabCode, _Mapping]]] = ...) -> None: ...

class VocabCode(_message.Message):
    __slots__ = ("code", "label", "description")
    CODE_FIELD_NUMBER: _ClassVar[int]
    LABEL_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    code: str
    label: str
    description: str
    def __init__(self, code: _Optional[str] = ..., label: _Optional[str] = ..., description: _Optional[str] = ...) -> None: ...

class UpdateRequirementRequest(_message.Message):
    __slots__ = ("requirement_id", "patch", "edit_reason")
    REQUIREMENT_ID_FIELD_NUMBER: _ClassVar[int]
    PATCH_FIELD_NUMBER: _ClassVar[int]
    EDIT_REASON_FIELD_NUMBER: _ClassVar[int]
    requirement_id: str
    patch: RequirementPatch
    edit_reason: str
    def __init__(self, requirement_id: _Optional[str] = ..., patch: _Optional[_Union[RequirementPatch, _Mapping]] = ..., edit_reason: _Optional[str] = ...) -> None: ...

class UpdateRequirementResponse(_message.Message):
    __slots__ = ("requirement",)
    REQUIREMENT_FIELD_NUMBER: _ClassVar[int]
    requirement: Requirement
    def __init__(self, requirement: _Optional[_Union[Requirement, _Mapping]] = ...) -> None: ...

class ReleaseRequirementsRequest(_message.Message):
    __slots__ = ("role_id",)
    ROLE_ID_FIELD_NUMBER: _ClassVar[int]
    role_id: str
    def __init__(self, role_id: _Optional[str] = ...) -> None: ...

class ReleaseRequirementsResponse(_message.Message):
    __slots__ = ("released", "run_started", "submission_id")
    RELEASED_FIELD_NUMBER: _ClassVar[int]
    RUN_STARTED_FIELD_NUMBER: _ClassVar[int]
    SUBMISSION_ID_FIELD_NUMBER: _ClassVar[int]
    released: _containers.RepeatedCompositeFieldContainer[Requirement]
    run_started: bool
    submission_id: str
    def __init__(self, released: _Optional[_Iterable[_Union[Requirement, _Mapping]]] = ..., run_started: _Optional[bool] = ..., submission_id: _Optional[str] = ...) -> None: ...

class HiringSettings(_message.Message):
    __slots__ = ("preferred_fraction",)
    PREFERRED_FRACTION_FIELD_NUMBER: _ClassVar[int]
    preferred_fraction: float
    def __init__(self, preferred_fraction: _Optional[float] = ...) -> None: ...

class GetHiringSettingsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetHiringSettingsResponse(_message.Message):
    __slots__ = ("settings",)
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    settings: HiringSettings
    def __init__(self, settings: _Optional[_Union[HiringSettings, _Mapping]] = ...) -> None: ...

class SetHiringSettingsRequest(_message.Message):
    __slots__ = ("settings",)
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    settings: HiringSettings
    def __init__(self, settings: _Optional[_Union[HiringSettings, _Mapping]] = ...) -> None: ...

class SetHiringSettingsResponse(_message.Message):
    __slots__ = ("settings",)
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    settings: HiringSettings
    def __init__(self, settings: _Optional[_Union[HiringSettings, _Mapping]] = ...) -> None: ...
