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
| `search_vault` | Local REST API | `POST /search/` — JsonLogic always; Dataview DQL only on REST **&lt;4.0** (rejected with a clear error on 4.x+) |
| `search_vault_simple` | Local REST API | `POST /search/simple/` |
| `list_vault_files` | Local REST API | `GET /vault/` |
| `get_vault_file` | Local REST API | `GET /vault/...` — paginated (`maxLength` default **32768**, `startIndex`); omit `format` for markdown; `format=json` returns NoteJson (`links`/`backlinks` on 4.x) |
| `create_vault_file` | Local REST API | `PUT /vault/...` |
| `append_to_vault_file` | Local REST API | `POST /vault/...` |
| `patch_vault_file` | Local REST API | `PATCH /vault/...` — response body capped with same `maxLength`/`startIndex` as `get_vault_file` |
| `delete_vault_file` | Local REST API | `DELETE /vault/...` |
| `move_vault_file` | Local REST API ≥4.1.0 | `MOVE /vault/...` — `Destination`, `Allow-Overwrite`; registered only when capability probe says ≥4.1.0 |
| `list_tags` | Local REST API | `GET /tags/` |
| `get_tag_files` | Local REST API | `POST /search/` JsonLogic (`{"in":[<tag>,{"var":"tags"}]}`) — upstream has no per-tag route |
| `list_frontmatter_keys` | Local Smart Lookup | `GET /frontmatter_keys/` (Properties inventory; not under `/si/*`) |
| `get_frontmatter_key_files` | Local Smart Lookup | `GET /frontmatter_keys/{name}/` (files with that property; unknown → `[]`) |
| `list_commands` | Local REST API | `GET /commands/` |
| `execute_command` | Local REST API | `POST /commands/{commandId}/` (runs in Obsidian UI) |
| `get_periodic_note` | Local REST API | `GET /periodic/{period}/` (current period only) |
| `update_periodic_note` | Local REST API | `PUT /periodic/{period}/` |
| `append_to_periodic_note` | Local REST API | `POST /periodic/{period}/` |
| `patch_periodic_note` | Local REST API | `PATCH /periodic/{period}/` |
| `delete_periodic_note` | Local REST API | `DELETE /periodic/{period}/` |
| `search_vault_local` | Local Smart Lookup | `POST /local-smart-lookup/search/` (extension route; optional oMLX preflight) |
| `si_health` | Local Smart Lookup SI | `GET /si/health/` |
| `si_index_info` | Local Smart Lookup SI | `GET /si/index_info/` |
| `si_embed_text` | Local Smart Lookup SI | `POST /si/embed_text/` |
| `si_query_metadata` | Local Smart Lookup SI | `POST /si/query_metadata/` (no `offset`; use `cursor`) |
| `si_knn` | Local Smart Lookup SI | `POST /si/knn/` (requires `type` in `where`; exactly one of `text`/`vector`/`chunk_id`) |
| `si_count_neighbors` | Local Smart Lookup SI | `POST /si/count_neighbors/` (same corpus + query XOR as `si_knn`) |
| `si_get_vectors` | Local Smart Lookup SI | `POST /si/get_vectors/` (requires `type` in `where`; no `offset`; use `cursor`) |
| `si_filter_validate` | Local Smart Lookup SI | `POST /si/filter/validate/` |
| `execute_template` | Templater | `POST /templates/execute` (Obsidian plugin route) |
| `fetch` | Built-in | HTML→Markdown via `html-to-markdown` |

**Count:** **37** tools on Local REST **3.6.x** (and fail-closed unknown/5.x); **38** on **≥4.1.0** (adds `move_vault_file`). Breakdown: 24–25 Local REST API + 2 Properties hygiene + local semantic search + 8 SI + templater + fetch.

### Capability matrix (Local REST API)

At server build the process probes `GET /` (`versions.self`, then `manifest.version`) unless `REST_API_VERSION` / `OBSIDIAN_REST_API_VERSION` (or `tools.Deps.RestAPIVersion`) overrides. Fail-closed → 3.6-safe catalog (no `move_vault_file`; never advertise 5.x-only tools).

| Plugin version | `move_vault_file` | REST Dataview DQL on `search_vault` | Periodic tools | Notes |
|----------------|-------------------|--------------------------------------|----------------|-------|
| 3.6.x | no | yes | yes | Current Cursor baseline |
| 4.0.x | no | no (JsonLogic only; clear error on `queryType=dataview`) | yes | |
| 4.1.x+ (target **4.1.7**) | yes | no | yes | NoteJson `links`/`backlinks` |
| 5.x / unknown / probe fail | no | yes (3.6-safe) | yes | 5.x unsupported; no `vault_copy` / trash-delete / JSON PATCH / document-map |

### `move_vault_file`

| Argument | Type | Notes |
|----------|------|-------|
| `filename` | string | Source vault-relative path |
| `destination` | string | Vault-relative; trailing `/` keeps the source filename under that directory; absolute `/…` rejected |
| `updateLinks` | bool | Default **true**. REST uses `app.fileManager.renameFile` (no separate header). If Obsidian **alwaysUpdateLinks** is off, a UI modal may block the MOVE. |
| `allowOverwrite` | bool | Default **false** → `Allow-Overwrite: false` |

### `search_vault` on 4.x

`search_vault` stays advertised for **JsonLogic**. Do not use `queryType=dataview` on ≥4.0 — the tool returns a clear MCP error. For Dataview, use `search_vault_local` (`dataviewQuery` / `dataviewSource`). `search_vault_simple` is unchanged.

### `search_vault_local` arguments

| Argument | Type | Notes |
|----------|------|-------|
| `query` | string | Required natural-language question |
| `limit` | number | Max chunk results (plugin default if omitted) |
| `dataviewSource` | string | Compatible alias for a bare Dataview source (`#tag`, `"Folder"`). Local Smart Lookup wraps it as `LIST FROM …` and runs it via `api.query` (same safe path as `dataviewQuery`). Prefer `dataviewQuery` for full DQL. |
| `dataviewQuery` | string | Full Dataview DQL (`LIST`/`TABLE`/`TASK`/`CALENDAR`) to resolve allowed paths (preferred) |
| `tags` | string[] | LanceDB metadata filter (frontmatter or inline tags) |
| `frontmatter` | object | LanceDB metadata filter on indexed scalar frontmatter fields |
| `where` | string | LanceDB SQL-style metadata filter (e.g. `type = 'note'`) |

> Tag rename is intentionally not exposed: upstream Local REST API has no `PATCH /tags/{tag}/` route, and emulating it client-side (rewriting every matching file) is too risky for a tool an LLM might call by mistake. Use Obsidian's UI to rename tags vault-wide.

### `get_vault_file` / `patch_vault_file` pagination

Large notes can exceed Cursor's ~50 KB inline tool-result limit. Both tools slice the **returned text by bytes** (same model as `fetch`) and never declare a field named `offset` (that produced JSON Schema `true` and broke Grok Bot `tools/list` — see RVG-104).

| Argument | Type | Notes |
|----------|------|-------|
| `filename` | string | Vault-relative path (required) |
| `format` | string | Optional; `json` returns the note JSON envelope when the payload fits in one page (`content`, `frontmatter`, `path`, `stat`, `tags`, and on Local REST 4.x `links` / `backlinks` from the metadata cache — not `unresolvedLinks`). Omit for markdown (preferred for agents). |
| `maxLength` | integer | Max bytes per page. Default **32768** for vault reads (unlike `fetch`, which defaults to 5000). |
| `startIndex` | integer | Byte index into the returned text for the next page (from the overflow notice / `pagination.endIndex`). |

On overflow the text includes a short notice (`file too large (N bytes); call again with startIndex=…`) and `structuredContent.pagination` exposes `totalLength`, `startIndex`, `endIndex`, and `hasMore`. `patch_vault_file` applies the same cap to the post-patch body. Pagination is an MCP response concern — the Local REST call still fetches the full file.

### Properties hygiene (`list_frontmatter_keys` / `get_frontmatter_key_files`)

Requires **Local Smart Lookup** with the `/frontmatter_keys*` extension routes (see that plugin's README). Use `list_frontmatter_keys` before inventing a new frontmatter key; prefer reusing an existing high-`count` property. `get_frontmatter_key_files` helps decide whether a rare key is a one-off.

| Tool argument | Type | Notes |
|---------------|------|-------|
| `name` (`get_frontmatter_key_files`) | string | Property key (YAML / Obsidian Properties name) |

### Semantic Index (`si_*`)

Requires **Local Smart Lookup** with `/si/*` routes (trailing slashes). These wrap the mining/query API used by dreamcycle: no cross-encoder rerank; cosine **distance** thresholds are inclusive (`<=`).

| Tool | Key arguments | Notes |
|------|---------------|-------|
| `si_health` | — | Liveness |
| `si_index_info` | — | Regime stamp (`embed_model` / `embed_dim` / `schema_ver`) |
| `si_embed_text` | `texts`, `normalize?` | Index-consistent embeddings |
| `si_query_metadata` | `where`, `fields`, `limit?`, `cursor?` | Keyset scan; `offset` not in schema |
| `si_knn` | `where`, exactly one of `text` / `vector` / `chunk_id`, `k?`, `threshold?` | `where` must contain `"type"`; `text` embeds internally and does **not** return the query vector |
| `si_count_neighbors` | same query XOR + required `threshold`, `group_by`, `where` | Exact grouped counts |
| `si_get_vectors` | `where`, `include_text?`, `limit?`, `cursor?` | Vector export; keep off public MCP allowlists |
| `si_filter_validate` | `where?`, `filter?`, `limit?` | Compile + live sample |

> Do not deploy binaries that expose `si_get_vectors` / `si_embed_text` on a public gateway until an allowlist filters them.

## Prerequisites

- **Local REST API** or **[obsidian-api](https://github.com/vigeron/obsidian-api)** with extension support (required).
- **obsidian-mcp-tools** Obsidian plugin (required for `/templates/execute` and vault prompts — this Go binary replaces only the **downloaded MCP server**, not those routes).
- **Local Smart Lookup** (`local-smart-lookup` plugin) + **oMLX** on `http://127.0.0.1:8000/v1` with embedding model loaded (required for `search_vault_local` and `si_*`). Set the plugin **Embedding server** to the same host as `OMLX_BASE_URL`.
- **Dataview** (optional; required when using `dataviewSource` / `dataviewQuery` on `search_vault_local`).
- **Templater** (required for `execute_template` and vault prompts).
- **Periodic Notes** (community plugin) configured in Obsidian — required for `/periodic/...` tools to resolve notes; the Local REST API returns errors if the plugin is missing or a period is disabled.

### `search_vault_local` MCP environment

| Variable | Default | Meaning |
|----------|---------|---------|
| `OMLX_BASE_URL` | `http://127.0.0.1:8000/v1` | oMLX OpenAI-compatible API base (preflight `GET /models`) |
| `OMLX_API_KEY` | _(empty)_ | Bearer token when oMLX auth is enabled |
| `OBSIDIAN_OMLX_CHECK` | `true` | When `true`, probe oMLX before calling Obsidian (set `false` to skip) |
