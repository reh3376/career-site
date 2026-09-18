# career-site — single entry point for local automation and CI.
# Targets are the portable interface: workflows call `make <target>`, never
# the underlying commands (UxTS convention, FSD §10.4).
#
# Toolchain: Go (current stable), uv, pnpm, buf. `make tools` installs buf.

SHELL := /bin/bash
.DEFAULT_GOAL := help

GOBIN ?= $(shell go env GOPATH)/bin
BUF   ?= $(GOBIN)/buf
UV    ?= uv
BUILD := build

GEN_PATHS := services/api/gen apps/web/src/gen services/sidecar/src/career services/sidecar/src/buf docs/api

.PHONY: help tools gen lint-proto breaking docs-api check-gen clean

help: ## List targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-14s %s\n", $$1, $$2}'

tools: ## Install buf into GOBIN
	go install github.com/bufbuild/buf/cmd/buf@latest

lint-proto: ## Lint the Protobuf contracts (STANDARD + COMMENTS rules)
	cd proto && $(BUF) dep update && $(BUF) lint

breaking: ## Check the contracts for breaking changes against main
	$(BUF) breaking proto --against '.git#branch=main,subdir=proto'

gen: ## Regenerate Go, TypeScript, and Python code from proto/
	$(BUF) generate --template proto/buf.gen.yaml
	$(BUF) generate --template proto/buf.gen.sidecar.yaml

docs-api: ## Regenerate docs/api/README.md and docs/api/endpoints.json from proto/
	mkdir -p $(BUILD)
	$(BUF) build proto -o $(BUILD)/api.binpb
	$(UV) run --python 3.12 --with protobuf python scripts/gen_api_docs.py $(BUILD)/api.binpb

check-gen: gen docs-api ## Fail if committed generated code or docs drift from proto/
	@git diff --exit-code --stat -- $(GEN_PATHS) \
	  || { echo "Generated outputs are stale: run 'make gen docs-api' and commit the result."; exit 1; }
	@echo "Generated outputs are current."

clean: ## Remove build artifacts
	rm -rf $(BUILD)
