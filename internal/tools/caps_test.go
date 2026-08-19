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
}

func TestResolveCapsNilClientFailClosed(t *testing.T) {
	d := ResolveCaps(Deps{Client: nil})
	require.Equal(t, obsidian.Safe36Caps(), d.Caps)
}

func TestCapsVersionLabel(t *testing.T) {
	require.Equal(t, "4.1.7", capsVersionLabel(obsidian.Caps{Version: "4.1.7"}))
	require.Equal(t, "4.x+", capsVersionLabel(obsidian.Caps{}))
}
