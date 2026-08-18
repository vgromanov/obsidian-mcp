package tools

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterAll registers Obsidian MCP tools gated by Local REST capabilities.
// Base catalog is 37 tools (3.6-safe). On REST ≥4.1.0, move_vault_file is also
// registered (38 tools). Probe/override runs via ResolveCaps before registration.
func RegisterAll(s *mcp.Server, d Deps) {
	d = ResolveCaps(d)
	RegisterLocalREST(s, d)
	RegisterTags(s, d)
	RegisterFrontmatterKeys(s, d)
	RegisterCommands(s, d)
	if d.Caps.Periodic {
		RegisterPeriodic(s, d)
	}
	RegisterLocalSmartLookup(s, d)
	RegisterSi(s, d)
	RegisterTemplater(s, d)
	RegisterFetch(s)
}
