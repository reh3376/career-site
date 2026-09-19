# syntax=docker/dockerfile:1.7

FROM python:3.12-slim-bookworm AS build
ENV PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1

COPY --from=ghcr.io/astral-sh/uv:0.9 /uv /uvx /usr/local/bin/

WORKDIR /src

# Install deps first (cached layer). --no-install-project keeps the layer
# invalidated only by pyproject.toml / uv.lock, not by source edits.
COPY pyproject.toml uv.lock README.md ./
RUN --mount=type=cache,target=/root/.cache/uv \
    uv sync --frozen --no-install-project --no-dev

COPY src/ ./src/
RUN --mount=type=cache,target=/root/.cache/uv \
    uv sync --frozen --no-dev

FROM python:3.12-slim-bookworm
ENV PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1

RUN useradd --system --create-home --uid 65532 sidecar
USER sidecar
WORKDIR /home/sidecar

COPY --from=build --chown=sidecar:sidecar /src/.venv /home/sidecar/.venv
COPY --from=build --chown=sidecar:sidecar /src/src /home/sidecar/src

ENV PATH="/home/sidecar/.venv/bin:${PATH}" \
    PYTHONPATH="/home/sidecar/src"

EXPOSE 50051
ENTRYPOINT ["python", "-m", "career_sidecar"]
