## Summary

<!-- What does this change do, and why? Reference any issues with `Fixes #123`. -->

## Type of change

- [ ] Bug fix
- [ ] New tool / transport / feature
- [ ] Refactor (no behavior change)
- [ ] Docs / build / CI

## Test plan

<!-- Commands you ran to validate this. -->

- [ ] `make fmt vet test`
- [ ] `make build`
- [ ] Manual smoke test against a real Obsidian (`make test-integration` or by-hand MCP call)

## Checklist

- [ ] `CHANGELOG.md` updated under `Unreleased`
- [ ] `docs/tools.md` updated if a tool was added/removed/renamed
- [ ] Tool count comment in `internal/tools/register.go` matches reality
- [ ] No new `gitlabci.*` / private-host references
- [ ] No secrets, binaries, or coverage files committed
