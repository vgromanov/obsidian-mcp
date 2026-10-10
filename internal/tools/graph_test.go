package tools

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
)

func TestScopeAllowed(t *testing.T) {
	t.Parallel()
	require.True(t, scopeAllowed("", nil))
	require.True(t, scopeAllowed("Notes", nil))
	require.True(t, scopeAllowed("Notes", []string{"Notes"}))
	require.True(t, scopeAllowed("/Notes/a", []string{"Notes"}))
	require.True(t, scopeAllowed("Notes/", []string{"Notes"}))
	require.False(t, scopeAllowed("NotesExtra", []string{"Notes"}))
	require.False(t, scopeAllowed("", []string{"Notes"}))
	require.False(t, scopeAllowed("Other", []string{"Notes"}))
	require.True(t, scopeAllowed("Notes/a", []string{"Notes/"}))
	require.True(t, scopeAllowed("Notes/", []string{"Notes/"}))
	require.False(t, scopeAllowed("Notes", []string{"Notes/"}))
	require.True(t, scopeAllowed("Notes/a", []string{" Other ", " /Notes "}))
	require.False(t, scopeAllowed("Secret", []string{"", "Notes"}))
}

func graphSession(t *testing.T, handler http.HandlerFunc, policy GraphPolicy, set bool) (context.Context, *mcp.ClientSession) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	RegisterGraph(server, Deps{Client: cli, GraphPolicy: policy, GraphPolicySet: set})

	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	_, err = server.Connect(ctx, st, nil)
	require.NoError(t, err)
	client := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "0"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return ctx, session
}

func callGraph(t *testing.T, ctx context.Context, cs *mcp.ClientSession, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "graph_traverse", Arguments: args})
	require.NoError(t, err)
	return res
}

func graphText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	require.NotEmpty(t, res.Content)
	text, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	return text.Text
}

func graphOKHandler(t *testing.T, body *string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/graph/traverse/", r.URL.Path)
		b, _ := io.ReadAll(r.Body)
		if body != nil {
			*body = string(b)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"nodes":[{"id":"n1","path":"Notes/a.md","depth":0,"fields":{"status":"open"}}],"edges":[],"unresolved":[],"conflicts":[],"cycles":[],"truncated":false,"index_ready":true}`))
	}
}

func validGraphArgs() map[string]any {
	return map[string]any{
		"scope":    "Notes",
		"id_field": "id",
		"edges":    []any{map[string]any{"source": "depends_on"}},
		"start":    []any{"n1"},
	}
}

func TestGraphTraversePassthroughAndDefaults(t *testing.T) {
	var saw string
	ctx, cs := graphSession(t, graphOKHandler(t, &saw), GraphPolicy{}, false)
	args := validGraphArgs()
	args["edges"] = []any{map[string]any{"source": "$body", "sections": []any{"Definition"}, "embeds": true}}
	args["include"] = []any{"status", "$body"}
	args["max_depth"] = 3
	args["limit_nodes"] = 25
	delete(args, "direction")

	res := callGraph(t, ctx, cs, args)
	require.False(t, res.IsError)
	require.JSONEq(t, `{
		"scope":"Notes",
		"id_field":"id",
		"edges":[{"source":"$body","sections":["Definition"],"embeds":true}],
		"start":["n1"],
		"max_depth":3,
		"include":["status","$body"],
		"limit_nodes":25
	}`, saw)
	require.Contains(t, graphText(t, res), `"index_ready":true`)
	structured, ok := res.StructuredContent.(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, structured["index_ready"])

	var tool *mcp.Tool
	for listed, err := range cs.Tools(ctx, nil) {
		require.NoError(t, err)
		if listed.Name == "graph_traverse" {
			tool = listed
		}
	}
	require.NotNil(t, tool)
	require.NotNil(t, tool.Annotations)
	require.True(t, tool.Annotations.ReadOnlyHint)
	require.NotNil(t, tool.Annotations.DestructiveHint)
	require.False(t, *tool.Annotations.DestructiveHint)
	require.Contains(t, tool.Description, "$body")
	require.Contains(t, tool.Description, "section")
	require.Contains(t, tool.Description, "Omitting start")
	require.Contains(t, tool.Description, `include ["$body"]`)
	require.Contains(t, tool.Description, "allowlist")

	raw, err := json.Marshal(tool.InputSchema)
	require.NoError(t, err)
	var schema map[string]any
	require.NoError(t, json.Unmarshal(raw, &schema))
	assertSchemaPropertiesAreObjects(t, tool.Name, schema)

	saw = ""
	minimal := callGraph(t, ctx, cs, map[string]any{
		"scope": "Notes",
		"edges": []any{map[string]any{"source": "depends_on"}},
	})
	require.False(t, minimal.IsError, graphText(t, minimal))
	require.JSONEq(t, `{"scope":"Notes","edges":[{"source":"depends_on"}]}`, saw)
	require.Contains(t, tool.Description, "vault path")
	require.Contains(t, tool.Description, "the route uses out")
}

func TestGraphTraverseValidationSkipsPlugin(t *testing.T) {
	cases := []struct {
		name string
		edit func(map[string]any)
		want string
	}{
		{"missing id", func(a map[string]any) { a["id_field"] = "  " }, "`id_field` must be a non-empty string"},
		{"empty edges", func(a map[string]any) { a["edges"] = []any{} }, "edges must be a non-empty list"},
		{"blank source", func(a map[string]any) { a["edges"] = []any{map[string]any{"source": " "}} }, "each edge source must be a non-empty string"},
		{"blank section", func(a map[string]any) {
			a["edges"] = []any{map[string]any{"source": "depends_on", "sections": []any{" "}}}
		}, "edge sections must be a list of strings"},
		{"bad direction", func(a map[string]any) { a["direction"] = "sideways" }, "direction must be one of: out, in, both"},
		{"negative depth", func(a map[string]any) { a["max_depth"] = -1 }, "`max_depth` must be a non-negative integer or null"},
		{"zero limit", func(a map[string]any) { a["limit_nodes"] = 0 }, "`limit_nodes` must be a positive integer"},
		{"blank include", func(a map[string]any) { a["include"] = []any{"status", ""} }, "`include` must be a list of field names"},
		{"empty start", func(a map[string]any) { a["start"] = []any{} }, "`start` must list at least one id or path"},
		{"blank start", func(a map[string]any) { a["start"] = []any{"  "} }, "`start` must list at least one id or path"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cs := graphSession(t, func(w http.ResponseWriter, r *http.Request) {
				t.Fatalf("plugin called on %s %s", r.Method, r.URL.Path)
			}, DefaultGraphPolicy(), true)
			args := validGraphArgs()
			tc.edit(args)
			res := callGraph(t, ctx, cs, args)
			require.True(t, res.IsError)
			require.Contains(t, graphText(t, res), tc.want)
		})
	}
}

func TestGraphTraverse400Passthrough(t *testing.T) {
	const parser = "scope has no notes"
	ctx, cs := graphSession(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"Bad Request\n` + parser + `","errorCode":40000}`))
	}, DefaultGraphPolicy(), true)
	res := callGraph(t, ctx, cs, validGraphArgs())
	require.True(t, res.IsError)
	text := graphText(t, res)
	require.Contains(t, text, parser)
	require.NotContains(t, text, "/graph/traverse/")
}

func TestGraphTraversePolicyKnobs(t *testing.T) {
	t.Run("scope", func(t *testing.T) {
		ctx, cs := graphSession(t, func(http.ResponseWriter, *http.Request) {
			t.Fatal("plugin called")
		}, GraphPolicy{ScopePrefixes: []string{"Projects/"}, AllowBody: true, AllowFullExport: true}, true)
		args := validGraphArgs()
		args["scope"] = "Notes"
		res := callGraph(t, ctx, cs, args)
		require.True(t, res.IsError)
		require.Contains(t, graphText(t, res), "OBSIDIAN_GRAPH_SCOPE_ALLOWLIST")
	})

	t.Run("limit explicit", func(t *testing.T) {
		ctx, cs := graphSession(t, func(http.ResponseWriter, *http.Request) {
			t.Fatal("plugin called")
		}, GraphPolicy{MaxLimitNodes: 500, AllowBody: true, AllowFullExport: true}, true)
		args := validGraphArgs()
		args["limit_nodes"] = 501
		res := callGraph(t, ctx, cs, args)
		require.True(t, res.IsError)
		require.Contains(t, graphText(t, res), "OBSIDIAN_GRAPH_MAX_LIMIT_NODES")
		require.Contains(t, graphText(t, res), "501")
	})

	t.Run("limit default", func(t *testing.T) {
		ctx, cs := graphSession(t, func(http.ResponseWriter, *http.Request) {
			t.Fatal("plugin called")
		}, GraphPolicy{MaxLimitNodes: 1999, AllowBody: true, AllowFullExport: true}, true)
		res := callGraph(t, ctx, cs, validGraphArgs())
		require.True(t, res.IsError)
		text := graphText(t, res)
		require.Contains(t, text, "OBSIDIAN_GRAPH_MAX_LIMIT_NODES")
		require.Contains(t, text, "2000")
	})

	t.Run("limit at cap", func(t *testing.T) {
		var saw string
		ctx, cs := graphSession(t, graphOKHandler(t, &saw), GraphPolicy{MaxLimitNodes: 2000, AllowBody: true, AllowFullExport: true}, true)
		res := callGraph(t, ctx, cs, validGraphArgs())
		require.False(t, res.IsError)
		require.NotContains(t, saw, "limit_nodes")
	})

	t.Run("body", func(t *testing.T) {
		ctx, cs := graphSession(t, func(http.ResponseWriter, *http.Request) {
			t.Fatal("plugin called")
		}, GraphPolicy{AllowBody: false, AllowFullExport: true}, true)
		args := validGraphArgs()
		args["include"] = []any{"status", "$body"}
		res := callGraph(t, ctx, cs, args)
		require.True(t, res.IsError)
		require.Contains(t, graphText(t, res), "OBSIDIAN_GRAPH_ALLOW_BODY")
	})

	t.Run("full export", func(t *testing.T) {
		ctx, cs := graphSession(t, func(http.ResponseWriter, *http.Request) {
			t.Fatal("plugin called")
		}, GraphPolicy{AllowBody: true, AllowFullExport: false}, true)
		args := validGraphArgs()
		delete(args, "start")
		res := callGraph(t, ctx, cs, args)
		require.True(t, res.IsError)
		require.Contains(t, graphText(t, res), "OBSIDIAN_GRAPH_ALLOW_FULL_EXPORT")
	})

	t.Run("invalid cap", func(t *testing.T) {
		ctx, cs := graphSession(t, func(http.ResponseWriter, *http.Request) {
			t.Fatal("plugin called")
		}, GraphPolicy{MaxLimitNodesInvalid: true, AllowBody: true, AllowFullExport: true}, true)
		res := callGraph(t, ctx, cs, validGraphArgs())
		require.True(t, res.IsError)
		require.Contains(t, graphText(t, res), "OBSIDIAN_GRAPH_MAX_LIMIT_NODES")
	})

	t.Run("allowed scope still calls", func(t *testing.T) {
		var saw string
		ctx, cs := graphSession(t, graphOKHandler(t, &saw), GraphPolicy{
			ScopePrefixes:   []string{"Notes/"},
			MaxLimitNodes:   50,
			AllowBody:       false,
			AllowFullExport: false,
		}, true)
		args := validGraphArgs()
		args["scope"] = "Notes/Meetings"
		args["limit_nodes"] = 50
		args["include"] = []any{"status"}
		args["direction"] = "both"
		args["max_depth"] = 0
		res := callGraph(t, ctx, cs, args)
		require.False(t, res.IsError, graphText(t, res))
		require.Contains(t, saw, `"direction":"both"`)
		require.Contains(t, saw, `"max_depth":0`)
	})
}

func TestGraphTraverseAdvertisedOnlyWhenRoutePresent(t *testing.T) {
	t.Setenv("REST_API_VERSION", "")
	t.Setenv("OBSIDIAN_REST_API_VERSION", "")

	var gets int
	info := map[string]any{
		"versions": map[string]string{"self": "4.1.7"},
		"apiExtensions": []any{
			map[string]any{"routes": []any{
				map[string]any{"path": "/dataview/query/"},
				"/graph/traverse/",
			}},
		},
	}
	rawInfo, err := json.Marshal(info)
	require.NoError(t, err)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" && r.Method == http.MethodGet {
			gets++
			_, _ = w.Write(rawInfo)
			return
		}
		t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())

	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	RegisterAll(server, Deps{Client: cli, OmlxCheck: false})
	st, ct := mcp.NewInMemoryTransports()
	_, err = server.Connect(ctx, st, nil)
	require.NoError(t, err)
	client := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "0"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })

	var names []string
	for tool, err := range session.Tools(ctx, nil) {
		require.NoError(t, err)
		names = append(names, tool.Name)
	}
	require.Contains(t, names, "graph_traverse")
	require.Equal(t, 1, gets, "plugin routes share one GET / probe")

	absentInfo, err := json.Marshal(map[string]any{
		"versions": map[string]string{"self": "4.1.7"},
		"apiExtensions": []any{
			map[string]any{"routes": []any{map[string]any{"path": "/dataview/query/"}}},
		},
	})
	require.NoError(t, err)
	ts2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" && r.Method == http.MethodGet {
			_, _ = w.Write(absentInfo)
			return
		}
		t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(ts2.Close)
	u2, err := url.Parse(ts2.URL)
	require.NoError(t, err)
	cli2 := obsidian.NewClientFromURL(u2, "secret", ts2.Client())
	server2 := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	RegisterAll(server2, Deps{Client: cli2, OmlxCheck: false})
	st2, ct2 := mcp.NewInMemoryTransports()
	_, err = server2.Connect(ctx, st2, nil)
	require.NoError(t, err)
	client2 := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "0"}, nil)
	session2, err := client2.Connect(ctx, ct2, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session2.Close() })
	var names2 []string
	for tool, err := range session2.Tools(ctx, nil) {
		require.NoError(t, err)
		names2 = append(names2, tool.Name)
	}
	require.NotContains(t, names2, "graph_traverse")
	require.Contains(t, names2, "search_vault")
}
