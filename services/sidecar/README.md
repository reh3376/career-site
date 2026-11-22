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
│   │   ├── report/           decision test report: charts.py, layout.py, fonts.py
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

Implemented: `Health`, `Embed`, `Generate`, `RenderResume`, `RenderDecisionTestReport`.

Still returning `UNIMPLEMENTED`: `Rerank`, `Classify`, `RunJob`, `GetJob`. None is currently blocking anything. Retrieval ranks by cosine similarity in Postgres without a reranking pass, classification happens in the Go API's `jd` package where the scoring rules live, and ingestion runs as an in-process job on the API side rather than as a sidecar job, so `RunJob`/`GetJob` have no caller.

`RenderDecisionTestReport` draws the decision test's individual results report (FSD FR-DT-21). The api sends the report as a finished document, a list of blocks: headings, paragraphs, tables and chart payloads, with every figure worked out and every sentence chosen in `services/api/internal/dtreport`. `report/charts.py` draws the charts with matplotlib, `report/layout.py` lays the pages out with reportlab, and the call returns two PDFs: the participant's copy, and the owner's copy, which adds the blocks the document marks `only: owner` (the answer-by-answer table and the questions with their answers). The renderer holds no figure and no wording, so a change to what a report says is a change to the api alone. The document carries a `schema` number and a mismatch is refused, which is what happens for the minute or two of a deploy in which the two services are from different builds; the api retries.

The report is set in Liberation Sans, installed in the image from Debian's `fonts-liberation`. It is metric-compatible with Arial, so a page lays out in the container exactly as on the workstation it was designed on. A machine with neither falls back to the DejaVu Sans that matplotlib carries, and the engine string returned with every render says which face was used (`reportlab:5.1.0+matplotlib:3.11.2+Liberation Sans`).

`Generate` is single-shot, not streaming. That is why an Ask Roger answer arrives as one delta rather than token by token: the streaming exists between the browser and the Go API, but the model call underneath it returns complete. Changing that means a server-streaming RPC here first.
