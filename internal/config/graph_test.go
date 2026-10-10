package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadGraphTraverseDefaults(t *testing.T) {
	t.Setenv("OBSIDIAN_GRAPH_SCOPE_ALLOWLIST", "")
	t.Setenv("OBSIDIAN_GRAPH_MAX_LIMIT_NODES", "")
	t.Setenv("OBSIDIAN_GRAPH_ALLOW_BODY", "")
	t.Setenv("OBSIDIAN_GRAPH_ALLOW_FULL_EXPORT", "")

	got := loadGraphTraverse()
	require.Empty(t, got.ScopeAllowlist)
	require.Equal(t, 0, got.MaxLimitNodes)
	require.False(t, got.MaxLimitNodesInvalid)
	require.True(t, got.AllowBody)
	require.True(t, got.AllowFullExport)
}

func TestLoadGraphTraversePublicProfile(t *testing.T) {
	t.Setenv("OBSIDIAN_GRAPH_SCOPE_ALLOWLIST", " Projects/ , Notes ")
	t.Setenv("OBSIDIAN_GRAPH_MAX_LIMIT_NODES", "500")
	t.Setenv("OBSIDIAN_GRAPH_ALLOW_BODY", "false")
	t.Setenv("OBSIDIAN_GRAPH_ALLOW_FULL_EXPORT", "0")

	got := loadGraphTraverse()
	require.Equal(t, []string{"Projects/", "Notes"}, got.ScopeAllowlist)
	require.Equal(t, 500, got.MaxLimitNodes)
	require.False(t, got.MaxLimitNodesInvalid)
	require.False(t, got.AllowBody)
	require.False(t, got.AllowFullExport)
}

func TestParseMaxLimitNodes(t *testing.T) {
	n, invalid := parseMaxLimitNodes("0")
	require.Equal(t, 0, n)
	require.False(t, invalid)

	n, invalid = parseMaxLimitNodes("nope")
	require.Equal(t, 0, n)
	require.True(t, invalid)

	n, invalid = parseMaxLimitNodes("-3")
	require.True(t, invalid)
	require.Equal(t, 0, n)
}
