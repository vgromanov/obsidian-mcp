package mcpapp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/vgromanov/obsidian-mcp/internal/obsidian"
	"github.com/vgromanov/obsidian-mcp/internal/tools"
)

func mcpSessionFromProbe(t *testing.T, cli *obsidian.Client) (context.Context, *mcp.ClientSession) {
	t.Helper()
	t.Setenv("REST_API_VERSION", "")
	t.Setenv("OBSIDIAN_REST_API_VERSION", "")
	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	d := testDeps(cli)
	d.RestAPIVersion = ""
	srv := NewMCPServer(nil, d)
	_, err := srv.Connect(ctx, st, nil)
	require.NoError(t, err)
	c := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := c.Connect(ctx, ct, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })
	return ctx, cs
}

func serverInfo(version string, withRoute bool) []byte {
	body := map[string]any{
		"versions": map[string]string{"self": version},
	}
	if withRoute {
		body["apiExtensions"] = []any{
			map[string]any{
				"id": "local-smart-lookup",
				"routes": []any{
					map[string]any{"path": "/dataview/query/", "authenticated": true},
				},
			},
		}
	}
	raw, _ := json.Marshal(body)
	return raw
}

func TestSearchVaultDataviewBelow40UsesSearch(t *testing.T) {
	var sawPath, sawType, sawBody string
	info := serverInfo("3.6.1", true)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" && r.Method == http.MethodGet {
			_, _ = w.Write(info)
			return
		}
		sawPath = r.URL.Path
		sawType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		sawBody = string(b)
		_, _ = w.Write([]byte(`[{"filename":"Notes/a.md"}]`))
	}))
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())
	ctx, cs := mcpSessionFromProbe(t, cli)

	var searchDesc string
	for tool, err := range cs.Tools(ctx, nil) {
		require.NoError(t, err)
		if tool.Name == "search_vault" {
			searchDesc = tool.Description
		}
	}
	require.Contains(t, searchDesc, "Dataview DQL")
	require.Contains(t, searchDesc, `TABLE status FROM "Projects" WHERE status = "active"`)
	require.Contains(t, searchDesc, "LIST FROM #research")

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "search_vault",
		Arguments: map[string]any{
			"queryType": "dataview",
			"query":     `LIST FROM "Notes"`,
			"limit":     10,
		},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Equal(t, "/search/", sawPath)
	require.Equal(t, "application/vnd.olrapi.dataview.dql+txt", sawType)
	require.Equal(t, `LIST FROM "Notes"`, sawBody)
}

func TestSearchVaultDataviewUsesPluginRoute(t *testing.T) {
	var posts []string
	var dqlBody string
	var dqlType string
	info := serverInfo("4.1.7", true)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" && r.Method == http.MethodGet {
			_, _ = w.Write(info)
			return
		}
		if r.Method == http.MethodPost {
			posts = append(posts, r.URL.Path)
			if r.URL.Path == "/dataview/query/" {
				dqlType = r.Header.Get("Content-Type")
				b, _ := io.ReadAll(r.Body)
				dqlBody = string(b)
				_, _ = w.Write([]byte(`{"type":"list","items":["Notes/a.md"],"truncated":false,"index_ready":true}`))
				return
			}
		}
		t.Fatalf("unexpected HTTP %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())
	ctx, cs := mcpSessionFromProbe(t, cli)

	var searchDesc string
	for tool, err := range cs.Tools(ctx, nil) {
		require.NoError(t, err)
		if tool.Name == "search_vault" {
			searchDesc = tool.Description
		}
	}
	require.Contains(t, searchDesc, "POST /dataview/query/")
	require.Contains(t, searchDesc, `TABLE status FROM "Projects" WHERE status = "active"`)
	require.Contains(t, searchDesc, "LIST FROM #research")
	require.NotContains(t, searchDesc, "removed in Local REST API 4.0")

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "search_vault",
		Arguments: map[string]any{
			"queryType": "dataview",
			"query":     `LIST FROM "Notes"`,
			"limit":     10,
		},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Equal(t, []string{"/dataview/query/"}, posts)
	require.Equal(t, "application/json", dqlType)
	require.JSONEq(t, `{"query":"LIST FROM \"Notes\"","limit":10}`, dqlBody)
	require.Contains(t, res.Content[0].(*mcp.TextContent).Text, `"type":"list"`)
}

func TestSearchVaultDataviewPluginRouteOmitsLimit(t *testing.T) {
	var dqlBody string
	info := serverInfo("5.0.3", true)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" && r.Method == http.MethodGet {
			_, _ = w.Write(info)
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/dataview/query/" {
			b, _ := io.ReadAll(r.Body)
			dqlBody = string(b)
			_, _ = w.Write([]byte(`{"type":"table","headers":["File"],"rows":[]}`))
			return
		}
		t.Fatalf("unexpected HTTP %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())
	ctx, cs := mcpSessionFromProbe(t, cli)

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "search_vault",
		Arguments: map[string]any{
			"queryType": "dataview",
			"query":     "TABLE file.name",
		},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.JSONEq(t, `{"query":"TABLE file.name"}`, dqlBody)
}

func TestSearchVaultDataviewMissingPluginErrors(t *testing.T) {
	info := serverInfo("4.0.0", false)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" && r.Method == http.MethodGet {
			_, _ = w.Write(info)
			return
		}
		t.Fatalf("unexpected HTTP %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())
	ctx, cs := mcpSessionFromProbe(t, cli)

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "search_vault",
		Arguments: map[string]any{
			"queryType": "dataview",
			"query":     `LIST FROM "Notes"`,
		},
	})
	require.NoError(t, err)
	require.True(t, res.IsError)
	text := res.Content[0].(*mcp.TextContent).Text
	require.Contains(t, text, "queryType=dataview")
	require.Contains(t, text, "POST /dataview/query/")
}

func TestSearchVaultDataview400Passthrough(t *testing.T) {
	const parser = "Expected one of: TABLE, LIST, TASK, CALENDAR at line 1, column 4"
	info := serverInfo("4.1.7", true)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" && r.Method == http.MethodGet {
			_, _ = w.Write(info)
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/dataview/query/" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"ok":false,"error":"` + parser + `","status":400}`))
			return
		}
		t.Fatalf("unexpected HTTP %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())
	ctx, cs := mcpSessionFromProbe(t, cli)

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "search_vault",
		Arguments: map[string]any{
			"queryType": "dataview",
			"query":     "not dql",
		},
	})
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Content[0].(*mcp.TextContent).Text, parser)
	require.NotContains(t, res.Content[0].(*mcp.TextContent).Text, "POST /dataview/query/")
}

func TestResolveCapsVersionOverrideSkipsPluginProbe(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected HTTP %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())
	d := tools.ResolveCaps(tools.Deps{Client: cli, RestAPIVersion: "4.1.7"})
	require.False(t, d.Caps.PluginDataviewQuery)
	require.False(t, d.Caps.PluginGraphTraverse)
	require.False(t, d.Caps.RestDataviewDQL)
}
