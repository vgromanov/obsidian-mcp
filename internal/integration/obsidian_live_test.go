//go:build integration

package integration

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vgromanov/obsidian-mcp/internal/obsidian"
)

// TestLiveGetServerInfo calls a real Obsidian Local REST API when OBSIDIAN_API_KEY is set.
func TestLiveGetServerInfo(t *testing.T) {
	key := os.Getenv("OBSIDIAN_API_KEY")
	if key == "" {
		t.Skip("set OBSIDIAN_API_KEY for integration")
	}
	host := os.Getenv("OBSIDIAN_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	useHTTP := os.Getenv("OBSIDIAN_USE_HTTP") == "true"
	cli, err := obsidian.NewClient(host, useHTTP, key)
	require.NoError(t, err)
	raw, err := cli.GetServerInfo(context.Background())
	require.NoError(t, err)
	require.Contains(t, string(raw), `"status"`)
}
