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

class ListContentRequest(_message.Message):
    __slots__ = ("type", "track_ids", "tags", "order", "page")
    class Order(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        ORDER_UNSPECIFIED: _ClassVar[ListContentRequest.Order]
        ORDER_RELEVANCE: _ClassVar[ListContentRequest.Order]
        ORDER_NEWEST: _ClassVar[ListContentRequest.Order]
        ORDER_CHRONOLOGICAL: _ClassVar[ListContentRequest.Order]
    ORDER_UNSPECIFIED: ListContentRequest.Order
    ORDER_RELEVANCE: ListContentRequest.Order
    ORDER_NEWEST: ListContentRequest.Order
    ORDER_CHRONOLOGICAL: ListContentRequest.Order
    TYPE_FIELD_NUMBER: _ClassVar[int]
    TRACK_IDS_FIELD_NUMBER: _ClassVar[int]
    TAGS_FIELD_NUMBER: _ClassVar[int]
    ORDER_FIELD_NUMBER: _ClassVar[int]
    PAGE_FIELD_NUMBER: _ClassVar[int]
    type: _common_pb2.ContentType
    track_ids: _containers.RepeatedScalarFieldContainer[str]
    tags: _containers.RepeatedScalarFieldContainer[str]
    order: ListContentRequest.Order
    page: _common_pb2.PageRequest
    def __init__(self, type: _Optional[_Union[_common_pb2.ContentType, str]] = ..., track_ids: _Optional[_Iterable[str]] = ..., tags: _Optional[_Iterable[str]] = ..., order: _Optional[_Union[ListContentRequest.Order, str]] = ..., page: _Optional[_Union[_common_pb2.PageRequest, _Mapping]] = ...) -> None: ...

class ListContentResponse(_message.Message):
    __slots__ = ("items", "page", "tailored")
    ITEMS_FIELD_NUMBER: _ClassVar[int]
    PAGE_FIELD_NUMBER: _ClassVar[int]
    TAILORED_FIELD_NUMBER: _ClassVar[int]
    items: _containers.RepeatedCompositeFieldContainer[_common_pb2.ContentSummary]
    page: _common_pb2.PageResponse
    tailored: bool
    def __init__(self, items: _Optional[_Iterable[_Union[_common_pb2.ContentSummary, _Mapping]]] = ..., page: _Optional[_Union[_common_pb2.PageResponse, _Mapping]] = ..., tailored: _Optional[bool] = ...) -> None: ...

class GetContentRequest(_message.Message):
    __slots__ = ("slug", "id")
    SLUG_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    slug: str
    id: str
    def __init__(self, slug: _Optional[str] = ..., id: _Optional[str] = ...) -> None: ...

class Accomplishment(_message.Message):
    __slots__ = ("text", "metric", "detail", "tracks")
    TEXT_FIELD_NUMBER: _ClassVar[int]
    METRIC_FIELD_NUMBER: _ClassVar[int]
    DETAIL_FIELD_NUMBER: _ClassVar[int]
    TRACKS_FIELD_NUMBER: _ClassVar[int]
    text: str
    metric: str
    detail: str
    tracks: _containers.RepeatedCompositeFieldContainer[_common_pb2.TrackWeight]
    def __init__(self, text: _Optional[str] = ..., metric: _Optional[str] = ..., detail: _Optional[str] = ..., tracks: _Optional[_Iterable[_Union[_common_pb2.TrackWeight, _Mapping]]] = ...) -> None: ...

class RoleDetail(_message.Message):
    __slots__ = ("organization", "location", "start", "end", "accomplishments", "technologies")
    ORGANIZATION_FIELD_NUMBER: _ClassVar[int]
    LOCATION_FIELD_NUMBER: _ClassVar[int]
    START_FIELD_NUMBER: _ClassVar[int]
    END_FIELD_NUMBER: _ClassVar[int]
    ACCOMPLISHMENTS_FIELD_NUMBER: _ClassVar[int]
    TECHNOLOGIES_FIELD_NUMBER: _ClassVar[int]
    organization: str
    location: str
    start: _timestamp_pb2.Timestamp
    end: _timestamp_pb2.Timestamp
    accomplishments: _containers.RepeatedCompositeFieldContainer[Accomplishment]
    technologies: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, organization: _Optional[str] = ..., location: _Optional[str] = ..., start: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., end: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., accomplishments: _Optional[_Iterable[_Union[Accomplishment, _Mapping]]] = ..., technologies: _Optional[_Iterable[str]] = ...) -> None: ...

class GithubMeta(_message.Message):
    __slots__ = ("stars", "language", "pushed_at", "fetched_at")
    STARS_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    PUSHED_AT_FIELD_NUMBER: _ClassVar[int]
    FETCHED_AT_FIELD_NUMBER: _ClassVar[int]
    stars: int
    language: str
    pushed_at: _timestamp_pb2.Timestamp
    fetched_at: _timestamp_pb2.Timestamp
    def __init__(self, stars: _Optional[int] = ..., language: _Optional[str] = ..., pushed_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., fetched_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ProjectDetail(_message.Message):
    __slots__ = ("role", "status", "repo_url", "stack", "problem", "approach", "outcomes", "diagram_mermaid", "github")
    class Status(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        STATUS_UNSPECIFIED: _ClassVar[ProjectDetail.Status]
        STATUS_ACTIVE: _ClassVar[ProjectDetail.Status]
        STATUS_MAINTAINED: _ClassVar[ProjectDetail.Status]
        STATUS_ARCHIVED: _ClassVar[ProjectDetail.Status]
        STATUS_EXPLORATION: _ClassVar[ProjectDetail.Status]
    STATUS_UNSPECIFIED: ProjectDetail.Status
    STATUS_ACTIVE: ProjectDetail.Status
    STATUS_MAINTAINED: ProjectDetail.Status
    STATUS_ARCHIVED: ProjectDetail.Status
    STATUS_EXPLORATION: ProjectDetail.Status
    ROLE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    REPO_URL_FIELD_NUMBER: _ClassVar[int]
    STACK_FIELD_NUMBER: _ClassVar[int]
    PROBLEM_FIELD_NUMBER: _ClassVar[int]
    APPROACH_FIELD_NUMBER: _ClassVar[int]
    OUTCOMES_FIELD_NUMBER: _ClassVar[int]
    DIAGRAM_MERMAID_FIELD_NUMBER: _ClassVar[int]
    GITHUB_FIELD_NUMBER: _ClassVar[int]
    role: str
    status: ProjectDetail.Status
    repo_url: str
    stack: _containers.RepeatedScalarFieldContainer[str]
    problem: str
    approach: str
    outcomes: _containers.RepeatedScalarFieldContainer[str]
    diagram_mermaid: str
    github: GithubMeta
    def __init__(self, role: _Optional[str] = ..., status: _Optional[_Union[ProjectDetail.Status, str]] = ..., repo_url: _Optional[str] = ..., stack: _Optional[_Iterable[str]] = ..., problem: _Optional[str] = ..., approach: _Optional[str] = ..., outcomes: _Optional[_Iterable[str]] = ..., diagram_mermaid: _Optional[str] = ..., github: _Optional[_Union[GithubMeta, _Mapping]] = ...) -> None: ...

class ArticleDetail(_message.Message):
    __slots__ = ("venue", "canonical_url", "series", "series_order", "reading_time_minutes", "hosted")
    VENUE_FIELD_NUMBER: _ClassVar[int]
    CANONICAL_URL_FIELD_NUMBER: _ClassVar[int]
    SERIES_FIELD_NUMBER: _ClassVar[int]
    SERIES_ORDER_FIELD_NUMBER: _ClassVar[int]
    READING_TIME_MINUTES_FIELD_NUMBER: _ClassVar[int]
    HOSTED_FIELD_NUMBER: _ClassVar[int]
    venue: str
    canonical_url: str
    series: str
    series_order: int
    reading_time_minutes: int
    hosted: bool
    def __init__(self, venue: _Optional[str] = ..., canonical_url: _Optional[str] = ..., series: _Optional[str] = ..., series_order: _Optional[int] = ..., reading_time_minutes: _Optional[int] = ..., hosted: _Optional[bool] = ...) -> None: ...

class PresentationDetail(_message.Message):
    __slots__ = ("event", "abstract", "slides_count", "file_url", "file_url_expires_at")
    EVENT_FIELD_NUMBER: _ClassVar[int]
    ABSTRACT_FIELD_NUMBER: _ClassVar[int]
    SLIDES_COUNT_FIELD_NUMBER: _ClassVar[int]
    FILE_URL_FIELD_NUMBER: _ClassVar[int]
    FILE_URL_EXPIRES_AT_FIELD_NUMBER: _ClassVar[int]
    event: str
    abstract: str
    slides_count: int
    file_url: str
    file_url_expires_at: _timestamp_pb2.Timestamp
    def __init__(self, event: _Optional[str] = ..., abstract: _Optional[str] = ..., slides_count: _Optional[int] = ..., file_url: _Optional[str] = ..., file_url_expires_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class CredentialDetail(_message.Message):
    __slots__ = ("kind", "issuer", "date")
    class Kind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        KIND_UNSPECIFIED: _ClassVar[CredentialDetail.Kind]
        KIND_AWARD: _ClassVar[CredentialDetail.Kind]
        KIND_EDUCATION: _ClassVar[CredentialDetail.Kind]
        KIND_SERVICE: _ClassVar[CredentialDetail.Kind]
        KIND_ELIGIBILITY: _ClassVar[CredentialDetail.Kind]
    KIND_UNSPECIFIED: CredentialDetail.Kind
    KIND_AWARD: CredentialDetail.Kind
    KIND_EDUCATION: CredentialDetail.Kind
    KIND_SERVICE: CredentialDetail.Kind
    KIND_ELIGIBILITY: CredentialDetail.Kind
    KIND_FIELD_NUMBER: _ClassVar[int]
    ISSUER_FIELD_NUMBER: _ClassVar[int]
    DATE_FIELD_NUMBER: _ClassVar[int]
    kind: CredentialDetail.Kind
    issuer: str
    date: _timestamp_pb2.Timestamp
    def __init__(self, kind: _Optional[_Union[CredentialDetail.Kind, str]] = ..., issuer: _Optional[str] = ..., date: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ImageVariant(_message.Message):
    __slots__ = ("url", "width", "height", "mime_type")
    URL_FIELD_NUMBER: _ClassVar[int]
    WIDTH_FIELD_NUMBER: _ClassVar[int]
    HEIGHT_FIELD_NUMBER: _ClassVar[int]
    MIME_TYPE_FIELD_NUMBER: _ClassVar[int]
    url: str
    width: int
    height: int
    mime_type: str
    def __init__(self, url: _Optional[str] = ..., width: _Optional[int] = ..., height: _Optional[int] = ..., mime_type: _Optional[str] = ...) -> None: ...

class PhotoDetail(_message.Message):
    __slots__ = ("variants", "caption", "context", "taken", "alt")
    VARIANTS_FIELD_NUMBER: _ClassVar[int]
    CAPTION_FIELD_NUMBER: _ClassVar[int]
    CONTEXT_FIELD_NUMBER: _ClassVar[int]
    TAKEN_FIELD_NUMBER: _ClassVar[int]
    ALT_FIELD_NUMBER: _ClassVar[int]
    variants: _containers.RepeatedCompositeFieldContainer[ImageVariant]
    caption: str
    context: str
    taken: _timestamp_pb2.Timestamp
    alt: str
    def __init__(self, variants: _Optional[_Iterable[_Union[ImageVariant, _Mapping]]] = ..., caption: _Optional[str] = ..., context: _Optional[str] = ..., taken: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., alt: _Optional[str] = ...) -> None: ...

class ResumeDetail(_message.Message):
    __slots__ = ("variant", "version")
    VARIANT_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    variant: str
    version: str
    def __init__(self, variant: _Optional[str] = ..., version: _Optional[str] = ...) -> None: ...

class QaDetail(_message.Message):
    __slots__ = ("question", "answer")
    QUESTION_FIELD_NUMBER: _ClassVar[int]
    ANSWER_FIELD_NUMBER: _ClassVar[int]
    question: str
    answer: str
    def __init__(self, question: _Optional[str] = ..., answer: _Optional[str] = ...) -> None: ...

class Asset(_message.Message):
    __slots__ = ("kind", "url", "alt")
    KIND_FIELD_NUMBER: _ClassVar[int]
    URL_FIELD_NUMBER: _ClassVar[int]
    ALT_FIELD_NUMBER: _ClassVar[int]
    kind: str
    url: str
    alt: str
    def __init__(self, kind: _Optional[str] = ..., url: _Optional[str] = ..., alt: _Optional[str] = ...) -> None: ...

class ContentItem(_message.Message):
    __slots__ = ("summary", "body", "related", "assets", "role", "project", "article", "presentation", "credential", "photo", "resume", "qa")
    SUMMARY_FIELD_NUMBER: _ClassVar[int]
    BODY_FIELD_NUMBER: _ClassVar[int]
    RELATED_FIELD_NUMBER: _ClassVar[int]
    ASSETS_FIELD_NUMBER: _ClassVar[int]
    ROLE_FIELD_NUMBER: _ClassVar[int]
    PROJECT_FIELD_NUMBER: _ClassVar[int]
    ARTICLE_FIELD_NUMBER: _ClassVar[int]
    PRESENTATION_FIELD_NUMBER: _ClassVar[int]
    CREDENTIAL_FIELD_NUMBER: _ClassVar[int]
    PHOTO_FIELD_NUMBER: _ClassVar[int]
    RESUME_FIELD_NUMBER: _ClassVar[int]
    QA_FIELD_NUMBER: _ClassVar[int]
    summary: _common_pb2.ContentSummary
    body: str
    related: _containers.RepeatedCompositeFieldContainer[_common_pb2.ContentSummary]
    assets: _containers.RepeatedCompositeFieldContainer[Asset]
    role: RoleDetail
    project: ProjectDetail
    article: ArticleDetail
    presentation: PresentationDetail
    credential: CredentialDetail
    photo: PhotoDetail
    resume: ResumeDetail
    qa: QaDetail
    def __init__(self, summary: _Optional[_Union[_common_pb2.ContentSummary, _Mapping]] = ..., body: _Optional[str] = ..., related: _Optional[_Iterable[_Union[_common_pb2.ContentSummary, _Mapping]]] = ..., assets: _Optional[_Iterable[_Union[Asset, _Mapping]]] = ..., role: _Optional[_Union[RoleDetail, _Mapping]] = ..., project: _Optional[_Union[ProjectDetail, _Mapping]] = ..., article: _Optional[_Union[ArticleDetail, _Mapping]] = ..., presentation: _Optional[_Union[PresentationDetail, _Mapping]] = ..., credential: _Optional[_Union[CredentialDetail, _Mapping]] = ..., photo: _Optional[_Union[PhotoDetail, _Mapping]] = ..., resume: _Optional[_Union[ResumeDetail, _Mapping]] = ..., qa: _Optional[_Union[QaDetail, _Mapping]] = ...) -> None: ...

class GetContentResponse(_message.Message):
    __slots__ = ("item",)
    ITEM_FIELD_NUMBER: _ClassVar[int]
    item: ContentItem
    def __init__(self, item: _Optional[_Union[ContentItem, _Mapping]] = ...) -> None: ...

class WhatsNewRequest(_message.Message):
    __slots__ = ("since", "limit")
    SINCE_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    since: _timestamp_pb2.Timestamp
    limit: int
    def __init__(self, since: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., limit: _Optional[int] = ...) -> None: ...

class WhatsNewResponse(_message.Message):
    __slots__ = ("items", "since")
    ITEMS_FIELD_NUMBER: _ClassVar[int]
    SINCE_FIELD_NUMBER: _ClassVar[int]
    items: _containers.RepeatedCompositeFieldContainer[_common_pb2.ContentSummary]
    since: _timestamp_pb2.Timestamp
    def __init__(self, items: _Optional[_Iterable[_Union[_common_pb2.ContentSummary, _Mapping]]] = ..., since: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ListTracksRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListTracksResponse(_message.Message):
    __slots__ = ("tracks",)
    TRACKS_FIELD_NUMBER: _ClassVar[int]
    tracks: _containers.RepeatedCompositeFieldContainer[_common_pb2.Track]
    def __init__(self, tracks: _Optional[_Iterable[_Union[_common_pb2.Track, _Mapping]]] = ...) -> None: ...

class GetSkillsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class Skill(_message.Message):
    __slots__ = ("id", "name", "level", "years", "evidence", "tracks")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    LEVEL_FIELD_NUMBER: _ClassVar[int]
    YEARS_FIELD_NUMBER: _ClassVar[int]
    EVIDENCE_FIELD_NUMBER: _ClassVar[int]
    TRACKS_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    level: int
    years: int
    evidence: _containers.RepeatedCompositeFieldContainer[_common_pb2.ContentSummary]
    tracks: _containers.RepeatedCompositeFieldContainer[_common_pb2.TrackWeight]
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., level: _Optional[int] = ..., years: _Optional[int] = ..., evidence: _Optional[_Iterable[_Union[_common_pb2.ContentSummary, _Mapping]]] = ..., tracks: _Optional[_Iterable[_Union[_common_pb2.TrackWeight, _Mapping]]] = ...) -> None: ...

class SkillCategory(_message.Message):
    __slots__ = ("name", "skills")
    NAME_FIELD_NUMBER: _ClassVar[int]
    SKILLS_FIELD_NUMBER: _ClassVar[int]
    name: str
    skills: _containers.RepeatedCompositeFieldContainer[Skill]
    def __init__(self, name: _Optional[str] = ..., skills: _Optional[_Iterable[_Union[Skill, _Mapping]]] = ...) -> None: ...

class GetSkillsResponse(_message.Message):
    __slots__ = ("categories",)
    CATEGORIES_FIELD_NUMBER: _ClassVar[int]
    categories: _containers.RepeatedCompositeFieldContainer[SkillCategory]
    def __init__(self, categories: _Optional[_Iterable[_Union[SkillCategory, _Mapping]]] = ...) -> None: ...

class SearchRequest(_message.Message):
    __slots__ = ("query", "type", "track_ids", "page")
    QUERY_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    TRACK_IDS_FIELD_NUMBER: _ClassVar[int]
    PAGE_FIELD_NUMBER: _ClassVar[int]
    query: str
    type: _common_pb2.ContentType
    track_ids: _containers.RepeatedScalarFieldContainer[str]
    page: _common_pb2.PageRequest
    def __init__(self, query: _Optional[str] = ..., type: _Optional[_Union[_common_pb2.ContentType, str]] = ..., track_ids: _Optional[_Iterable[str]] = ..., page: _Optional[_Union[_common_pb2.PageRequest, _Mapping]] = ...) -> None: ...

class SearchHit(_message.Message):
    __slots__ = ("item", "snippet", "score")
    ITEM_FIELD_NUMBER: _ClassVar[int]
    SNIPPET_FIELD_NUMBER: _ClassVar[int]
    SCORE_FIELD_NUMBER: _ClassVar[int]
    item: _common_pb2.ContentSummary
    snippet: str
    score: float
    def __init__(self, item: _Optional[_Union[_common_pb2.ContentSummary, _Mapping]] = ..., snippet: _Optional[str] = ..., score: _Optional[float] = ...) -> None: ...

class SearchResponse(_message.Message):
    __slots__ = ("hits", "page")
    HITS_FIELD_NUMBER: _ClassVar[int]
    PAGE_FIELD_NUMBER: _ClassVar[int]
    hits: _containers.RepeatedCompositeFieldContainer[SearchHit]
    page: _common_pb2.PageResponse
    def __init__(self, hits: _Optional[_Iterable[_Union[SearchHit, _Mapping]]] = ..., page: _Optional[_Union[_common_pb2.PageResponse, _Mapping]] = ...) -> None: ...
