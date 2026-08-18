#!/usr/bin/env bash
# Idempotent Cloud Agent bootstrap for obsidian-mcp (Go 1.25 module).
# Runs after checkout; safe to run repeatedly against cached state.
set -euo pipefail

cd "$(dirname "$0")/.."

# Fetch modules and the Go toolchain pinned in go.mod, then warm the build cache.
go mod download
go build ./...

# Pin golangci-lint to the version CI uses (.github/workflows/ci.yml) so
# `golangci-lint run ./...` reproduces CI locally. Install into GOPATH/bin,
# which is already on PATH. Skip re-install when the pinned version is present.
GOLANGCI_LINT_VERSION="v2.11.4"
GOBIN="$(go env GOPATH)/bin"
current=""
if command -v golangci-lint >/dev/null 2>&1; then
  current="v$(golangci-lint version 2>/dev/null | grep -oE 'version [0-9]+\.[0-9]+\.[0-9]+' | awk '{print $2}')"
fi
if [ "$current" != "$GOLANGCI_LINT_VERSION" ]; then
  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
    | sh -s -- -b "$GOBIN" "$GOLANGCI_LINT_VERSION"
fi

echo "obsidian-mcp environment ready: $(go version), $(golangci-lint version 2>/dev/null | head -1)"
