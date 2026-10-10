package tools

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/vgromanov/obsidian-mcp/internal/obsidian"
)

func TestResolveCapsFieldOverride(t *testing.T) {
	d := ResolveCaps(Deps{RestAPIVersion: "4.1.7"})
	require.True(t, d.Caps.MoveVaultFile)
	require.False(t, d.Caps.RestDataviewDQL)
	require.True(t, d.Caps.Periodic)

	d2 := ResolveCaps(d) // idempotent
	require.Equal(t, d.Caps, d2.Caps)

	d3 := ResolveCaps(Deps{RestAPIVersion: "3.6.1"})
	require.False(t, d3.Caps.MoveVaultFile)
	require.True(t, d3.Caps.RestDataviewDQL)

	d5 := ResolveCaps(Deps{RestAPIVersion: "5.0.3"})
	require.True(t, d5.Caps.MoveVaultFile)
	require.False(t, d5.Caps.RestDataviewDQL)
	require.False(t, d5.Caps.Periodic)
	require.False(t, d5.Caps.PluginGraphTraverse)
	require.Equal(t, "5.0.3", d5.Caps.Version)
}

func TestResolveCapsNilClientFailClosed(t *testing.T) {
	d := ResolveCaps(Deps{Client: nil})
	require.Equal(t, obsidian.Safe36Caps(), d.Caps)
}

func TestCapsVersionLabel(t *testing.T) {
	require.Equal(t, "4.1.7", capsVersionLabel(obsidian.Caps{Version: "4.1.7"}))
	require.Equal(t, "4.x+", capsVersionLabel(obsidian.Caps{}))
}

func TestSearchVaultDescription(t *testing.T) {
	rest := searchVaultDescription(obsidian.Caps{RestDataviewDQL: true})
	require.Contains(t, rest, "Dataview DQL")
	require.Contains(t, rest, `TABLE status FROM "Projects" WHERE status = "active"`)
	require.Contains(t, rest, "LIST FROM #research")
	require.NotContains(t, rest, "removed in Local REST API 4.0")

	plugin := searchVaultDescription(obsidian.Caps{PluginDataviewQuery: true})
	require.Contains(t, plugin, "POST /dataview/query/")
	require.Contains(t, plugin, `TABLE status FROM "Projects"`)
	require.Contains(t, plugin, "LIST FROM #research")
	require.NotContains(t, plugin, "removed in Local REST API 4.0")

	none := searchVaultDescription(obsidian.Caps{})
	require.Contains(t, none, "JsonLogic")
	require.Contains(t, none, "removed in Local REST API 4.0")
	require.Contains(t, none, "POST /dataview/query/")
}
