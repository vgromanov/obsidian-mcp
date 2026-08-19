package mcpapp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/vgromanov/obsidian-mcp/internal/obsidian"
	"github.com/vgromanov/obsidian-mcp/internal/tools"
)

func mcpSessionWithVersion(t *testing.T, cli *obsidian.Client, restVersion string) (context.Context, *mcp.ClientSession) {
	t.Helper()
	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	d := testDeps(cli)
	d.RestAPIVersion = restVersion
	srv := NewMCPServer(nil, d)
	_, err := srv.Connect(ctx, st, nil)
	require.NoError(t, err)
	c := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := c.Connect(ctx, ct, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })
	return ctx, cs
}

func toolNames(t *testing.T, ctx context.Context, cs *mcp.ClientSession) []string {
	t.Helper()
	var names []string
	for tool, err := range cs.Tools(ctx, nil) {
		require.NoError(t, err)
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	return names
}

func hasTool(names []string, name string) bool {
	return slices.Contains(names, name)
}

func TestCapabilityMatrixToolCatalog(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())

	cases := []struct {
		version      string
		wantMove     bool
		wantPeriodic bool
		wantDataview bool // search_vault description mentions Dataview DQL
		forbid5xOnly bool
	}{
		{"3.6.1", false, true, true, true},
		{"4.0.0", false, true, false, true},
		{"4.1.7", true, true, false, true},
		{"", false, true, true, true}, // unknown override empty → probe; use explicit unknown via garbage
		{"unknown", false, true, true, true},
		{"5.0.0", false, true, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.version, func(t *testing.T) {
			ver := tc.version
			if ver == "" {
				ver = "not-a-semver"
			}
			ctx, cs := mcpSessionWithVersion(t, cli, ver)
			names := toolNames(t, ctx, cs)
			require.Equal(t, tc.wantMove, hasTool(names, "move_vault_file"), names)
			require.Equal(t, tc.wantPeriodic, hasTool(names, "get_periodic_note"), names)
			require.True(t, hasTool(names, "search_vault"), names)
			require.True(t, hasTool(names, "search_vault_simple"), names)
			require.True(t, hasTool(names, "search_vault_local"), names)
			for _, banned := range []string{"vault_copy", "vault_get_document_map"} {
				require.False(t, hasTool(names, banned), banned)
			}

			var searchDesc string
			for tool, err := range cs.Tools(ctx, nil) {
				require.NoError(t, err)
				if tool.Name == "search_vault" {
					searchDesc = tool.Description
					break
				}
			}
			mentionsDataview := strings.Contains(searchDesc, "Dataview DQL") && !strings.Contains(searchDesc, "removed in Local REST API 4.0")
			if tc.wantDataview {
				require.True(t, strings.Contains(searchDesc, "Dataview"), searchDesc)
			} else {
				require.False(t, mentionsDataview, searchDesc)
				require.Contains(t, searchDesc, "JsonLogic")
			}
		})
	}
}

func TestMoveVaultFileTool(t *testing.T) {
	var gotMethod, gotDest, gotOverwrite string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotDest = r.Header.Get("Destination")
		gotOverwrite = r.Header.Get("Allow-Overwrite")
		w.Header().Set("Content-Location", "archive/a.md")
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())
	ctx, cs := mcpSessionWithVersion(t, cli, "4.1.7")

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "move_vault_file",
		Arguments: map[string]any{
			"filename":    "notes/a.md",
			"destination": "archive/",
		},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Content[0].(*mcp.TextContent).Text, "archive/a.md")
	require.Equal(t, "MOVE", gotMethod)
	require.Equal(t, "archive/", gotDest)
	require.Equal(t, "false", gotOverwrite)
}

func TestHiddenMoveVaultFileCallErrors(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())
	ctx, cs := mcpSessionWithVersion(t, cli, "3.6.1")

	_, err = cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "move_vault_file",
		Arguments: map[string]any{
			"filename":    "a.md",
			"destination": "b.md",
		},
	})
	require.Error(t, err)
	require.NotContains(t, err.Error(), "404")
	require.NotContains(t, err.Error(), "405")
}

func TestSearchVaultRejectsDataviewOn4x(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected HTTP %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())
	ctx, cs := mcpSessionWithVersion(t, cli, "4.1.7")

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "search_vault",
		Arguments: map[string]any{
			"queryType": "dataview",
			"query":     "LIST FROM #tag",
		},
	})
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Content[0].(*mcp.TextContent).Text, "queryType=dataview")
}

func TestGetVaultFileJSONLinksBacklinks(t *testing.T) {
	note := `{"content":"hi","frontmatter":{},"path":"Notes/j.md","stat":{"ctime":1,"mtime":1,"size":2},"tags":[],"links":["Other"],"backlinks":["Index"]}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "application/vnd.olrapi.note+json", r.Header.Get("Accept"))
		_, _ = w.Write([]byte(note))
	}))
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())
	ctx, cs := mcpSessionWithVersion(t, cli, "4.1.7")

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_vault_file",
		Arguments: map[string]any{"filename": "Notes/j.md", "format": "json"},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	structured := res.StructuredContent.(map[string]any)
	require.Equal(t, []any{"Other"}, structured["links"])
	require.Equal(t, []any{"Index"}, structured["backlinks"])

	var typed obsidian.NoteJSON
	require.NoError(t, json.Unmarshal([]byte(note), &typed))
	require.Equal(t, []string{"Other"}, typed.Links)
	require.Equal(t, []string{"Index"}, typed.Backlinks)
}

func TestNewMCPServerNilClient(t *testing.T) {
	// Override skips probe; nil client must not panic at RegisterAll.
	srv := NewMCPServer(nil, tools.Deps{
		Client:         nil,
		PromptsDir:     "Prompts",
		OmlxCheck:      false,
		RestAPIVersion: "3.6.1",
	})
	require.NotNil(t, srv)

	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	_, err := srv.Connect(ctx, st, nil)
	require.NoError(t, err)
	c := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := c.Connect(ctx, ct, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })
	names := toolNames(t, ctx, cs)
	require.False(t, hasTool(names, "move_vault_file"))
	require.True(t, hasTool(names, "fetch"))
}

func TestNewMCPServerProbeFailClosedOn404(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())

	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	srv := NewMCPServer(nil, tools.Deps{Client: cli, PromptsDir: "Prompts", OmlxCheck: false})
	_, err = srv.Connect(ctx, st, nil)
	require.NoError(t, err)
	c := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := c.Connect(ctx, ct, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })
	names := toolNames(t, ctx, cs)
	require.False(t, hasTool(names, "move_vault_file"))
	require.True(t, hasTool(names, "get_periodic_note"))
	require.True(t, hasTool(names, "search_vault"))
}

func TestResolveCapsEnvOverride(t *testing.T) {
	t.Setenv("REST_API_VERSION", "4.1.7")
	d := tools.ResolveCaps(tools.Deps{Client: nil})
	require.True(t, d.Caps.MoveVaultFile)
	require.False(t, d.Caps.RestDataviewDQL)
}
