import math

import pytest

from career_sidecar.embed import OllamaEmbedder, StubEmbedder, build_embedder


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


def test_ollama_name_includes_model():
    e = OllamaEmbedder(base_url="http://ollama:11434", model="nomic-embed-text", dimensions=768)
    assert e.name == "ollama:nomic-embed-text"


def test_build_embedder_stub_name():
    assert build_embedder("stub", "", "", 768).name == "stub"


def test_build_embedder_rejects_unknown():
    with pytest.raises(RuntimeError):
        build_embedder("nope", "", "", 768)
