package mcpapp

import (
	"context"
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

func testDeps(cli *obsidian.Client) tools.Deps {
	return tools.Deps{
		Client:     cli,
		PromptsDir: "Prompts",
		OmlxCheck:  false,
	}
}

func TestMCPToolGetServerInfo(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/", r.URL.Path)
		require.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())

	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	srv := NewMCPServer(nil, testDeps(cli))
	_, err = srv.Connect(ctx, st, nil)
	require.NoError(t, err)

	c := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := c.Connect(ctx, ct, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_server_info"})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.NotEmpty(t, res.Content)
	txt := res.Content[0].(*mcp.TextContent).Text
	require.Contains(t, txt, `"status"`)
	require.NotNil(t, res.StructuredContent)
	structured, ok := res.StructuredContent.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "ok", structured["status"])
}

func TestMCPPromptsListDynamic(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/vault/Prompts/":
			_, _ = w.Write([]byte(`{"files":["Hello.md"]}`))
		case "/vault/Prompts/Hello.md":
			require.Equal(t, "application/vnd.olrapi.note+json", r.Header.Get("Accept"))
			note := `{"content":"<% tp.mcpTools.prompt(\"topic\", \"desc\") %>\nbody","frontmatter":{"tags":["mcp-tools-prompt"],"description":"Hi"},"path":"Prompts/Hello.md","stat":{"ctime":1,"mtime":1,"size":10},"tags":["mcp-tools-prompt"]}`
			_, _ = w.Write([]byte(note))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())

	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	srv := NewMCPServer(nil, testDeps(cli))
	_, err = srv.Connect(ctx, st, nil)
	require.NoError(t, err)

	c := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := c.Connect(ctx, ct, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })

	var names []string
	for p, err := range cs.Prompts(ctx, nil) {
		require.NoError(t, err)
		names = append(names, p.Name)
	}
	require.Equal(t, []string{"Hello.md"}, names)
}

func TestMCPToolListTags(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/tags/", r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tags":[{"name":"project","count":3}]}`))
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())

	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	srv := NewMCPServer(nil, testDeps(cli))
	_, err = srv.Connect(ctx, st, nil)
	require.NoError(t, err)

	c := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := c.Connect(ctx, ct, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_tags"})
	require.NoError(t, err)
	require.False(t, res.IsError)
	txt := res.Content[0].(*mcp.TextContent).Text
	require.Contains(t, txt, `"name"`)
	require.Contains(t, txt, `"count"`)
	require.NotNil(t, res.StructuredContent)
}

func TestMCPToolExecuteCommand(t *testing.T) {
	var sawPath, ct string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		require.Equal(t, http.MethodPost, r.Method)
		ct = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		require.Equal(t, "{}", string(b))
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())

	ctx := context.Background()
	ctM, st := mcp.NewInMemoryTransports()
	srv := NewMCPServer(nil, testDeps(cli))
	_, err = srv.Connect(ctx, st, nil)
	require.NoError(t, err)

	c := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := c.Connect(ctx, ctM, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "execute_command",
		Arguments: map[string]any{"commandId": "editor:save-file"},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, sawPath, "/commands/")
	require.Contains(t, sawPath, "editor:save-file")
	require.Equal(t, "application/json", ct)

	txt := res.Content[0].(*mcp.TextContent).Text
	require.Contains(t, txt, "executed successfully")
}

func TestMCPToolPatchPeriodicNoteInvalidPeriod(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called for invalid period")
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())

	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	srv := NewMCPServer(nil, testDeps(cli))
	_, err = srv.Connect(ctx, st, nil)
	require.NoError(t, err)

	c := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := c.Connect(ctx, ct, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "patch_periodic_note",
		Arguments: map[string]any{
			"period":     "hourly",
			"operation":  "append",
			"targetType": "heading",
			"target":     "Log",
			"content":    "x",
		},
	})
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Content[0].(*mcp.TextContent).Text, "invalid period")
}

func TestMCPToolPatchPeriodicNoteHeaders(t *testing.T) {
	var op, tgtType, tgt string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/periodic/daily/", r.URL.Path)
		require.Equal(t, http.MethodPatch, r.Method)
		op = r.Header.Get("Operation")
		tgtType = r.Header.Get("Target-Type")
		tgt = r.Header.Get("Target")
		_, _ = w.Write([]byte(`patched`))
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())

	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	srv := NewMCPServer(nil, testDeps(cli))
	_, err = srv.Connect(ctx, st, nil)
	require.NoError(t, err)

	c := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := c.Connect(ctx, ct, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "patch_periodic_note",
		Arguments: map[string]any{
			"period":     "daily",
			"operation":  "append",
			"targetType": "heading",
			"target":     "Log",
			"content":    "line",
		},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Equal(t, "append", op)
	require.Equal(t, "heading", tgtType)
	require.Equal(t, "Log", tgt)

	txt := res.Content[0].(*mcp.TextContent).Text
	require.Contains(t, txt, "patched successfully")
	require.Contains(t, res.Content[1].(*mcp.TextContent).Text, "patched")
}

func TestMCPToolGetTagFilesViaJsonLogic(t *testing.T) {
	var sawPath, sawCT, sawBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		require.Equal(t, http.MethodPost, r.Method)
		sawCT = r.Header.Get("Content-Type")
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		sawBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"filename":"Notes/A.md","result":true}]`))
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())

	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	srv := NewMCPServer(nil, testDeps(cli))
	_, err = srv.Connect(ctx, st, nil)
	require.NoError(t, err)

	c := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := c.Connect(ctx, ct, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_tag_files",
		Arguments: map[string]any{"tag": "#project"},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Equal(t, "/search/", sawPath)
	require.Equal(t, "application/vnd.olrapi.jsonlogic+json", sawCT)
	require.Equal(t, `{"in":["project",{"var":"tags"}]}`, sawBody)
	require.Contains(t, res.Content[0].(*mcp.TextContent).Text, `"filename"`)
	require.NotNil(t, res.StructuredContent)
	structured := res.StructuredContent.(map[string]any)
	data, ok := structured["data"].([]any)
	require.True(t, ok)
	require.Len(t, data, 1)
}

func TestMCPToolFetchStructuredPagination(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body><p>Hello</p></body></html>"))
	}))
	t.Cleanup(ts.Close)

	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	srv := NewMCPServer(nil, testDeps(nil))
	_, err := srv.Connect(ctx, st, nil)
	require.NoError(t, err)

	c := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := c.Connect(ctx, ct, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "fetch",
		Arguments: map[string]any{"url": ts.URL},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Len(t, res.Content, 1)
	require.NotContains(t, res.Content[0].(*mcp.TextContent).Text, "Pagination:")

	structured := res.StructuredContent.(map[string]any)
	pagination, ok := structured["pagination"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(0), pagination["startIndex"])
	require.Equal(t, true, pagination["hasMore"])
}

func TestMCPToolSearchVaultLocal(t *testing.T) {
	var sawPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		require.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"path":"A.md","text":"chunk","score":0.9}]}`))
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())

	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	srv := NewMCPServer(nil, testDeps(cli))
	_, err = srv.Connect(ctx, st, nil)
	require.NoError(t, err)

	c := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := c.Connect(ctx, ct, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "search_vault_local",
		Arguments: map[string]any{
			"query":       "local-first AI",
			"tags":        []any{"research"},
			"frontmatter": map[string]any{"status": "active"},
			"limit":       float64(5),
		},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Equal(t, "/local-smart-lookup/search/", sawPath)
	structured := res.StructuredContent.(map[string]any)
	results, ok := structured["results"].([]any)
	require.True(t, ok)
	require.Len(t, results, 1)
}

func TestMCPToolSearchVaultLocalOmlxPreflightBlocks(t *testing.T) {
	obsidianCalled := false
	obsTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		obsidianCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(obsTS.Close)

	omlxTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(omlxTS.Close)

	u, err := url.Parse(obsTS.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", obsTS.Client())

	deps := testDeps(cli)
	deps.OmlxCheck = true
	deps.OmlxBaseURL = omlxTS.URL + "/v1"

	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	srv := NewMCPServer(nil, deps)
	_, err = srv.Connect(ctx, st, nil)
	require.NoError(t, err)

	c := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := c.Connect(ctx, ct, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "search_vault_local",
		Arguments: map[string]any{"query": "test"},
	})
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.False(t, obsidianCalled)
}

func TestMCPToolSearchVaultLocalOmlxPreflightPasses(t *testing.T) {
	var sawPath string
	omlxTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/models", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(omlxTS.Close)

	obsTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	t.Cleanup(obsTS.Close)

	u, err := url.Parse(obsTS.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", obsTS.Client())

	deps := testDeps(cli)
	deps.OmlxCheck = true
	deps.OmlxBaseURL = omlxTS.URL + "/v1"

	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	srv := NewMCPServer(nil, deps)
	_, err = srv.Connect(ctx, st, nil)
	require.NoError(t, err)

	c := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := c.Connect(ctx, ct, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "search_vault_local",
		Arguments: map[string]any{"query": "test"},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Equal(t, "/local-smart-lookup/search/", sawPath)
}
