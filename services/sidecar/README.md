# services/sidecar

Python gRPC sidecar for career-site. Owns embedding, reranking, classification, and batch jobs — everything where Python's libraries are decisive and latency is not on the request-critical path (per [ADR 0004](../../docs/adr/0004-go-api-with-python-sidecar.md)).

## Layout

```
services/sidecar/
├── pyproject.toml
├── src/
│   ├── career_sidecar/
│   │   ├── server.py         gRPC server, thread pool, graceful stop
│   │   ├── servicer.py       the RPC surface
│   │   ├── config.py         env-driven configuration
│   │   ├── embed.py          embedding providers (ollama, stub)
│   │   ├── llm.py            generation providers (ollama, stub)
│   │   ├── render.py         résumé rendering to PDF
│   │   ├── build.py          Typst/template assembly
│   │   └── templates/        résumé templates
│   ├── career/               generated Protobuf + gRPC stubs (do not edit)
│   └── buf/                  generated buf.validate stubs (do not edit)
├── tests/
└── Dockerfile
```

## Run locally

```bash
uv sync --extra dev
uv run python -m career_sidecar             # binds [::]:50051 by default
SIDECAR_ADDR=[::]:6000 uv run python -m career_sidecar
```

Verify with `grpcurl` (the Go API reaches it at `SIDECAR_ADDR`):

```bash
grpcurl -plaintext localhost:50051 career.sidecar.v1.SidecarService/Health
```

## Test

```bash
uv run pytest
```

## Build

```bash
docker build -t career-site/sidecar:dev .
```

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `SIDECAR_ADDR` | `[::]:50051` | Listen address |
| `SIDECAR_MAX_WORKERS` | `8` | Thread pool size |
| `SIDECAR_SHUTDOWN_GRACE_SECONDS` | `5` | Graceful stop deadline |
| `SIDECAR_VERSION` | `<package metadata>` | Overrides the version reported by `Health` |
| `SIDECAR_EMBED_PROVIDER` | `stub` | `ollama` or `stub` |
| `SIDECAR_EMBED_DIMENSIONS` | `768` | Must match the pgvector column |
| `SIDECAR_LLM_PROVIDER` | `stub` | `ollama` or `stub` |
| `SIDECAR_LLM_TIMEOUT_SECONDS` | `600` | Production runs `3000`; CPU generation is slow |
| `SIDECAR_LLM_NUM_CTX` | `16384` | Production runs `8192` to fit the memory cap |
| `OLLAMA_URL` | `http://ollama:11434` | Embedding endpoint |
| `OLLAMA_LLM_URL` | falls back to `OLLAMA_URL` | Generation endpoint |
| `OLLAMA_EMBED_MODEL` | `nomic-embed-text` | 768-dim, asymmetric prefixes |
| `OLLAMA_LLM_MODEL` | `qwen3:4b-q8_0` | |
| `OLLAMA_API_KEY` | empty | For a hosted endpoint; unused on-box |

Both providers default to `stub` so a checkout runs, and CI tests, without Ollama. Production values are in `deploy/README.md`; the two `NUM_CTX` and timeout defaults above are deliberately *not* the production ones, which is worth knowing before debugging a local timeout.

## The RPC surface

Implemented: `Health`, `Embed`, `Generate`, `RenderResume`.

Still returning `UNIMPLEMENTED`: `Rerank`, `Classify`, `RunJob`, `GetJob`. None is currently blocking anything. Retrieval ranks by cosine similarity in Postgres without a reranking pass, classification happens in the Go API's `jd` package where the scoring rules live, and ingestion runs as an in-process job on the API side rather than as a sidecar job, so `RunJob`/`GetJob` have no caller.

`Generate` is single-shot, not streaming. That is why an Ask Roger answer arrives as one delta rather than token by token: the streaming exists between the browser and the Go API, but the model call underneath it returns complete. Changing that means a server-streaming RPC here first.
