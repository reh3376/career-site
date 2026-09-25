"""The health check must answer "can I serve this model", not "am I
configured for one".

On 2026-09-25 those were the same answer. Stopping Ollama entirely did
not change what the sidecar reported: it read its own configuration and
said the model was ready. The api then started, logged "jd assessor
enabled", and failed every call later at request time, one posting at a
time. The previous time an untested degradation path ran, it scored
eight golden postings in three seconds with no verdicts behind any of
them and the numbers looked ordinary.

So each way this can fail gets a test.
"""

from __future__ import annotations

import io
import json
import urllib.error
from typing import Any

import pytest

from career_sidecar.llm import OllamaLLM, StubLLM


def _tags_response(names: list[str]) -> io.BytesIO:
    body = json.dumps({"models": [{"name": n} for n in names]}).encode("utf-8")
    stream = io.BytesIO(body)
    # urlopen's result is used as a context manager.
    stream.__enter__ = lambda self=stream: self  # type: ignore[attr-defined]
    stream.__exit__ = lambda *a, **k: False  # type: ignore[attr-defined]
    return stream


@pytest.fixture
def llm() -> OllamaLLM:
    return OllamaLLM(base_url="http://ollama:11434", model="qwen3:4b-q8_0")


def test_model_present_is_ready(monkeypatch: pytest.MonkeyPatch, llm: OllamaLLM) -> None:
    monkeypatch.setattr(
        "urllib.request.urlopen",
        lambda *a, **k: _tags_response(["qwen3:4b-q8_0", "nomic-embed-text:latest"]),
    )
    ok, detail = llm.probe()
    assert ok is True
    assert detail == ""


def test_model_absent_is_not_ready(monkeypatch: pytest.MonkeyPatch, llm: OllamaLLM) -> None:
    """The model was never pulled, or was deleted. The server is up, so a
    plain connectivity check would have called this healthy."""
    monkeypatch.setattr(
        "urllib.request.urlopen",
        lambda *a, **k: _tags_response(["nomic-embed-text:latest"]),
    )
    ok, detail = llm.probe()
    assert ok is False
    assert "qwen3:4b-q8_0" in detail
    # The detail names what IS available, because "not found" without
    # that leaves the reader guessing whether the server is even right.
    assert "nomic-embed-text:latest" in detail


def test_server_down_is_not_ready(monkeypatch: pytest.MonkeyPatch, llm: OllamaLLM) -> None:
    """The case that started this: Ollama stopped and nothing noticed."""

    def boom(*a: Any, **k: Any) -> Any:
        raise urllib.error.URLError("Connection refused")

    monkeypatch.setattr("urllib.request.urlopen", boom)
    ok, detail = llm.probe()
    assert ok is False
    assert "unreachable" in detail


def test_http_error_is_not_ready(monkeypatch: pytest.MonkeyPatch, llm: OllamaLLM) -> None:
    def boom(*a: Any, **k: Any) -> Any:
        raise urllib.error.HTTPError("http://ollama:11434/api/tags", 502, "Bad Gateway", {}, None)  # type: ignore[arg-type]

    monkeypatch.setattr("urllib.request.urlopen", boom)
    ok, detail = llm.probe()
    assert ok is False
    assert "502" in detail


def test_unexpected_exception_is_not_ready(monkeypatch: pytest.MonkeyPatch, llm: OllamaLLM) -> None:
    """A health check that raises is an outage of its own. Whatever goes
    wrong, the answer is False and a reason."""

    def boom(*a: Any, **k: Any) -> Any:
        raise ValueError("malformed")

    monkeypatch.setattr("urllib.request.urlopen", boom)
    ok, detail = llm.probe()
    assert ok is False
    assert "ValueError" in detail


def test_untagged_model_matches_latest(monkeypatch: pytest.MonkeyPatch) -> None:
    """Ollama resolves a bare name to its :latest tag, so the probe must
    too, or a correctly configured server reads as broken."""
    llm = OllamaLLM(base_url="http://ollama:11434", model="qwen3")
    monkeypatch.setattr(
        "urllib.request.urlopen", lambda *a, **k: _tags_response(["qwen3:latest"])
    )
    ok, detail = llm.probe()
    assert ok is True, detail


def test_stub_is_always_ready() -> None:
    """The stub is the implementation; there is nothing for it to reach."""
    ok, detail = StubLLM().probe()
    assert ok is True
    assert detail == ""
