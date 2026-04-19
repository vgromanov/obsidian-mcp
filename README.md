# obsidian-mcp

[![CI](https://github.com/vgromanov/obsidian-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/vgromanov/obsidian-mcp/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/vgromanov/obsidian-mcp.svg)](https://pkg.go.dev/github.com/vgromanov/obsidian-mcp)
[![Go Report Card](https://goreportcard.com/badge/github.com/vgromanov/obsidian-mcp)](https://goreportcard.com/report/github.com/vgromanov/obsidian-mcp)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/vgromanov/obsidian-mcp?sort=semver)](https://github.com/vgromanov/obsidian-mcp/releases)

A single-binary [Model Context Protocol](https://modelcontextprotocol.io/) server for [Obsidian](https://obsidian.md/), written in Go. Drop-in replacement for the MCP server portion of [jacksteamdev/obsidian-mcp-tools](https://github.com/jacksteamdev/obsidian-mcp-tools): same tool names, no Node/Bun runtime, stdio + streamable HTTP transports, vault-backed dynamic prompts.

## What it gives you

- **26 MCP tools** covering the full [Local REST API](https://github.com/coddingtonbear/obsidian-local-rest-api) surface (active note, vault CRUD, search, open, tags, commands, periodic notes), plus Smart Connections semantic search, Templater execution, and a generic `fetch` tool with HTML→Markdown conversion. See [docs/tools.md](docs/tools.md).
- **Vault-backed prompts** — any note tagged `mcp-tools-prompt` in your prompts folder is exposed as an MCP prompt, executed through Templater on the Obsidian side. See [docs/prompts.md](docs/prompts.md).
- **Two transports** — `stdio` (default) for editor integrations, `--transport=http` for shared local use.
- **Single static binary** (~12 MB), no runtime dependencies, easy to ship.

## Why a Go rewrite

Upstream is a TypeScript/Bun monorepo with an Obsidian "install MCP server" wrapper UI. This repo is the inverse: a small, scriptable binary you `go install` once and configure manually. Use it when you want the same MCP capabilities without Node, Bun, or in-app installer flows. The upstream Obsidian plugin is still required so the Local REST API exposes `POST /search/smart` and `POST /templates/execute`; only the external MCP **process** is replaced.

## Prerequisites

| Component | Required for |
|-----------|--------------|
| [Obsidian](https://obsidian.md/) | All tools |
| [Local REST API](https://github.com/coddingtonbear/obsidian-local-rest-api) plugin | All tools |
| [obsidian-mcp-tools](https://github.com/jacksteamdev/obsidian-mcp-tools) plugin | `search_vault_smart`, `execute_template`, vault prompts |
| [Smart Connections](https://github.com/brianpetro/obsidian-smart-connections) | `search_vault_smart` |
| [Templater](https://github.com/SilentVoid13/Templater) | `execute_template` and dynamic prompts |
| [Periodic Notes](https://github.com/liamcain/obsidian-periodic-notes) | `*_periodic_note` tools |

API key is from **Local REST API → Settings → API Key**.

## Install

### From source

```bash
go install github.com/vgromanov/obsidian-mcp/cmd/obsidian-mcp@latest
```

### Pre-built binary

Grab the archive for your OS/arch from [Releases](https://github.com/vgromanov/obsidian-mcp/releases) and put `obsidian-mcp` on your `PATH`.

### Docker

```bash
docker pull ghcr.io/vgromanov/obsidian-mcp:latest
docker run --rm -i \
  -e OBSIDIAN_API_KEY=... \
  -e OBSIDIAN_HOST=host.docker.internal \
  ghcr.io/vgromanov/obsidian-mcp:latest
```

### Build locally

```bash
git clone https://github.com/vgromanov/obsidian-mcp.git
cd obsidian-mcp
make build
```

## Configuration

All knobs are environment variables (CLI flags override). Defaults work for a single-user local Obsidian install.

| Variable | Default | Meaning |
|----------|---------|---------|
| `OBSIDIAN_API_KEY` | _(required)_ | Bearer token for Local REST API |
| `OBSIDIAN_HOST` | `127.0.0.1` | Local REST API hostname |
| `OBSIDIAN_USE_HTTP` | `false` | `true` → HTTP `:27123`; `false` → HTTPS `:27124` (TLS verify off; same self-signed-cert tradeoff as upstream) |
| `OBSIDIAN_PROMPTS_DIR` | `Prompts` | Vault folder scanned for prompt-template notes |
| `OBSIDIAN_MCP_TRANSPORT` | `stdio` | Default for `--transport` (`stdio` or `http`) |
| `OBSIDIAN_MCP_ADDR` | `127.0.0.1:8765` | Default for `--addr` when `--transport=http` |

CLI:

```text
obsidian-mcp [--transport=stdio|http] [--addr=host:port] [--prompts-dir=Folder] [--version]
```

## Use with Cursor

### stdio (recommended)

`~/.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "obsidian": {
      "command": "obsidian-mcp",
      "env": {
        "OBSIDIAN_API_KEY": "YOUR_KEY"
      }
    }
  }
}
```

### Streamable HTTP

```bash
obsidian-mcp --transport=http --addr=127.0.0.1:8765
```

```json
{
  "mcpServers": {
    "obsidian": {
      "url": "http://127.0.0.1:8765/mcp"
    }
  }
}
```

The HTTP transport is **unauthenticated** by design. Bind to localhost only. See [SECURITY.md](SECURITY.md).

## Development

```bash
make fmt vet test    # standard loop
make cover           # coverage summary
make build           # static binary at ./obsidian-mcp
make dist            # cross-platform binaries under ./dist/
```

Integration tests against a real Obsidian (skipped without an API key):

```bash
export OBSIDIAN_API_KEY=...
make test-integration
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full workflow.

## Documentation

- [docs/tools.md](docs/tools.md) — full tool reference and parity table
- [docs/prompts.md](docs/prompts.md) — vault-backed dynamic prompts
- [SECURITY.md](SECURITY.md) — threat model and secret handling
- [CHANGELOG.md](CHANGELOG.md) — release notes

## Acknowledgements

- [jacksteamdev/obsidian-mcp-tools](https://github.com/jacksteamdev/obsidian-mcp-tools) — upstream Obsidian plugin and original TypeScript MCP server (this repo replaces only the latter).
- [coddingtonbear/obsidian-local-rest-api](https://github.com/coddingtonbear/obsidian-local-rest-api) — the HTTP surface this server talks to.
- [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) — official MCP Go SDK.

## License

[Apache-2.0](LICENSE)
