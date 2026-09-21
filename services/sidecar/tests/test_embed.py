import math

import pytest

from career_sidecar.embed import (
    PURPOSE_DOCUMENT,
    PURPOSE_QUERY,
    OllamaEmbedder,
    StubEmbedder,
    build_embedder,
    task_prefix,
)


def test_stub_is_deterministic_and_normalised():
    e = StubEmbedder(dimensions=768)
    a, b = e.embed(["hello", "hello"])
    assert a == b
    assert len(a) == 768
    assert math.isclose(math.sqrt(sum(x * x for x in a)), 1.0, rel_tol=1e-6)


def test_stub_differs_for_different_text():
    e = StubEmbedder(dimensions=32)
    a, b = e.embed(["one", "two"])
    assert a != b


def test_ollama_name_includes_model_and_recipe():
    e = OllamaEmbedder(base_url="http://ollama:11434", model="nomic-embed-text", dimensions=768)
    assert e.name == "ollama:nomic-embed-text#p1"
    other = OllamaEmbedder(
        base_url="http://ollama:11434", model="mxbai-embed-large", dimensions=768
    )
    assert other.name == "ollama:mxbai-embed-large#raw"


def test_nomic_task_prefixes_are_asymmetric():
    assert task_prefix("nomic-embed-text", PURPOSE_DOCUMENT) == "search_document: "
    assert task_prefix("nomic-embed-text:latest", PURPOSE_QUERY) == "search_query: "
    assert task_prefix("nomic-embed-text", "bogus") == "search_document: "
    assert task_prefix("mxbai-embed-large", PURPOSE_QUERY) == ""


def test_stub_ignores_purpose():
    e = StubEmbedder(dimensions=16)
    assert e.embed(["x"], PURPOSE_QUERY) == e.embed(["x"], PURPOSE_DOCUMENT)


def test_build_embedder_stub_name():
    assert build_embedder("stub", "", "", 768).name == "stub"


def test_build_embedder_rejects_unknown():
    with pytest.raises(RuntimeError):
        build_embedder("nope", "", "", 768)
