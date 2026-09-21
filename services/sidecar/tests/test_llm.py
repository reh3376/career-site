import pytest

from career_sidecar.llm import OllamaLLM, StubLLM, build_llm, strip_thinking


def test_strip_thinking_removes_scaffolding():
    raw = "<think>\nplanning...\n</think>\n# Résumé\n\nBody."
    assert strip_thinking(raw) == "# Résumé\n\nBody."
    assert strip_thinking("no scaffolding") == "no scaffolding"


def test_stub_is_deterministic_and_echoes_first_line():
    llm = StubLLM()
    a = llm.generate("sys", "Task line one\nmore")
    b = llm.generate("sys", "Task line one\nmore")
    assert a == b
    assert "Task line one" in a.text
    assert a.model == "stub"
    assert a.finish_reason == "stop"


def test_stub_json_mode_returns_json():
    import json

    out = StubLLM().generate("", "hello", json_mode=True)
    assert json.loads(out.text)["stub"] is True


def test_stub_honours_json_schema():
    import json

    schema = json.dumps(
        {
            "type": "object",
            "required": ["requirements"],
            "properties": {
                "requirements": {
                    "type": "array",
                    "minItems": 2,
                    "items": {
                        "type": "object",
                        "required": ["id", "text", "category", "weight"],
                        "properties": {
                            "id": {"type": "string"},
                            "text": {"type": "string"},
                            "category": {"type": "string", "enum": ["must", "nice"]},
                            "weight": {"type": "integer", "minimum": 1},
                        },
                    },
                }
            },
        }
    )
    out = json.loads(StubLLM().generate("", "x", json_schema=schema).text)
    assert len(out["requirements"]) == 2
    assert out["requirements"][0]["category"] == "must"
    assert out["requirements"][0]["weight"] == 1


def test_ollama_name_includes_model():
    llm = OllamaLLM(base_url="http://ollama:11434", model="qwen3:14b")
    assert llm.name == "ollama:qwen3:14b"


def test_build_llm_rejects_unknown():
    with pytest.raises(RuntimeError):
        build_llm("nope", "", "", 10)
    assert build_llm("stub", "", "", 10).name == "stub"
