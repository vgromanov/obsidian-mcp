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
| `search_vault_smart` | Smart Connections | `POST /search/smart` (Obsidian plugin route) |
| `execute_template` | Templater | `POST /templates/execute` (Obsidian plugin route) |
| `fetch` | Built-in | HTML→Markdown via `html-to-markdown` |

**Count:** 26 tools (24 Local REST API + smart search + templater + fetch).

> Tag rename is intentionally not exposed: upstream Local REST API has no `PATCH /tags/{tag}/` route, and emulating it client-side (rewriting every matching file) is too risky for a tool an LLM might call by mistake. Use Obsidian's UI to rename tags vault-wide.

## Prerequisites

- **Local REST API** plugin (required).
- **obsidian-mcp-tools** Obsidian plugin (required for `/search/smart` and `/templates/execute` — this Go binary replaces only the **downloaded MCP server**, not those routes).
- **Smart Connections** + **Templater** (recommended; required for the two plugin routes above to succeed).
- **Periodic Notes** (community plugin) configured in Obsidian — required for `/periodic/...` tools to resolve notes; the Local REST API returns errors if the plugin is missing or a period is disabled.
