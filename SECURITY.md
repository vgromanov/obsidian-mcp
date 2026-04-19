# Security policy

## Supported versions

The latest minor release is supported. Older versions get fixes only at the
maintainers' discretion.

## Reporting a vulnerability

Please report security issues privately via a
[GitHub Security Advisory](https://github.com/vgromanov/obsidian-mcp/security/advisories/new)
rather than a public issue. We will acknowledge within a few business days.

If GitHub advisories are unavailable to you, contact the maintainer directly
through the email listed on their GitHub profile.

## Threat model

`obsidian-mcp` is intended for **personal, local use** alongside an Obsidian
vault. The defaults reflect that:

- **stdio transport (default).** The MCP client (e.g. Cursor) launches the
  binary as a subprocess; secrets are passed via the client's `env` block.
  This is the recommended deployment.
- **streamable HTTP transport (`--transport=http`).** Listens on the configured
  address with **no authentication**. Bind it to `127.0.0.1` only. Do not
  expose it to other hosts or to the public internet without putting an
  authenticating reverse proxy in front of it.

## Secrets

- `OBSIDIAN_API_KEY` is sent as an `Authorization: Bearer ...` header to the
  Obsidian Local REST API. Treat it like a password. Prefer environment
  injection from your MCP client over committing it to disk.
- HTTPS to Obsidian uses `InsecureSkipVerify` because the Local REST API ships
  a self-signed certificate by default. This matches the upstream Node server's
  behavior. Set `OBSIDIAN_USE_HTTP=true` to use plain HTTP `:27123` instead.

## Tool side effects

Several MCP tools are destructive or trigger UI side effects in Obsidian
(e.g. `delete_vault_file`, `update_*`, `execute_command`). LLMs invoking these
tools should be sandboxed or supervised; this server does not implement
per-tool allowlisting beyond what the MCP client provides.
