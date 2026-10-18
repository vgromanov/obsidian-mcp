#!/usr/bin/env bash
# Idempotent Cloud Agent bootstrap for obsidian-mcp (Go 1.25 module).
# Runs after checkout; safe to run repeatedly against cached state.
set -euo pipefail

cd "$(dirname "$0")/.."

# Fetch modules and the Go toolchain pinned in go.mod, then warm the build cache.
go mod download
go build ./...

# Pin golangci-lint to the version CI uses (.github/workflows/ci.yml) so
# `golangci-lint run ./...` reproduces CI locally. Install into /usr/local/bin
# (the canonical on-PATH tools dir the Cloud Agent image uses for gh et al.);
# GOPATH/bin is not guaranteed to be on PATH in the built environment.
# Skip re-install when the pinned version is already present.
GOLANGCI_LINT_VERSION="v2.11.4"
if ! golangci-lint version 2>/dev/null | grep -qE 'version 2\.11\.4'; then
  tmpdir="$(mktemp -d)"
  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
    | sh -s -- -b "$tmpdir" "$GOLANGCI_LINT_VERSION"
  if [ -w /usr/local/bin ]; then
    install -m 0755 "$tmpdir/golangci-lint" /usr/local/bin/golangci-lint
  elif sudo -n true 2>/dev/null; then
    sudo install -m 0755 "$tmpdir/golangci-lint" /usr/local/bin/golangci-lint
  else
    install -d "$(go env GOPATH)/bin"
    install -m 0755 "$tmpdir/golangci-lint" "$(go env GOPATH)/bin/golangci-lint"
  fi
  rm -rf "$tmpdir"
fi

echo "obsidian-mcp environment ready: $(go version), $(golangci-lint version 2>/dev/null | head -1)"
