"""LLM providers for the sidecar's Generate RPC.

``OllamaLLM`` talks to an Ollama server's /api/chat endpoint (local
development against the owner's machine, or the on-host service in
prod). ``StubLLM`` returns a deterministic canned completion so CI and
the default dev stack exercise the RPC without a model.

The sidecar is a provider gateway only: it never composes or rewrites
prompts. Prompt text and its version are owned by the Go API's prompt
registry so a prompt change is a versioned code change there.
"""

from __future__ import annotations

import json
import re
import urllib.error
import urllib.request
from dataclasses import dataclass
from typing import Protocol

# qwen3 (and other reasoning models) can emit <think>...</think>
# scaffolding even when asked not to; strip it defensively so callers
# only ever see the answer.
_THINK_RE = re.compile(r"<think>.*?</think>\s*", re.DOTALL)


@dataclass(frozen=True)
class GenerateResult:
    text: str
    model: str
    prompt_tokens: int
    completion_tokens: int
    finish_reason: str


class LLM(Protocol):
    name: str

    def generate(
        self,
        system: str,
        user: str,
        max_tokens: int = 0,
        temperature: float = 0.2,
        json_mode: bool = False,
        json_schema: str = "",
    ) -> GenerateResult: ...

    def probe(self) -> tuple[bool, str]:
        """Can this provider actually serve its model right now?

        Distinct from "is a provider configured", which is all the
        health check used to report. On 2026-09-25 stopping Ollama
        entirely did not change the sidecar's answer: it still said the
        model was ready, because it was reading its own configuration.
        Every call then failed later, at request time, one posting at a
        time, and the previous time an untested degradation path ran it
        produced eight plausible scores from nothing.

        Returns (ok, detail). detail explains a false and is empty on
        success.
        """
        ...


def strip_thinking(text: str) -> str:
    return _THINK_RE.sub("", text).strip()


def parse_schema(json_schema: str) -> dict | None:
    """Parse a caller-supplied JSON schema; raise on malformed input so a
    bad schema fails loudly instead of silently producing free text."""
    if not json_schema.strip():
        return None
    schema = json.loads(json_schema)
    if not isinstance(schema, dict):
        raise ValueError("json_schema must be an object")
    return schema


@dataclass
class OllamaLLM:
    """POST /api/chat with stream=false. ``think`` is sent as false so
    reasoning models answer directly; older servers ignore the field.
    """

    base_url: str
    model: str
    timeout_seconds: int = 600
    # Context window requested per call. Ollama's server default (4096
    # on current releases, 2048 on older) silently truncates longer
    # prompts, which turned a 15k-token judgment prompt into a 2k one in
    # prod and produced confident, wrong verdicts. Always set it, and
    # check the reported prompt_eval_count against the prompt size.
    num_ctx: int = 16384
    # Bearer token for a hosted Ollama endpoint (ollama.com). Empty for a
    # local or tailnet server.
    api_key: str = ""
    name: str = "ollama"

    def __post_init__(self) -> None:
        self.name = f"ollama:{self.model}"

    def generate(
        self,
        system: str,
        user: str,
        max_tokens: int = 0,
        temperature: float = 0.2,
        json_mode: bool = False,
        json_schema: str = "",
    ) -> GenerateResult:
        messages = []
        if system:
            messages.append({"role": "system", "content": system})
        messages.append({"role": "user", "content": user})
        body: dict = {
            "model": self.model,
            "messages": messages,
            "stream": False,
            "think": False,
            "options": {"temperature": temperature, "num_ctx": self.num_ctx},
        }
        if max_tokens > 0:
            body["options"]["num_predict"] = max_tokens
        # Ollama's `format` takes either "json" or a JSON schema object,
        # in which case decoding is constrained to the schema.
        schema = parse_schema(json_schema)
        if schema is not None:
            body["format"] = schema
        elif json_mode:
            body["format"] = "json"
        headers = {"Content-Type": "application/json"}
        if self.api_key:
            headers["Authorization"] = f"Bearer {self.api_key}"
        req = urllib.request.Request(
            f"{self.base_url.rstrip('/')}/api/chat",
            data=json.dumps(body).encode(),
            headers=headers,
            method="POST",
        )
        try:
            with urllib.request.urlopen(req, timeout=self.timeout_seconds) as resp:
                payload = json.loads(resp.read().decode("utf-8"))
        except urllib.error.HTTPError as e:
            detail = e.read(300).decode("utf-8", "replace") if e.fp else ""
            raise RuntimeError(f"ollama generate failed: HTTP {e.code} {detail}".strip()) from e
        except urllib.error.URLError as e:
            raise RuntimeError(f"ollama generate failed: {e}") from e
        content = (payload.get("message") or {}).get("content", "")
        if not isinstance(content, str):
            raise RuntimeError("ollama returned no message content")
        prompt_tokens = int(payload.get("prompt_eval_count") or 0)
        check_truncation(
            prompt_tokens, estimate_tokens(system) + estimate_tokens(user), self.num_ctx
        )
        return GenerateResult(
            text=strip_thinking(content),
            model=self.name,
            prompt_tokens=prompt_tokens,
            completion_tokens=int(payload.get("eval_count") or 0),
            finish_reason=str(payload.get("done_reason") or ""),
        )

    # Deliberately short. A health check that can block for ten minutes
    # is a second outage, and the question here is only whether the
    # server answers and lists the model.
    probe_timeout_seconds: int = 5

    def probe(self) -> tuple[bool, str]:
        """Ask Ollama for its tag list and look for the configured model.

        Chosen over a one-token generation because it is cheap enough to
        run on every health call and still distinguishes the three ways
        this fails: the server being down, the model never having been
        pulled, and the model having been deleted. A generation would
        also prove the model loads, but loading a 4b model into memory
        on each health check would make the check the problem.

        Tags are reported as "name:tag". A model configured without an
        explicit tag matches its ":latest" entry, which is how Ollama
        itself resolves it.
        """
        url = f"{self.base_url.rstrip('/')}/api/tags"
        req = urllib.request.Request(url, method="GET")
        if self.api_key:
            req.add_header("Authorization", f"Bearer {self.api_key}")
        try:
            with urllib.request.urlopen(req, timeout=self.probe_timeout_seconds) as resp:
                payload = json.loads(resp.read().decode("utf-8"))
        except urllib.error.HTTPError as e:
            return False, f"{url} returned HTTP {e.code}"
        except urllib.error.URLError as e:
            return False, f"{url} is unreachable: {e.reason}"
        except Exception as e:  # noqa: BLE001 - a health check never raises
            return False, f"{url} failed: {type(e).__name__}: {e}"

        names = {str(m.get("name", "")) for m in (payload.get("models") or [])}
        wanted = self.model if ":" in self.model else f"{self.model}:latest"
        if wanted in names or self.model in names:
            return True, ""
        listed = ", ".join(sorted(n for n in names if n)) or "none"
        return False, f"model {self.model!r} is not served by {url} (available: {listed})"


def estimate_tokens(text: str) -> int:
    """Rough token estimate for English prose and markup (about 3.6
    characters per token for these prompts)."""
    return int(len(text) / 3.6)


def check_truncation(prompt_tokens: int, estimated: int, num_ctx: int) -> None:
    """Raise when the server evidently dropped part of the prompt. Ollama
    truncates to the context window without an error; a judgment made on
    a partial prompt is worse than no judgment, so refuse it."""
    if prompt_tokens <= 0 or estimated < 512:
        return
    if prompt_tokens < 0.6 * estimated:
        raise RuntimeError(
            f"prompt truncated by the model server: {prompt_tokens} tokens evaluated of "
            f"about {estimated} sent (num_ctx={num_ctx}); raise SIDECAR_LLM_NUM_CTX or "
            "shorten the prompt"
        )


@dataclass
class StubLLM:
    """Deterministic completion for CI and the default dev stack. Echoes
    a short markdown document that mentions the first line of the user
    turn so callers can assert the round-trip without a model.
    """

    name: str = "stub"

    def probe(self) -> tuple[bool, str]:
        """Always serviceable: the stub is the implementation, so there
        is nothing for it to be unable to reach."""
        return True, ""

    def generate(
        self,
        system: str,
        user: str,
        max_tokens: int = 0,
        temperature: float = 0.2,
        json_mode: bool = False,
        json_schema: str = "",
    ) -> GenerateResult:
        first_line = user.strip().splitlines()[0][:80] if user.strip() else ""
        schema = parse_schema(json_schema)
        if schema is not None:
            text = json.dumps(_stub_instance(schema))
        elif json_mode:
            text = json.dumps({"stub": True, "echo": first_line})
        else:
            text = (
                "# Stub completion\n\n"
                f"Provider stub received a {len(user)} character user turn "
                f"beginning: {first_line}\n"
            )
        return GenerateResult(
            text=text,
            model=self.name,
            prompt_tokens=len(system.split()) + len(user.split()),
            completion_tokens=len(text.split()),
            finish_reason="stop",
        )


def _stub_instance(schema: dict):
    """Build a minimal instance satisfying a simple JSON schema (object /
    array / string / number / integer / boolean / enum, `required` only),
    so the stub provider produces schema-valid output and structured
    pipelines run end to end in CI without a model."""
    if "enum" in schema and schema["enum"]:
        return schema["enum"][0]
    t = schema.get("type")
    if isinstance(t, list):
        t = t[0] if t else "string"
    if t == "object":
        props = schema.get("properties", {})
        required = schema.get("required", list(props.keys()))
        return {k: _stub_instance(props[k]) for k in required if k in props}
    if t == "array":
        items = schema.get("items")
        n = int(schema.get("minItems", 1) or 1)
        return [_stub_instance(items) for _ in range(n)] if isinstance(items, dict) else []
    if t == "integer":
        return int(schema.get("minimum", 1))
    if t == "number":
        return float(schema.get("minimum", 1))
    if t == "boolean":
        return True
    return "stub"


def build_llm(
    provider: str,
    ollama_url: str,
    model: str,
    timeout_seconds: int,
    num_ctx: int = 16384,
    api_key: str = "",
) -> LLM:
    """Construct the configured LLM provider or raise. Called once at boot."""
    if provider == "ollama":
        return OllamaLLM(
            base_url=ollama_url,
            model=model,
            timeout_seconds=timeout_seconds,
            num_ctx=num_ctx,
            api_key=api_key,
        )
    if provider == "stub":
        return StubLLM()
    raise RuntimeError(f"unknown SIDECAR_LLM_PROVIDER: {provider!r} (want 'ollama' or 'stub')")
