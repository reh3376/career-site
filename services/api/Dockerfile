# syntax=docker/dockerfile:1.7

FROM golang:1.26-alpine AS build
WORKDIR /src

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=bind,source=go.mod,target=go.mod \
    --mount=type=bind,source=go.sum,target=go.sum \
    go mod download

COPY . .

ARG VERSION=0.0.0-dev
ARG COMMIT=unknown
ARG BUILT_AT=unknown

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux \
    go build -trimpath \
      -ldflags "-s -w \
        -X github.com/reh3376/career-site/services/api/internal/build.Version=${VERSION} \
        -X github.com/reh3376/career-site/services/api/internal/build.Commit=${COMMIT} \
        -X github.com/reh3376/career-site/services/api/internal/build.BuiltAt=${BUILT_AT}" \
      -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /api
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/api"]
