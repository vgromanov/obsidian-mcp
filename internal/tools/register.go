package tools

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterAll registers every Obsidian MCP tool (26 total).
func RegisterAll(s *mcp.Server, d Deps) {
	RegisterLocalREST(s, d)
	RegisterTags(s, d)
	RegisterCommands(s, d)
	RegisterPeriodic(s, d)
	RegisterSmartConnections(s, d)
	RegisterTemplater(s, d)
	RegisterFetch(s)
}
