#!/usr/bin/env python3
"""Generate the API reference from the compiled Protobuf contracts.

Reads a buf image (a FileDescriptorSet with source info) and writes:

  docs/api/README.md       human-readable reference for frontend and API work
  docs/api/endpoints.json  machine-readable index (used by the UATS scaffolder
                           and by tooling that needs paths and auth levels)

Everything in the output — descriptions, auth levels, rate limits, validation
rules, JSON field names — comes from the .proto files, so the reference cannot
drift from the contracts. Run through `make docs-api`:

  buf build proto -o build/api.binpb
  uv run --with protobuf python scripts/gen_api_docs.py build/api.binpb

CI regenerates and fails if the committed output differs.
"""

from __future__ import annotations

import json
import sys
from collections import OrderedDict
from pathlib import Path

from google.protobuf import descriptor_pb2, descriptor_pool, text_format

REPO = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(REPO / "services" / "sidecar" / "src"))

# Registers the career.v1 method options and buf.validate field rules on the
# default descriptor pool so they parse from the image.
from career.v1 import options_pb2  # noqa: E402
from buf.validate import validate_pb2  # noqa: E402,F401

VALIDATE_FIELD_EXT = descriptor_pool.Default().FindExtensionByName("buf.validate.field")

PUBLIC_PACKAGE = "career.v1"
INTERNAL_PACKAGE = "career.sidecar.v1"
BASE_PATH = "/api"

SERVICE_ORDER = [
    "AuthService",
    "MemberService",
    "ContentService",
    "HomeService",
    "ActivityService",
    "ChatService",
    "DownloadService",
    "ContactService",
    "AdminService",
    "SystemService",
]

SCALARS = {
    descriptor_pb2.FieldDescriptorProto.TYPE_STRING: ("string", "string"),
    descriptor_pb2.FieldDescriptorProto.TYPE_BOOL: ("bool", "boolean"),
    descriptor_pb2.FieldDescriptorProto.TYPE_INT32: ("int32", "number"),
    descriptor_pb2.FieldDescriptorProto.TYPE_UINT32: ("uint32", "number"),
    descriptor_pb2.FieldDescriptorProto.TYPE_SINT32: ("sint32", "number"),
    descriptor_pb2.FieldDescriptorProto.TYPE_FIXED32: ("fixed32", "number"),
    descriptor_pb2.FieldDescriptorProto.TYPE_SFIXED32: ("sfixed32", "number"),
    descriptor_pb2.FieldDescriptorProto.TYPE_INT64: ("int64", "string (decimal)"),
    descriptor_pb2.FieldDescriptorProto.TYPE_UINT64: ("uint64", "string (decimal)"),
    descriptor_pb2.FieldDescriptorProto.TYPE_SINT64: ("sint64", "string (decimal)"),
    descriptor_pb2.FieldDescriptorProto.TYPE_FIXED64: ("fixed64", "string (decimal)"),
    descriptor_pb2.FieldDescriptorProto.TYPE_SFIXED64: ("sfixed64", "string (decimal)"),
    descriptor_pb2.FieldDescriptorProto.TYPE_FLOAT: ("float", "number"),
    descriptor_pb2.FieldDescriptorProto.TYPE_DOUBLE: ("double", "number"),
    descriptor_pb2.FieldDescriptorProto.TYPE_BYTES: ("bytes", "string (base64)"),
}

WELL_KNOWN = {
    ".google.protobuf.Timestamp": ("Timestamp", "string (RFC 3339, UTC)", '"2026-09-18T12:00:00Z"'),
    ".google.protobuf.FieldMask": ("FieldMask", "string (comma-separated paths)", '"name,organization"'),
    ".google.protobuf.Duration": ("Duration", "string (e.g. \"1.5s\")", '"1.5s"'),
    ".google.protobuf.Empty": ("Empty", "object", "{}"),
}

CONNECT_HTTP_STATUS = [
    ("invalid_argument", 400, "Request failed validation; `details` carries `buf.validate.Violations` listing each field"),
    ("unauthenticated", 401, "No session, expired session, or unverified member calling a verified-only method"),
    ("permission_denied", 403, "Session lacks the role, or admin MFA is not fresh"),
    ("not_found", 404, "Unknown conversation, content item, export, or job"),
    ("already_exists", 409, "Duplicate where duplicates are rejected"),
    ("failed_precondition", 412, "State does not allow the call (e.g. `turnstile_required`, questionnaire pending)"),
    ("resource_exhausted", 429, "Rate limit or assistant quota exceeded; honor `Retry-After`"),
    ("unavailable", 503, "A dependency is down; retry with backoff"),
    ("deadline_exceeded", 408, "Server-side deadline exceeded"),
    ("unimplemented", 404, "Method reserved for a later phase"),
    ("internal", 500, "Bug; the response `message` carries a request ID to quote"),
]


class Comments:
    """Leading comments indexed by descriptor path."""

    def __init__(self, file_proto: descriptor_pb2.FileDescriptorProto):
        self.by_path: dict[tuple[int, ...], str] = {}
        for loc in file_proto.source_code_info.location:
            if loc.leading_comments:
                self.by_path[tuple(loc.path)] = _clean(loc.leading_comments)
        self.file_comment = ""
        # The file-level comment is attached to the `syntax` statement (path [12]).
        for loc in file_proto.source_code_info.location:
            if tuple(loc.path) == (12,) and loc.leading_detached_comments:
                self.file_comment = _clean("".join(loc.leading_detached_comments))

    def get(self, *path: int) -> str:
        return self.by_path.get(tuple(path), "")


def _clean(comment: str) -> str:
    lines = [line[1:] if line.startswith(" ") else line for line in comment.rstrip().splitlines()]
    return "\n".join(lines).strip()


def first_sentence(text: str) -> str:
    text = " ".join(text.split())
    for end in (". ", ".\n"):
        idx = text.find(end)
        if idx != -1:
            return text[: idx + 1]
    return text


def anchor(name: str) -> str:
    return name.lower().replace(".", "").replace(" ", "-")


class Model:
    """Everything the renderer needs, collected from the image."""

    def __init__(self, fds: descriptor_pb2.FileDescriptorSet):
        self.messages: "OrderedDict[str, tuple[descriptor_pb2.DescriptorProto, Comments, tuple[int, ...]]]" = OrderedDict()
        self.enums: "OrderedDict[str, tuple[descriptor_pb2.EnumDescriptorProto, Comments, tuple[int, ...]]]" = OrderedDict()
        self.services: list[tuple[str, descriptor_pb2.ServiceDescriptorProto, Comments, int]] = []
        self.file_comments: dict[str, str] = {}
        for fp in fds.file:
            if not fp.package.startswith("career."):
                continue
            comments = Comments(fp)
            self.file_comments[fp.name] = comments.file_comment
            for i, msg in enumerate(fp.message_type):
                self._collect_message(fp.package, msg, comments, (4, i), prefix="")
            for i, enum in enumerate(fp.enum_type):
                self.enums[f".{fp.package}.{enum.name}"] = (enum, comments, (5, i))
            for i, svc in enumerate(fp.service):
                self.services.append((fp.package, svc, comments, i))

    def _collect_message(self, package, msg, comments, path, prefix):
        full = f".{package}.{prefix}{msg.name}"
        self.messages[full] = (msg, comments, path)
        for j, nested in enumerate(msg.nested_type):
            if nested.options.map_entry:
                self.messages[f"{full}.{nested.name}"] = (nested, comments, path + (3, j))
                continue
            self._collect_message(package, nested, comments, path + (3, j), prefix=f"{prefix}{msg.name}.")
        for j, enum in enumerate(msg.enum_type):
            self.enums[f"{full}.{enum.name}"] = (enum, comments, path + (4, j))

    def short(self, full: str) -> str:
        for pkg in (f".{PUBLIC_PACKAGE}.", f".{INTERNAL_PACKAGE}."):
            if full.startswith(pkg):
                return full[len(pkg):]
        return full.lstrip(".")


def field_type(model: Model, field: descriptor_pb2.FieldDescriptorProto) -> tuple[str, str]:
    """Returns (display type with link, JSON encoding note)."""
    repeated = field.label == descriptor_pb2.FieldDescriptorProto.LABEL_REPEATED
    if field.type == descriptor_pb2.FieldDescriptorProto.TYPE_MESSAGE:
        if field.type_name in WELL_KNOWN:
            name, js, _ = WELL_KNOWN[field.type_name]
            disp = f"`{name}`"
        else:
            entry = model.messages.get(field.type_name)
            if entry and entry[0].options.map_entry:
                k = entry[0].field[0]
                v = entry[0].field[1]
                kt, _ = field_type(model, k)
                vt, vjs = field_type(model, v)
                return f"map<{kt}, {vt}>", f"object of {vjs}"
            short = model.short(field.type_name)
            disp = f"[`{short}`](#{anchor(short)})"
            js = "object"
    elif field.type == descriptor_pb2.FieldDescriptorProto.TYPE_ENUM:
        short = model.short(field.type_name)
        disp = f"[`{short}`](#{anchor(short)})"
        js = "string (enum name)"
    else:
        name, js = SCALARS[field.type]
        disp = f"`{name}`"
    if repeated:
        return f"{disp}[]", f"array of {js}"
    return disp, js


def rules_text(field: descriptor_pb2.FieldDescriptorProto) -> str:
    if not field.options.HasExtension(VALIDATE_FIELD_EXT):
        return ""
    rules = field.options.Extensions[VALIDATE_FIELD_EXT]
    text = text_format.MessageToString(rules, as_one_line=True)
    # "string { min_len: 12 max_len: 256 }" -> "string: min_len 12, max_len 256"
    text = text.replace(" {", ":").replace(" }", "").replace("}", "")
    return f"`{text.strip()}`"


def example_value(model: Model, field: descriptor_pb2.FieldDescriptorProto, depth: int):
    repeated = field.label == descriptor_pb2.FieldDescriptorProto.LABEL_REPEATED
    if field.type == descriptor_pb2.FieldDescriptorProto.TYPE_MESSAGE:
        if field.type_name in WELL_KNOWN:
            val = json.loads(WELL_KNOWN[field.type_name][2])
        else:
            entry = model.messages.get(field.type_name)
            if entry and entry[0].options.map_entry:
                val = {"key": example_value(model, entry[0].field[1], depth + 1)}
                return val
            val = example_message(model, field.type_name, depth + 1) if depth < 2 else {}
    elif field.type == descriptor_pb2.FieldDescriptorProto.TYPE_ENUM:
        enum = model.enums[field.type_name][0]
        val = enum.value[1].name if len(enum.value) > 1 else enum.value[0].name
    elif field.type == descriptor_pb2.FieldDescriptorProto.TYPE_STRING:
        val = "string"
    elif field.type == descriptor_pb2.FieldDescriptorProto.TYPE_BOOL:
        val = True
    elif field.type in (
        descriptor_pb2.FieldDescriptorProto.TYPE_FLOAT,
        descriptor_pb2.FieldDescriptorProto.TYPE_DOUBLE,
    ):
        val = 0.5
    elif field.type == descriptor_pb2.FieldDescriptorProto.TYPE_BYTES:
        val = "base64"
    elif field.type in (
        descriptor_pb2.FieldDescriptorProto.TYPE_INT64,
        descriptor_pb2.FieldDescriptorProto.TYPE_UINT64,
        descriptor_pb2.FieldDescriptorProto.TYPE_SINT64,
        descriptor_pb2.FieldDescriptorProto.TYPE_FIXED64,
        descriptor_pb2.FieldDescriptorProto.TYPE_SFIXED64,
    ):
        val = "0"
    else:
        val = 0
    return [val] if repeated else val


def example_message(model: Model, full: str, depth: int = 0) -> dict:
    msg = model.messages[full][0]
    out: dict = {}
    seen_oneofs: set[int] = set()
    for field in msg.field:
        if field.HasField("oneof_index"):
            if field.oneof_index in seen_oneofs:
                continue
            seen_oneofs.add(field.oneof_index)
        out[field.json_name] = example_value(model, field, depth)
    return out


def method_meta(method: descriptor_pb2.MethodDescriptorProto) -> dict:
    opts = method.options
    level = opts.Extensions[options_pb2.auth] if opts.HasExtension(options_pb2.auth) else 0
    return {
        "auth": options_pb2.AuthLevel.Name(level).replace("AUTH_LEVEL_", "").lower() if level else "unspecified",
        "allow_unverified": bool(opts.Extensions[options_pb2.allow_unverified]) if opts.HasExtension(options_pb2.allow_unverified) else False,
        "rate_limit_per_minute": int(opts.Extensions[options_pb2.rate_limit_per_minute]) if opts.HasExtension(options_pb2.rate_limit_per_minute) else 0,
        "mfa_fresh": bool(opts.Extensions[options_pb2.mfa_fresh]) if opts.HasExtension(options_pb2.mfa_fresh) else False,
        "server_streaming": method.server_streaming,
        "client_streaming": method.client_streaming,
    }


def auth_label(meta: dict) -> str:
    label = {"public": "Public", "member": "Member", "admin": "Admin"}.get(meta["auth"], "Unspecified")
    extras = []
    if meta["allow_unverified"]:
        extras.append("unverified OK")
    if meta["mfa_fresh"]:
        extras.append("fresh MFA")
    return label + (f" ({', '.join(extras)})" if extras else "")


# ----------------------------------------------------------------------------- rendering


def render_fields(model: Model, full: str, out: list[str], heading: str) -> None:
    msg, comments, path = model.messages[full]
    out.append(f"**{heading}** — [`{model.short(full)}`](#{anchor(model.short(full))})")
    out.append("")
    if not msg.field:
        out.append("_No fields; send `{}`._")
        out.append("")
        return
    out.append("| Field (JSON) | Type | JSON encoding | Rules | Description |")
    out.append("|---|---|---|---|---|")
    for j, field in enumerate(msg.field):
        disp, js = field_type(model, field)
        desc = " ".join(comments.get(*path, 2, j).split())
        if field.HasField("oneof_index"):
            oneof = msg.oneof_decl[field.oneof_index].name
            desc = f"_(oneof `{oneof}`)_ {desc}"
        out.append(f"| `{field.json_name}` | {disp} | {js} | {rules_text(field)} | {desc} |")
    out.append("")


def render_service(model: Model, package: str, svc, comments: Comments, index: int, out: list[str], endpoints: list[dict], internal: bool) -> None:
    out.append(f"## {svc.name}")
    out.append("")
    svc_comment = comments.get(6, index)
    if svc_comment:
        out.append(svc_comment)
        out.append("")
    if internal:
        out.append("> **Internal.** Served over gRPC on the Compose network only and never routed by the reverse proxy. Documented here because the API's request path depends on it.")
        out.append("")
    out.append("| Method | Path | Auth | Rate limit /min | Request → Response | Summary |")
    out.append("|---|---|---|---|---|---|")
    for j, method in enumerate(svc.method):
        meta = method_meta(method)
        path = f"{BASE_PATH}/{package}.{svc.name}/{method.name}" if not internal else f"/{package}.{svc.name}/{method.name} (gRPC)"
        stream = " (server-streaming)" if meta["server_streaming"] else ""
        rate = str(meta["rate_limit_per_minute"]) if meta["rate_limit_per_minute"] else "default"
        summary = first_sentence(comments.get(6, index, 2, j))
        req, res = model.short(method.input_type), model.short(method.output_type)
        out.append(
            f"| [`{method.name}`](#{anchor(svc.name + '-' + method.name)}) | `{path}` | {auth_label(meta) if not internal else '—'} | {rate if not internal else '—'} | `{req}` → `{res}`{stream} | {summary} |"
        )
        endpoints.append(
            {
                "service": f"{package}.{svc.name}",
                "method": method.name,
                "path": path if not internal else None,
                "grpc_full_method": f"/{package}.{svc.name}/{method.name}",
                "internal": internal,
                "request": method.input_type.lstrip("."),
                "response": method.output_type.lstrip("."),
                "summary": summary,
                **meta,
            }
        )
    out.append("")
    for j, method in enumerate(svc.method):
        meta = method_meta(method)
        out.append(f"### {svc.name}.{method.name}")
        out.append("")
        if not internal:
            out.append(f"`POST {BASE_PATH}/{package}.{svc.name}/{method.name}` · **Auth:** {auth_label(meta)} · **Rate limit:** {meta['rate_limit_per_minute'] or 'default'}/min" + (" · **Server-streaming**" if meta["server_streaming"] else ""))
        else:
            out.append(f"`/{package}.{svc.name}/{method.name}` (gRPC{', server-streaming' if meta['server_streaming'] else ''})")
        out.append("")
        desc = comments.get(6, index, 2, j)
        if desc:
            out.append(desc)
            out.append("")
        render_fields(model, method.input_type, out, "Request")
        render_fields(model, method.output_type, out, "Response" + (" (one event per stream message)" if meta["server_streaming"] else ""))
        example = json.dumps(example_message(model, method.input_type), indent=2)
        out.append("<details><summary>Example request body</summary>")
        out.append("")
        out.append("```json")
        out.append(example)
        out.append("```")
        out.append("")
        out.append("</details>")
        out.append("")


def render_types(model: Model, out: list[str]) -> None:
    out.append("## Types")
    out.append("")
    out.append("Every message and enum, in declaration order. Request and response messages already shown above are included so links resolve.")
    out.append("")
    for full, (msg, comments, path) in model.messages.items():
        if msg.options.map_entry:
            continue
        short = model.short(full)
        out.append(f"### {short}")
        out.append("")
        desc = comments.get(*path)
        if desc:
            out.append(desc)
            out.append("")
        if not msg.field:
            out.append("_No fields._")
            out.append("")
            continue
        out.append("| Field (JSON) | Type | JSON encoding | Rules | Description |")
        out.append("|---|---|---|---|---|")
        for j, field in enumerate(msg.field):
            disp, js = field_type(model, field)
            fdesc = " ".join(comments.get(*path, 2, j).split())
            if field.HasField("oneof_index"):
                fdesc = f"_(oneof `{msg.oneof_decl[field.oneof_index].name}`)_ {fdesc}"
            out.append(f"| `{field.json_name}` | {disp} | {js} | {rules_text(field)} | {fdesc} |")
        out.append("")
    for full, (enum, comments, path) in model.enums.items():
        short = model.short(full)
        out.append(f"### {short}")
        out.append("")
        desc = comments.get(*path)
        if desc:
            out.append(desc)
            out.append("")
        out.append("| Value | Number | Description |")
        out.append("|---|---|---|")
        for j, value in enumerate(enum.value):
            vdesc = " ".join(comments.get(*path, 2, j).split())
            out.append(f"| `{value.name}` | {value.number} | {vdesc} |")
        out.append("")


PREAMBLE = """# career-site API reference

> Generated from `proto/` by `scripts/gen_api_docs.py`. Do not edit by hand — change the `.proto` files and run `make docs-api`. CI fails when this file drifts from the contracts.

The site's API is a set of [ConnectRPC](https://connectrpc.com) services defined in Protobuf under `proto/career/v1/`. The Go server implements them; the Next.js app calls them through the generated TypeScript client in `apps/web/src/gen/`. The same definitions drive request validation, the auth interceptor, this reference, and the UATS contract specs, so the contract is declared once.

## Conventions

### Transport

| Aspect | Rule |
|---|---|
| Base path | `/api` (same origin as the site; Caddy routes it to the Go API) |
| Method URL | `POST /api/{package}.{Service}/{Method}`, e.g. `POST /api/career.v1.AuthService/Login` |
| Encoding | JSON (`Content-Type: application/json`); the TypeScript client can switch to binary Protobuf with `useBinaryFormat: true` without any server change |
| Required header | `Connect-Protocol-Version: 1` — the generated clients add it; `curl` must add it by hand. Being a non-simple header it also blocks cross-site request forgery, since a cross-origin page cannot send it without a CORS preflight that the server refuses |
| Streaming | Server-streaming methods (`ChatService.SendMessage`) return a stream of enveloped messages over the same `POST`; the TypeScript client exposes it as an `AsyncIterable`, so the UI never handles framing |
| Timeouts | Clients may send `Connect-Timeout-Ms`; the server caps unary calls at 30 s and the chat stream at 120 s |
| Versioning | The package is `career.v1`; additive changes (new fields, methods, enum values) do not bump it; `buf breaking` in CI blocks incompatible changes |

### JSON encoding of Protobuf

| Protobuf | JSON | Notes |
|---|---|---|
| field names | lowerCamelCase | `stated_role` → `statedRole`; the tables below show the JSON names |
| `int64`, `uint64` | string | e.g. `"sizeBytes": "123456"` |
| `float`, `double`, `int32` | number | |
| enum | string name | e.g. `"type": "CONTENT_TYPE_PROJECT"`; unknown names are rejected |
| `google.protobuf.Timestamp` | RFC 3339 string, UTC | `"2026-09-18T12:00:00Z"` |
| `google.protobuf.FieldMask` | comma-separated paths | `"updateMask": "name,profile.seniority"` |
| `bytes` | base64 string | |
| `oneof` | at most one member present | sending two is `invalid_argument` |
| unset / default values | omitted from responses | clients must treat a missing field as the default (`""`, `0`, `false`, `[]`) |

### Authentication and sessions

- Sessions live in the `career_session` cookie (`HttpOnly; Secure; SameSite=Lax`; 30-day absolute lifetime, 7-day idle timeout). The browser sends it automatically on same-origin requests; the client never handles a token.
- **Auth levels** (declared per method in the `.proto` and enforced by an interceptor before the handler runs):
  - **Public** — no session.
  - **Member** — a session whose member is `ACTIVE`. Methods marked *unverified OK* also accept a session whose member is `UNVERIFIED` (verification, resend, sign-out, deletion).
  - **Admin** — a session with role `ADMIN`; methods marked *fresh MFA* additionally require a TOTP verification within the freshness window (default 12 hours), otherwise `permission_denied` with reason `mfa_fresh_required`.
- Sign-in and verification responses set the cookie; `Logout` clears it. Social sign-in is a browser redirect flow (see *Plain HTTP endpoints*).
- Rate limits are per account (per IP for public methods). Responses carry `RateLimit-Limit` and `RateLimit-Remaining`; a `resource_exhausted` error carries `Retry-After`.

### Errors

Errors are Connect errors: a non-200 status and a JSON body `{"code": "...", "message": "...", "details": [...]}`. Branch on `code`, not on the HTTP status. Validation failures carry a `buf.validate.Violations` detail listing every violated field and rule.

| `code` | HTTP | When |
|---|---|---|
"""

PREAMBLE_TAIL = """
### Plain HTTP endpoints

Browsers navigate to these, so they are ordinary `GET` handlers rather than RPCs.

| Endpoint | Auth | Behavior |
|---|---|---|
| `GET /api/healthz` | Public | `200 {"status":"ok"}` when the process is up |
| `GET /api/readyz` | Public | `200 {"status":"ok","checks":{...},"version":"..."}` when database, object storage, and sidecar are reachable; `503` with the failing checks otherwise |
| `GET /api/auth/oauth/{provider}/start` | Public | Redirects to LinkedIn (v1; `github` and `google` later) with PKCE and `state` |
| `GET /api/auth/oauth/{provider}/callback` | Public | Completes the exchange, creates or links the account by verified email, sets the session cookie, redirects to `/me/interests` or `/home` |
| `GET /api/downloads/{variant}` | Member | Records the download and `302`s to a short-lived signed URL; `401` without a session, `404` for an unknown variant |

### TypeScript quick start

```ts
// apps/web: one transport for the whole app
import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { AuthService } from "@/gen/career/v1/auth_pb";
import { ChatService } from "@/gen/career/v1/chat_pb";

const transport = createConnectTransport({ baseUrl: "/api" }); // same origin; cookie sent automatically

const auth = createClient(AuthService, transport);
const { me, questionnairePending } = await auth.login({ email, password });

// Streaming chat: iterate the events as they arrive
const chat = createClient(ChatService, transport);
let text = "";
for await (const res of chat.sendMessage({ conversationId, text: question })) {
  switch (res.event.case) {
    case "start":     /* res.event.value.assistantMessageId */ break;
    case "delta":     text += res.event.value.text; break;
    case "citations": /* res.event.value.citations */ break;
    case "usage":     /* res.event.value.remainingToday */ break;
    case "done":      /* res.event.value.message — the stored message */ break;
  }
}
```

Errors surface as `ConnectError` with `.code` (a `Code` enum) and `.findDetails(ViolationsSchema)` for validation details. Server components can call the same client with the incoming request's cookie forwarded in a custom fetch.

### curl

```sh
curl -sS -X POST https://<host>/api/career.v1.SystemService/GetVersion \\
  -H 'Content-Type: application/json' -H 'Connect-Protocol-Version: 1' -d '{}'
```

---

"""


def main(image_path: str) -> None:
    fds = descriptor_pb2.FileDescriptorSet()
    fds.ParseFromString(Path(image_path).read_bytes())
    model = Model(fds)

    out: list[str] = [PREAMBLE.rstrip("\n")]
    for code, status, when in CONNECT_HTTP_STATUS:
        out.append(f"| `{code}` | {status} | {when} |")
    out.append(PREAMBLE_TAIL.rstrip("\n"))
    out.append("")

    # Service index
    public = sorted(
        [s for s in model.services if s[0] == PUBLIC_PACKAGE],
        key=lambda s: SERVICE_ORDER.index(s[1].name) if s[1].name in SERVICE_ORDER else 99,
    )
    internal = [s for s in model.services if s[0] == INTERNAL_PACKAGE]
    out.append("## Services")
    out.append("")
    out.append("| Service | Purpose | Methods |")
    out.append("|---|---|---|")
    for package, svc, comments, index in public + internal:
        purpose = first_sentence(comments.get(6, index))
        out.append(f"| [`{svc.name}`](#{anchor(svc.name)}) | {purpose}{' _(internal)_' if package == INTERNAL_PACKAGE else ''} | {len(svc.method)} |")
    out.append("")

    endpoints: list[dict] = []
    for package, svc, comments, index in public:
        render_service(model, package, svc, comments, index, out, endpoints, internal=False)
    for package, svc, comments, index in internal:
        render_service(model, package, svc, comments, index, out, endpoints, internal=True)
    render_types(model, out)

    docs_dir = REPO / "docs" / "api"
    docs_dir.mkdir(parents=True, exist_ok=True)
    (docs_dir / "README.md").write_text("\n".join(out).rstrip("\n") + "\n", encoding="utf-8")
    (docs_dir / "endpoints.json").write_text(
        json.dumps({"base_path": BASE_PATH, "endpoints": endpoints}, indent=2) + "\n", encoding="utf-8"
    )
    public_methods = sum(len(s[1].method) for s in public)
    internal_methods = sum(len(s[1].method) for s in internal)
    print(f"docs/api/README.md: {len(public)} public services / {public_methods} methods, {len(internal)} internal / {internal_methods} methods, {len(model.messages)} messages, {len(model.enums)} enums")


if __name__ == "__main__":
    if len(sys.argv) != 2:
        sys.exit("usage: gen_api_docs.py <buf image .binpb>")
    main(sys.argv[1])
