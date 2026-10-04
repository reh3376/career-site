from buf.validate import validate_pb2 as _validate_pb2
from career.v1 import options_pb2 as _options_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Intake(_message.Message):
    __slots__ = ("display_name", "age_range", "education", "occupation", "email", "wants_results")
    DISPLAY_NAME_FIELD_NUMBER: _ClassVar[int]
    AGE_RANGE_FIELD_NUMBER: _ClassVar[int]
    EDUCATION_FIELD_NUMBER: _ClassVar[int]
    OCCUPATION_FIELD_NUMBER: _ClassVar[int]
    EMAIL_FIELD_NUMBER: _ClassVar[int]
    WANTS_RESULTS_FIELD_NUMBER: _ClassVar[int]
    display_name: str
    age_range: str
    education: str
    occupation: str
    email: str
    wants_results: bool
    def __init__(self, display_name: _Optional[str] = ..., age_range: _Optional[str] = ..., education: _Optional[str] = ..., occupation: _Optional[str] = ..., email: _Optional[str] = ..., wants_results: _Optional[bool] = ...) -> None: ...

class Conditions(_message.Message):
    __slots__ = ("audio_mode", "device_class", "tap_check_passed", "baseline_rt_ms", "baseline_rt_sd_ms")
    AUDIO_MODE_FIELD_NUMBER: _ClassVar[int]
    DEVICE_CLASS_FIELD_NUMBER: _ClassVar[int]
    TAP_CHECK_PASSED_FIELD_NUMBER: _ClassVar[int]
    BASELINE_RT_MS_FIELD_NUMBER: _ClassVar[int]
    BASELINE_RT_SD_MS_FIELD_NUMBER: _ClassVar[int]
    audio_mode: str
    device_class: str
    tap_check_passed: bool
    baseline_rt_ms: int
    baseline_rt_sd_ms: int
    def __init__(self, audio_mode: _Optional[str] = ..., device_class: _Optional[str] = ..., tap_check_passed: _Optional[bool] = ..., baseline_rt_ms: _Optional[int] = ..., baseline_rt_sd_ms: _Optional[int] = ...) -> None: ...

class StartSessionRequest(_message.Message):
    __slots__ = ("intake", "conditions", "synthetic")
    INTAKE_FIELD_NUMBER: _ClassVar[int]
    CONDITIONS_FIELD_NUMBER: _ClassVar[int]
    SYNTHETIC_FIELD_NUMBER: _ClassVar[int]
    intake: Intake
    conditions: Conditions
    synthetic: bool
    def __init__(self, intake: _Optional[_Union[Intake, _Mapping]] = ..., conditions: _Optional[_Union[Conditions, _Mapping]] = ..., synthetic: _Optional[bool] = ...) -> None: ...

class StartSessionResponse(_message.Message):
    __slots__ = ("session_key", "practice", "block_count", "question_count")
    SESSION_KEY_FIELD_NUMBER: _ClassVar[int]
    PRACTICE_FIELD_NUMBER: _ClassVar[int]
    BLOCK_COUNT_FIELD_NUMBER: _ClassVar[int]
    QUESTION_COUNT_FIELD_NUMBER: _ClassVar[int]
    session_key: str
    practice: Block
    block_count: int
    question_count: int
    def __init__(self, session_key: _Optional[str] = ..., practice: _Optional[_Union[Block, _Mapping]] = ..., block_count: _Optional[int] = ..., question_count: _Optional[int] = ...) -> None: ...

class GetBlockRequest(_message.Message):
    __slots__ = ("session_key", "block_no")
    SESSION_KEY_FIELD_NUMBER: _ClassVar[int]
    BLOCK_NO_FIELD_NUMBER: _ClassVar[int]
    session_key: str
    block_no: int
    def __init__(self, session_key: _Optional[str] = ..., block_no: _Optional[int] = ...) -> None: ...

class GetBlockResponse(_message.Message):
    __slots__ = ("block",)
    BLOCK_FIELD_NUMBER: _ClassVar[int]
    block: Block
    def __init__(self, block: _Optional[_Union[Block, _Mapping]] = ...) -> None: ...

class Block(_message.Message):
    __slots__ = ("block_no", "load", "digits", "transform", "questions")
    BLOCK_NO_FIELD_NUMBER: _ClassVar[int]
    LOAD_FIELD_NUMBER: _ClassVar[int]
    DIGITS_FIELD_NUMBER: _ClassVar[int]
    TRANSFORM_FIELD_NUMBER: _ClassVar[int]
    QUESTIONS_FIELD_NUMBER: _ClassVar[int]
    block_no: int
    load: str
    digits: str
    transform: str
    questions: _containers.RepeatedCompositeFieldContainer[Question]
    def __init__(self, block_no: _Optional[int] = ..., load: _Optional[str] = ..., digits: _Optional[str] = ..., transform: _Optional[str] = ..., questions: _Optional[_Iterable[_Union[Question, _Mapping]]] = ...) -> None: ...

class Question(_message.Message):
    __slots__ = ("code", "version", "prompt", "reminder", "options", "position_overall")
    CODE_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    PROMPT_FIELD_NUMBER: _ClassVar[int]
    REMINDER_FIELD_NUMBER: _ClassVar[int]
    OPTIONS_FIELD_NUMBER: _ClassVar[int]
    POSITION_OVERALL_FIELD_NUMBER: _ClassVar[int]
    code: str
    version: int
    prompt: str
    reminder: str
    options: _containers.RepeatedScalarFieldContainer[str]
    position_overall: int
    def __init__(self, code: _Optional[str] = ..., version: _Optional[int] = ..., prompt: _Optional[str] = ..., reminder: _Optional[str] = ..., options: _Optional[_Iterable[str]] = ..., position_overall: _Optional[int] = ...) -> None: ...

class SubmitAnswerRequest(_message.Message):
    __slots__ = ("session_key", "block_no", "position_overall", "chosen_index", "latency_ms", "confidence")
    SESSION_KEY_FIELD_NUMBER: _ClassVar[int]
    BLOCK_NO_FIELD_NUMBER: _ClassVar[int]
    POSITION_OVERALL_FIELD_NUMBER: _ClassVar[int]
    CHOSEN_INDEX_FIELD_NUMBER: _ClassVar[int]
    LATENCY_MS_FIELD_NUMBER: _ClassVar[int]
    CONFIDENCE_FIELD_NUMBER: _ClassVar[int]
    session_key: str
    block_no: int
    position_overall: int
    chosen_index: int
    latency_ms: int
    confidence: int
    def __init__(self, session_key: _Optional[str] = ..., block_no: _Optional[int] = ..., position_overall: _Optional[int] = ..., chosen_index: _Optional[int] = ..., latency_ms: _Optional[int] = ..., confidence: _Optional[int] = ...) -> None: ...

class SubmitAnswerResponse(_message.Message):
    __slots__ = ("stored",)
    STORED_FIELD_NUMBER: _ClassVar[int]
    stored: bool
    def __init__(self, stored: _Optional[bool] = ...) -> None: ...

class SubmitRecallRequest(_message.Message):
    __slots__ = ("session_key", "block_no", "digits", "latency_ms")
    SESSION_KEY_FIELD_NUMBER: _ClassVar[int]
    BLOCK_NO_FIELD_NUMBER: _ClassVar[int]
    DIGITS_FIELD_NUMBER: _ClassVar[int]
    LATENCY_MS_FIELD_NUMBER: _ClassVar[int]
    session_key: str
    block_no: int
    digits: str
    latency_ms: int
    def __init__(self, session_key: _Optional[str] = ..., block_no: _Optional[int] = ..., digits: _Optional[str] = ..., latency_ms: _Optional[int] = ...) -> None: ...

class SubmitRecallResponse(_message.Message):
    __slots__ = ("stored",)
    STORED_FIELD_NUMBER: _ClassVar[int]
    stored: bool
    def __init__(self, stored: _Optional[bool] = ...) -> None: ...

class FinishSessionRequest(_message.Message):
    __slots__ = ("session_key", "recall_strategy")
    SESSION_KEY_FIELD_NUMBER: _ClassVar[int]
    RECALL_STRATEGY_FIELD_NUMBER: _ClassVar[int]
    session_key: str
    recall_strategy: str
    def __init__(self, session_key: _Optional[str] = ..., recall_strategy: _Optional[str] = ...) -> None: ...

class FinishSessionResponse(_message.Message):
    __slots__ = ("correct", "total")
    CORRECT_FIELD_NUMBER: _ClassVar[int]
    TOTAL_FIELD_NUMBER: _ClassVar[int]
    correct: int
    total: int
    def __init__(self, correct: _Optional[int] = ..., total: _Optional[int] = ...) -> None: ...
