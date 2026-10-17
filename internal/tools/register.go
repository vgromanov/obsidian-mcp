package tools

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vgromanov/obsidian-mcp/internal/obsidian"
)

// RegisterAll registers Obsidian MCP tools gated by Local REST capabilities.
// Base catalog is 37 tools (3.6-safe). On REST ≥4.1.0, move_vault_file is also
// registered (38 tools). On REST ≥5.0, periodic tools are omitted (33 tools:
// 4.1 catalog minus five *_periodic_note). graph_traverse is added only when
// GET / advertises POST /graph/traverse/ (independent of the REST semver).
// Probe/override runs via ResolveCaps before registration. While the caps are
// the unknown-version fallback, the server re-probes lazily and re-registers the
// version-dependent tools (the SDK then sends notifications/tools/list_changed).
func RegisterAll(s *mcp.Server, d Deps) {
	d = ResolveCaps(d)
	d.CapsState.SetOnChange(func(old, next obsidian.Caps) { syncCapsTools(s, d, old, next) })
	s.AddReceivingMiddleware(capsRefreshMiddleware(d.CapsState))
	RegisterLocalREST(s, d)
	RegisterTags(s, d)
	RegisterFrontmatterKeys(s, d)
	RegisterCommands(s, d)
	if d.Caps.Periodic {
		RegisterPeriodic(s, d)
	}
	RegisterLocalSmartLookup(s, d)
	RegisterSi(s, d)
	if d.Caps.PluginGraphTraverse {
		RegisterGraph(s, d)
	}
	RegisterTemplater(s, d)
	RegisterFetch(s)
}

// syncCapsTools re-registers the tools whose presence or description depends on
// the caps. AddTool replaces by name; RemoveTools drops the rest. Each change
// makes the SDK queue notifications/tools/list_changed for connected sessions.
func syncCapsTools(s *mcp.Server, d Deps, old, next obsidian.Caps) {
	if searchVaultDescription(old) != searchVaultDescription(next) {
		registerSearchVault(s, d)
	}
	if old.MoveVaultFile != next.MoveVaultFile {
		if next.MoveVaultFile {
			registerMoveVaultFile(s, d)
		} else {
			s.RemoveTools(moveVaultFileToolName)
		}
	}
	if old.Periodic != next.Periodic {
		if next.Periodic {
			RegisterPeriodic(s, d)
		} else {
			s.RemoveTools(periodicToolNames...)
		}
	}
	if old.PluginGraphTraverse != next.PluginGraphTraverse {
		if next.PluginGraphTraverse {
			RegisterGraph(s, d)
		} else {
			s.RemoveTools(graphTraverseToolName)
		}
	}
}
