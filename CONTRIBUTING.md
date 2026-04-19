# Contributing

Thanks for considering a contribution. Issues, bug reports, and pull requests
are all welcome.

## Requirements

- **Go 1.25+** (see `go.mod`).
- **Obsidian** with the [Local REST API](https://github.com/coddingtonbear/obsidian-local-rest-api)
  plugin enabled, plus the
  [obsidian-mcp-tools](https://github.com/jacksteamdev/obsidian-mcp-tools)
  plugin if you want to test Smart Connections / Templater / vault prompts.

## Workflow

1. Fork and clone the repo.
2. Create a topic branch.
3. Make your change.
4. Run the standard loop:

   ```bash
   make fmt vet test
   ```

5. If you touched anything user-facing, update [README.md](README.md),
   [docs/](docs/), and [CHANGELOG.md](CHANGELOG.md) under the `Unreleased`
   section.
6. Open a pull request. Describe the user-visible change and the test plan in
   the PR body.

## Tests

- Unit tests are stdlib `testing` plus
  [`testify/require`](https://github.com/stretchr/testify), backed by
  `httptest.Server` so they run without a real Obsidian.
- `make cover` prints a coverage summary.
- Integration tests against a real Obsidian (skipped without an API key):

  ```bash
  export OBSIDIAN_API_KEY=...
  make test-integration
  ```

## Conventions

- Tool names mirror upstream
  [`jacksteamdev/obsidian-mcp-tools`](https://github.com/jacksteamdev/obsidian-mcp-tools)
  (MCP server portion only) so client configs remain portable. Avoid renames.
- Tool input structs go through the MCP Go SDK schema generator. Keep field
  types JSON-Schema friendly (no `any` / `map[string]any` if avoidable). Use
  `json.RawMessage` for free-form payloads.
- Keep new client methods documented with the upstream route they call.
- Keep `docs/tools.md` and the registrar count comment in
  `internal/tools/register.go` in sync with reality.

## Releases

Releases are cut from `main` by pushing a `vX.Y.Z` tag. The
[`release` workflow](.github/workflows/release.yml) runs
[GoReleaser](https://goreleaser.com/) to publish:

- cross-platform archives + `SHA256SUMS` to GitHub Releases,
- a multi-arch Docker image to `ghcr.io/vgromanov/obsidian-mcp`.

Update `CHANGELOG.md` in the same commit as the tag.

## Reporting security issues

See [SECURITY.md](SECURITY.md). Do **not** open public issues for vulnerabilities.

## License

By contributing, you agree that your contributions are licensed under the
[Apache License 2.0](LICENSE).
