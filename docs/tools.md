# Tools parity

Go `obsidian-mcp` mirrors [jacksteamdev/obsidian-mcp-tools](https://github.com/jacksteamdev/obsidian-mcp-tools) `packages/mcp-server` tools (same **names** and roles for the original set). This binary also exposes extra Local REST API routes (tags, commands, periodic notes) as MCP tools.

| Tool | Upstream region | Notes |
|------|-----------------|-------|
| `get_server_info` | Local REST API | `GET /` |
| `get_active_file` | Local REST API | `GET /active/` |
| `update_active_file` | Local REST API | `PUT /active/` |
| `append_to_active_file` | Local REST API | `POST /active/` |
| `patch_active_file` | Local REST API | `PATCH /active/` |
| `delete_active_file` | Local REST API | `DELETE /active/` |
| `show_file_in_obsidian` | Local REST API | `POST /open/...` |
| `search_vault` | Local REST API | `POST /search/` (Dataview / JsonLogic) |
| `search_vault_simple` | Local REST API | `POST /search/simple/` |
| `list_vault_files` | Local REST API | `GET /vault/` |
| `get_vault_file` | Local REST API | `GET /vault/...` |
| `create_vault_file` | Local REST API | `PUT /vault/...` |
| `append_to_vault_file` | Local REST API | `POST /vault/...` |
| `patch_vault_file` | Local REST API | `PATCH /vault/...` |
| `delete_vault_file` | Local REST API | `DELETE /vault/...` |
| `list_tags` | Local REST API | `GET /tags/` |
| `get_tag_files` | Local REST API | `POST /search/` JsonLogic (`{"in":[<tag>,{"var":"tags"}]}`) — upstream has no per-tag route |
| `list_commands` | Local REST API | `GET /commands/` |
| `execute_command` | Local REST API | `POST /commands/{commandId}/` (runs in Obsidian UI) |
| `get_periodic_note` | Local REST API | `GET /periodic/{period}/` (current period only) |
| `update_periodic_note` | Local REST API | `PUT /periodic/{period}/` |
| `append_to_periodic_note` | Local REST API | `POST /periodic/{period}/` |
| `patch_periodic_note` | Local REST API | `PATCH /periodic/{period}/` |
| `delete_periodic_note` | Local REST API | `DELETE /periodic/{period}/` |
| `search_vault_local` | Local Smart Lookup | `POST /local-smart-lookup/search/` (extension route; optional oMLX preflight) |
| `execute_template` | Templater | `POST /templates/execute` (Obsidian plugin route) |
| `fetch` | Built-in | HTML→Markdown via `html-to-markdown` |

**Count:** 26 tools (24 Local REST API + local semantic search + templater + fetch).

### `search_vault_local` arguments

| Argument | Type | Notes |
|----------|------|-------|
| `query` | string | Required natural-language question |
| `limit` | number | Max chunk results (plugin default if omitted) |
| `dataviewSource` | string | Dataview source expression to narrow paths before vector search |
| `dataviewQuery` | string | Full Dataview DQL to resolve allowed paths |
| `tags` | string[] | LanceDB metadata filter (frontmatter or inline tags) |
| `frontmatter` | object | LanceDB metadata filter on indexed scalar frontmatter fields |
| `where` | string | LanceDB SQL-style metadata filter (e.g. `type = 'note'`) |

> Tag rename is intentionally not exposed: upstream Local REST API has no `PATCH /tags/{tag}/` route, and emulating it client-side (rewriting every matching file) is too risky for a tool an LLM might call by mistake. Use Obsidian's UI to rename tags vault-wide.

## Prerequisites

- **Local REST API** or **[obsidian-api](https://github.com/vigeron/obsidian-api)** with extension support (required).
- **obsidian-mcp-tools** Obsidian plugin (required for `/templates/execute` and vault prompts — this Go binary replaces only the **downloaded MCP server**, not those routes).
- **Local Smart Lookup** (`local-smart-lookup` plugin) + **oMLX** on `http://127.0.0.1:8000/v1` with embedding model loaded (required for `search_vault_local`). Set the plugin **Embedding server** to the same host as `OMLX_BASE_URL`.
- **Dataview** (optional; required when using `dataviewSource` / `dataviewQuery` on `search_vault_local`).
- **Templater** (required for `execute_template` and vault prompts).
- **Periodic Notes** (community plugin) configured in Obsidian — required for `/periodic/...` tools to resolve notes; the Local REST API returns errors if the plugin is missing or a period is disabled.

### `search_vault_local` MCP environment

| Variable | Default | Meaning |
|----------|---------|---------|
| `OMLX_BASE_URL` | `http://127.0.0.1:8000/v1` | oMLX OpenAI-compatible API base (preflight `GET /models`) |
| `OMLX_API_KEY` | _(empty)_ | Bearer token when oMLX auth is enabled |
| `OBSIDIAN_OMLX_CHECK` | `true` | When `true`, probe oMLX before calling Obsidian (set `false` to skip) |
