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

class Coverage(_message.Message):
    __slots__ = ("evidenced", "unresolved", "unmet", "equivalent_share", "self_attested_share", "formula_version", "preferred_fraction")
    EVIDENCED_FIELD_NUMBER: _ClassVar[int]
    UNRESOLVED_FIELD_NUMBER: _ClassVar[int]
    UNMET_FIELD_NUMBER: _ClassVar[int]
    EQUIVALENT_SHARE_FIELD_NUMBER: _ClassVar[int]
    SELF_ATTESTED_SHARE_FIELD_NUMBER: _ClassVar[int]
    FORMULA_VERSION_FIELD_NUMBER: _ClassVar[int]
    PREFERRED_FRACTION_FIELD_NUMBER: _ClassVar[int]
    evidenced: float
    unresolved: float
    unmet: float
    equivalent_share: float
    self_attested_share: float
    formula_version: str
    preferred_fraction: float
    def __init__(self, evidenced: _Optional[float] = ..., unresolved: _Optional[float] = ..., unmet: _Optional[float] = ..., equivalent_share: _Optional[float] = ..., self_attested_share: _Optional[float] = ..., formula_version: _Optional[str] = ..., preferred_fraction: _Optional[float] = ...) -> None: ...

class EvidenceRef(_message.Message):
    __slots__ = ("id", "source_kind", "title", "chunk_id", "withdrawn")
    ID_FIELD_NUMBER: _ClassVar[int]
    SOURCE_KIND_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    CHUNK_ID_FIELD_NUMBER: _ClassVar[int]
    WITHDRAWN_FIELD_NUMBER: _ClassVar[int]
    id: str
    source_kind: str
    title: str
    chunk_id: int
    withdrawn: bool
    def __init__(self, id: _Optional[str] = ..., source_kind: _Optional[str] = ..., title: _Optional[str] = ..., chunk_id: _Optional[int] = ..., withdrawn: _Optional[bool] = ...) -> None: ...

class RuleAdjustment(_message.Message):
    __slots__ = ("rule", "rule_version", "from_determination", "to_determination", "from_basis", "to_basis", "added_uncertainty", "reason")
    RULE_FIELD_NUMBER: _ClassVar[int]
    RULE_VERSION_FIELD_NUMBER: _ClassVar[int]
    FROM_DETERMINATION_FIELD_NUMBER: _ClassVar[int]
    TO_DETERMINATION_FIELD_NUMBER: _ClassVar[int]
    FROM_BASIS_FIELD_NUMBER: _ClassVar[int]
    TO_BASIS_FIELD_NUMBER: _ClassVar[int]
    ADDED_UNCERTAINTY_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    rule: str
    rule_version: str
    from_determination: str
    to_determination: str
    from_basis: str
    to_basis: str
    added_uncertainty: _containers.RepeatedScalarFieldContainer[str]
    reason: str
    def __init__(self, rule: _Optional[str] = ..., rule_version: _Optional[str] = ..., from_determination: _Optional[str] = ..., to_determination: _Optional[str] = ..., from_basis: _Optional[str] = ..., to_basis: _Optional[str] = ..., added_uncertainty: _Optional[_Iterable[str]] = ..., reason: _Optional[str] = ...) -> None: ...

class ReportedFacts(_message.Message):
    __slots__ = ("stated_span_years", "most_recent_year", "stated_scope", "parties_evidenced")
    STATED_SPAN_YEARS_FIELD_NUMBER: _ClassVar[int]
    MOST_RECENT_YEAR_FIELD_NUMBER: _ClassVar[int]
    STATED_SCOPE_FIELD_NUMBER: _ClassVar[int]
    PARTIES_EVIDENCED_FIELD_NUMBER: _ClassVar[int]
    stated_span_years: float
    most_recent_year: int
    stated_scope: ScopeThreshold
    parties_evidenced: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, stated_span_years: _Optional[float] = ..., most_recent_year: _Optional[int] = ..., stated_scope: _Optional[_Union[ScopeThreshold, _Mapping]] = ..., parties_evidenced: _Optional[_Iterable[str]] = ...) -> None: ...

class JudgmentOverride(_message.Message):
    __slots__ = ("determination", "basis", "role", "at", "by_display", "reason", "carried_from_attempt", "recheck", "model_determination_then", "model_basis_then")
    DETERMINATION_FIELD_NUMBER: _ClassVar[int]
    BASIS_FIELD_NUMBER: _ClassVar[int]
    ROLE_FIELD_NUMBER: _ClassVar[int]
    AT_FIELD_NUMBER: _ClassVar[int]
    BY_DISPLAY_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    CARRIED_FROM_ATTEMPT_FIELD_NUMBER: _ClassVar[int]
    RECHECK_FIELD_NUMBER: _ClassVar[int]
    MODEL_DETERMINATION_THEN_FIELD_NUMBER: _ClassVar[int]
    MODEL_BASIS_THEN_FIELD_NUMBER: _ClassVar[int]
    determination: str
    basis: str
    role: str
    at: _timestamp_pb2.Timestamp
    by_display: str
    reason: str
    carried_from_attempt: int
    recheck: bool
    model_determination_then: str
    model_basis_then: str
    def __init__(self, determination: _Optional[str] = ..., basis: _Optional[str] = ..., role: _Optional[str] = ..., at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., by_display: _Optional[str] = ..., reason: _Optional[str] = ..., carried_from_attempt: _Optional[int] = ..., recheck: _Optional[bool] = ..., model_determination_then: _Optional[str] = ..., model_basis_then: _Optional[str] = ...) -> None: ...

class EarlierOverride(_message.Message):
    __slots__ = ("attempt", "determination", "basis", "role", "at", "same_version")
    ATTEMPT_FIELD_NUMBER: _ClassVar[int]
    DETERMINATION_FIELD_NUMBER: _ClassVar[int]
    BASIS_FIELD_NUMBER: _ClassVar[int]
    ROLE_FIELD_NUMBER: _ClassVar[int]
    AT_FIELD_NUMBER: _ClassVar[int]
    SAME_VERSION_FIELD_NUMBER: _ClassVar[int]
    attempt: int
    determination: str
    basis: str
    role: str
    at: _timestamp_pb2.Timestamp
    same_version: bool
    def __init__(self, attempt: _Optional[int] = ..., determination: _Optional[str] = ..., basis: _Optional[str] = ..., role: _Optional[str] = ..., at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., same_version: _Optional[bool] = ...) -> None: ...

class Judgment(_message.Message):
    __slots__ = ("id", "requirement_id", "requirement_local_id", "determination", "basis", "uncertainty", "effective_determination", "effective_basis", "override", "derived_verdict", "rationale", "evidence", "rule_adjustments", "reported_facts", "gap_class", "not_assessed", "stale", "keyword_known", "keyword_found", "keyword_terms", "decision_id", "earlier_override")
    ID_FIELD_NUMBER: _ClassVar[int]
    REQUIREMENT_ID_FIELD_NUMBER: _ClassVar[int]
    REQUIREMENT_LOCAL_ID_FIELD_NUMBER: _ClassVar[int]
    DETERMINATION_FIELD_NUMBER: _ClassVar[int]
    BASIS_FIELD_NUMBER: _ClassVar[int]
    UNCERTAINTY_FIELD_NUMBER: _ClassVar[int]
    EFFECTIVE_DETERMINATION_FIELD_NUMBER: _ClassVar[int]
    EFFECTIVE_BASIS_FIELD_NUMBER: _ClassVar[int]
    OVERRIDE_FIELD_NUMBER: _ClassVar[int]
    DERIVED_VERDICT_FIELD_NUMBER: _ClassVar[int]
    RATIONALE_FIELD_NUMBER: _ClassVar[int]
    EVIDENCE_FIELD_NUMBER: _ClassVar[int]
    RULE_ADJUSTMENTS_FIELD_NUMBER: _ClassVar[int]
    REPORTED_FACTS_FIELD_NUMBER: _ClassVar[int]
    GAP_CLASS_FIELD_NUMBER: _ClassVar[int]
    NOT_ASSESSED_FIELD_NUMBER: _ClassVar[int]
    STALE_FIELD_NUMBER: _ClassVar[int]
    KEYWORD_KNOWN_FIELD_NUMBER: _ClassVar[int]
    KEYWORD_FOUND_FIELD_NUMBER: _ClassVar[int]
    KEYWORD_TERMS_FIELD_NUMBER: _ClassVar[int]
    DECISION_ID_FIELD_NUMBER: _ClassVar[int]
    EARLIER_OVERRIDE_FIELD_NUMBER: _ClassVar[int]
    id: str
    requirement_id: str
    requirement_local_id: str
    determination: str
    basis: str
    uncertainty: _containers.RepeatedScalarFieldContainer[str]
    effective_determination: str
    effective_basis: str
    override: JudgmentOverride
    derived_verdict: str
    rationale: str
    evidence: _containers.RepeatedCompositeFieldContainer[EvidenceRef]
    rule_adjustments: _containers.RepeatedCompositeFieldContainer[RuleAdjustment]
    reported_facts: ReportedFacts
    gap_class: str
    not_assessed: bool
    stale: bool
    keyword_known: bool
    keyword_found: bool
    keyword_terms: _containers.RepeatedScalarFieldContainer[str]
    decision_id: int
    earlier_override: EarlierOverride
    def __init__(self, id: _Optional[str] = ..., requirement_id: _Optional[str] = ..., requirement_local_id: _Optional[str] = ..., determination: _Optional[str] = ..., basis: _Optional[str] = ..., uncertainty: _Optional[_Iterable[str]] = ..., effective_determination: _Optional[str] = ..., effective_basis: _Optional[str] = ..., override: _Optional[_Union[JudgmentOverride, _Mapping]] = ..., derived_verdict: _Optional[str] = ..., rationale: _Optional[str] = ..., evidence: _Optional[_Iterable[_Union[EvidenceRef, _Mapping]]] = ..., rule_adjustments: _Optional[_Iterable[_Union[RuleAdjustment, _Mapping]]] = ..., reported_facts: _Optional[_Union[ReportedFacts, _Mapping]] = ..., gap_class: _Optional[str] = ..., not_assessed: _Optional[bool] = ..., stale: _Optional[bool] = ..., keyword_known: _Optional[bool] = ..., keyword_found: _Optional[bool] = ..., keyword_terms: _Optional[_Iterable[str]] = ..., decision_id: _Optional[int] = ..., earlier_override: _Optional[_Union[EarlierOverride, _Mapping]] = ...) -> None: ...

class MatchRecord(_message.Message):
    __slots__ = ("run_id", "role_id", "submission_id", "attempt", "supersedes_run_id", "superseded", "run_trigger", "role_title", "employer", "coverage", "requirements", "judgments", "override_count", "keyword_rule", "keyword_skipped", "versions", "fit", "match_score", "finished_at", "corpus_documents", "model", "requirements_edited", "reviewed_score", "reviewed_fit")
    class VersionsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    RUN_ID_FIELD_NUMBER: _ClassVar[int]
    ROLE_ID_FIELD_NUMBER: _ClassVar[int]
    SUBMISSION_ID_FIELD_NUMBER: _ClassVar[int]
    ATTEMPT_FIELD_NUMBER: _ClassVar[int]
    SUPERSEDES_RUN_ID_FIELD_NUMBER: _ClassVar[int]
    SUPERSEDED_FIELD_NUMBER: _ClassVar[int]
    RUN_TRIGGER_FIELD_NUMBER: _ClassVar[int]
    ROLE_TITLE_FIELD_NUMBER: _ClassVar[int]
    EMPLOYER_FIELD_NUMBER: _ClassVar[int]
    COVERAGE_FIELD_NUMBER: _ClassVar[int]
    REQUIREMENTS_FIELD_NUMBER: _ClassVar[int]
    JUDGMENTS_FIELD_NUMBER: _ClassVar[int]
    OVERRIDE_COUNT_FIELD_NUMBER: _ClassVar[int]
    KEYWORD_RULE_FIELD_NUMBER: _ClassVar[int]
    KEYWORD_SKIPPED_FIELD_NUMBER: _ClassVar[int]
    VERSIONS_FIELD_NUMBER: _ClassVar[int]
    FIT_FIELD_NUMBER: _ClassVar[int]
    MATCH_SCORE_FIELD_NUMBER: _ClassVar[int]
    FINISHED_AT_FIELD_NUMBER: _ClassVar[int]
    CORPUS_DOCUMENTS_FIELD_NUMBER: _ClassVar[int]
    MODEL_FIELD_NUMBER: _ClassVar[int]
    REQUIREMENTS_EDITED_FIELD_NUMBER: _ClassVar[int]
    REVIEWED_SCORE_FIELD_NUMBER: _ClassVar[int]
    REVIEWED_FIT_FIELD_NUMBER: _ClassVar[int]
    run_id: str
    role_id: str
    submission_id: str
    attempt: int
    supersedes_run_id: str
    superseded: bool
    run_trigger: str
    role_title: str
    employer: str
    coverage: Coverage
    requirements: _containers.RepeatedCompositeFieldContainer[Requirement]
    judgments: _containers.RepeatedCompositeFieldContainer[Judgment]
    override_count: int
    keyword_rule: str
    keyword_skipped: str
    versions: _containers.ScalarMap[str, str]
    fit: str
    match_score: float
    finished_at: _timestamp_pb2.Timestamp
    corpus_documents: int
    model: str
    requirements_edited: bool
    reviewed_score: float
    reviewed_fit: str
    def __init__(self, run_id: _Optional[str] = ..., role_id: _Optional[str] = ..., submission_id: _Optional[str] = ..., attempt: _Optional[int] = ..., supersedes_run_id: _Optional[str] = ..., superseded: _Optional[bool] = ..., run_trigger: _Optional[str] = ..., role_title: _Optional[str] = ..., employer: _Optional[str] = ..., coverage: _Optional[_Union[Coverage, _Mapping]] = ..., requirements: _Optional[_Iterable[_Union[Requirement, _Mapping]]] = ..., judgments: _Optional[_Iterable[_Union[Judgment, _Mapping]]] = ..., override_count: _Optional[int] = ..., keyword_rule: _Optional[str] = ..., keyword_skipped: _Optional[str] = ..., versions: _Optional[_Mapping[str, str]] = ..., fit: _Optional[str] = ..., match_score: _Optional[float] = ..., finished_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., corpus_documents: _Optional[int] = ..., model: _Optional[str] = ..., requirements_edited: _Optional[bool] = ..., reviewed_score: _Optional[float] = ..., reviewed_fit: _Optional[str] = ...) -> None: ...

class OverrideChoice(_message.Message):
    __slots__ = ("determination", "bases")
    DETERMINATION_FIELD_NUMBER: _ClassVar[int]
    BASES_FIELD_NUMBER: _ClassVar[int]
    determination: str
    bases: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, determination: _Optional[str] = ..., bases: _Optional[_Iterable[str]] = ...) -> None: ...

class GetMatchRecordRequest(_message.Message):
    __slots__ = ("submission_id", "run_id", "as_member")
    SUBMISSION_ID_FIELD_NUMBER: _ClassVar[int]
    RUN_ID_FIELD_NUMBER: _ClassVar[int]
    AS_MEMBER_FIELD_NUMBER: _ClassVar[int]
    submission_id: str
    run_id: str
    as_member: bool
    def __init__(self, submission_id: _Optional[str] = ..., run_id: _Optional[str] = ..., as_member: _Optional[bool] = ...) -> None: ...

class GetMatchRecordResponse(_message.Message):
    __slots__ = ("record", "determinations", "bases", "uncertainties", "types", "classes", "source_kinds", "override_roles", "can_override", "override_choices")
    RECORD_FIELD_NUMBER: _ClassVar[int]
    DETERMINATIONS_FIELD_NUMBER: _ClassVar[int]
    BASES_FIELD_NUMBER: _ClassVar[int]
    UNCERTAINTIES_FIELD_NUMBER: _ClassVar[int]
    TYPES_FIELD_NUMBER: _ClassVar[int]
    CLASSES_FIELD_NUMBER: _ClassVar[int]
    SOURCE_KINDS_FIELD_NUMBER: _ClassVar[int]
    OVERRIDE_ROLES_FIELD_NUMBER: _ClassVar[int]
    CAN_OVERRIDE_FIELD_NUMBER: _ClassVar[int]
    OVERRIDE_CHOICES_FIELD_NUMBER: _ClassVar[int]
    record: MatchRecord
    determinations: _containers.RepeatedCompositeFieldContainer[VocabCode]
    bases: _containers.RepeatedCompositeFieldContainer[VocabCode]
    uncertainties: _containers.RepeatedCompositeFieldContainer[VocabCode]
    types: _containers.RepeatedCompositeFieldContainer[VocabCode]
    classes: _containers.RepeatedCompositeFieldContainer[VocabCode]
    source_kinds: _containers.RepeatedCompositeFieldContainer[VocabCode]
    override_roles: _containers.RepeatedCompositeFieldContainer[VocabCode]
    can_override: bool
    override_choices: _containers.RepeatedCompositeFieldContainer[OverrideChoice]
    def __init__(self, record: _Optional[_Union[MatchRecord, _Mapping]] = ..., determinations: _Optional[_Iterable[_Union[VocabCode, _Mapping]]] = ..., bases: _Optional[_Iterable[_Union[VocabCode, _Mapping]]] = ..., uncertainties: _Optional[_Iterable[_Union[VocabCode, _Mapping]]] = ..., types: _Optional[_Iterable[_Union[VocabCode, _Mapping]]] = ..., classes: _Optional[_Iterable[_Union[VocabCode, _Mapping]]] = ..., source_kinds: _Optional[_Iterable[_Union[VocabCode, _Mapping]]] = ..., override_roles: _Optional[_Iterable[_Union[VocabCode, _Mapping]]] = ..., can_override: _Optional[bool] = ..., override_choices: _Optional[_Iterable[_Union[OverrideChoice, _Mapping]]] = ...) -> None: ...

class OverrideJudgmentRequest(_message.Message):
    __slots__ = ("judgment_id", "determination", "basis", "reason", "role", "withdraw")
    JUDGMENT_ID_FIELD_NUMBER: _ClassVar[int]
    DETERMINATION_FIELD_NUMBER: _ClassVar[int]
    BASIS_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    ROLE_FIELD_NUMBER: _ClassVar[int]
    WITHDRAW_FIELD_NUMBER: _ClassVar[int]
    judgment_id: str
    determination: str
    basis: str
    reason: str
    role: str
    withdraw: bool
    def __init__(self, judgment_id: _Optional[str] = ..., determination: _Optional[str] = ..., basis: _Optional[str] = ..., reason: _Optional[str] = ..., role: _Optional[str] = ..., withdraw: _Optional[bool] = ...) -> None: ...

class OverrideJudgmentResponse(_message.Message):
    __slots__ = ("record",)
    RECORD_FIELD_NUMBER: _ClassVar[int]
    record: MatchRecord
    def __init__(self, record: _Optional[_Union[MatchRecord, _Mapping]] = ...) -> None: ...
