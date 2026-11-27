# syntax=docker/dockerfile:1.7

FROM python:3.14-slim-bookworm AS build
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

FROM python:3.14-slim-bookworm
ENV PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1

# The decision test report is set in Liberation Sans, which is
# metric-compatible with Arial: a report lays out here exactly as it
# does on the workstation it was designed on. Installed from Debian so
# no font binary lives in the repository. Without it the renderer falls
# back to DejaVu Sans, which is wider and moves every line break; see
# report/fonts.py.
RUN apt-get update \
    && apt-get install -y --no-install-recommends fonts-liberation \
    && rm -rf /var/lib/apt/lists/*

RUN useradd --system --create-home --uid 65532 sidecar
USER sidecar
WORKDIR /home/sidecar

COPY --from=build --chown=sidecar:sidecar /src/.venv /home/sidecar/.venv
COPY --from=build --chown=sidecar:sidecar /src/src /home/sidecar/src

ENV PATH="/home/sidecar/.venv/bin:${PATH}" \
    PYTHONPATH="/home/sidecar/src"

EXPOSE 50051
ENTRYPOINT ["python", "-m", "career_sidecar"]
