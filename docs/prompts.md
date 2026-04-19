# Prompts parity

Dynamic prompts match upstream `setupObsidianPrompts` in `packages/mcp-server/src/features/prompts/index.ts`:

1. **`prompts/list`** — `GET /vault/{PromptsDir}/`, then for each `*.md` load `application/vnd.olrapi.note+json`, keep files whose **top-level `tags`** include `mcp-tools-prompt`. Each list entry includes `name` (filename), `description` from YAML frontmatter when present, and `arguments` parsed from Templater tags.
2. **`prompts/get`** — Load `PromptsDir/name`, require tag `mcp-tools-prompt` (note tags or YAML `tags`), validate arguments against `<% tp.mcpTools.prompt("arg","desc") %>` declarations, `POST /templates/execute`, strip trailing segment after splitting on `---` (same heuristic as upstream).

## Configuration

- CLI/env: `--prompts-dir` / `OBSIDIAN_PROMPTS_DIR` (default `Prompts`).

## Implementation note

The official MCP Go SDK lists registered prompts from an internal table; this server **intercepts** `prompts/list` and `prompts/get` via receiving middleware so the catalog always reflects the current vault (same freshness as the TypeScript server).
