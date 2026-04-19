# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-04-19

Initial public release.

### Added

- Go MCP server (`obsidian-mcp`) that talks to the Obsidian Local REST API.
- 26 MCP tools:
  - 16 Local REST API tools: `get_server_info`, active-file CRUD + patch + open,
    vault list/get/create/append/patch/delete, and the two search tools
    (`search_vault`, `search_vault_simple`).
  - `list_tags`, `get_tag_files` (the latter implemented as a JsonLogic search
    because upstream Local REST API exposes only `GET /tags/`).
  - `list_commands`, `execute_command`.
  - `get_periodic_note`, `update_periodic_note`, `append_to_periodic_note`,
    `patch_periodic_note`, `delete_periodic_note` (current period only).
  - `search_vault_smart` (Smart Connections), `execute_template` (Templater),
    and a generic `fetch` tool with HTML→Markdown conversion.
- Vault-backed dynamic prompts via an MCP middleware: any note tagged
  `mcp-tools-prompt` in the prompts folder is exposed as an MCP prompt and
  executed through the Templater endpoint.
- Two transports: `stdio` (default) and streamable HTTP at `/mcp`.
- Configuration via environment variables and CLI flags.
- Apache-2.0 license, README, CONTRIBUTING, SECURITY, full tool reference docs.
- `Makefile` targets for fmt/vet/test/cover/build/dist.
- `Dockerfile` (distroless-style, static binary on Alpine).
- GitHub Actions CI (test + vet on Ubuntu and macOS).
- Tag-triggered release workflow producing cross-platform archives and a
  multi-arch GHCR image via GoReleaser.

[Unreleased]: https://github.com/vgromanov/obsidian-mcp/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/vgromanov/obsidian-mcp/releases/tag/v0.1.0
