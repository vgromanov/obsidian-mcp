package mcpapp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/vgromanov/obsidian-mcp/internal/obsidian"
	"github.com/vgromanov/obsidian-mcp/internal/tools"
)

func testDeps(cli *obsidian.Client) tools.Deps {
	return tools.Deps{
		Client:         cli,
		PromptsDir:     "Prompts",
		OmlxCheck:      false,
		RestAPIVersion: "3.6.1", // skip live GET / probe in unit tests
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

func TestMCPToolListFrontmatterKeys(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/frontmatter_keys/", r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"name":"workspace","count":3,"type":"text"}]`))
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

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_frontmatter_keys"})
	require.NoError(t, err)
	require.False(t, res.IsError)
	txt := res.Content[0].(*mcp.TextContent).Text
	require.Contains(t, txt, `"workspace"`)
	require.Contains(t, txt, `"count"`)
	require.Contains(t, txt, `"type"`)
	require.NotNil(t, res.StructuredContent)
}

func TestMCPToolGetFrontmatterKeyFiles(t *testing.T) {
	var sawPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		require.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"filename":"Notes/A.md"}]`))
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
		Name:      "get_frontmatter_key_files",
		Arguments: map[string]any{"name": "workspace"},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Equal(t, "/frontmatter_keys/workspace/", sawPath)
	require.Contains(t, res.Content[0].(*mcp.TextContent).Text, `"filename"`)
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
	var patchVer string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/periodic/daily/", r.URL.Path)
		require.Equal(t, http.MethodPatch, r.Method)
		op = r.Header.Get("Operation")
		tgtType = r.Header.Get("Target-Type")
		tgt = r.Header.Get("Target")
		patchVer = r.Header.Get("Markdown-Patch-Version")
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
	require.Equal(t, "1", patchVer)

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

func mcpSession(t *testing.T, cli *obsidian.Client) (context.Context, *mcp.ClientSession) {
	t.Helper()
	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	srv := NewMCPServer(nil, testDeps(cli))
	_, err := srv.Connect(ctx, st, nil)
	require.NoError(t, err)
	c := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := c.Connect(ctx, ct, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })
	return ctx, cs
}

func TestMCPToolSIHealth(t *testing.T) {
	var sawPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		require.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"chunks":2}`))
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())
	ctx, cs := mcpSession(t, cli)

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "si_health"})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Equal(t, "/si/health/", sawPath)
	require.Contains(t, res.Content[0].(*mcp.TextContent).Text, `"ok"`)
}

func TestMCPToolSIKnnRejectsMissingType(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		t.Fatal("HTTP should not be called")
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())
	ctx, cs := mcpSession(t, cli)

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "si_knn",
		Arguments: map[string]any{
			"text":  "should not embed",
			"where": "workspace = 'x'",
		},
	})
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.False(t, called)
	require.Contains(t, res.Content[0].(*mcp.TextContent).Text, "corpus-type")
}

func TestMCPToolSICountNeighborsRequiresThreshold(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("HTTP should not be called")
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())
	ctx, cs := mcpSession(t, cli)

	_, err = cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "si_count_neighbors",
		Arguments: map[string]any{
			"chunk_id": "p#h#0",
			"group_by": "uuid",
			"where":    "type = 'note'",
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "threshold")
}

func TestMCPToolSIKnnTextPathNoVectorEcho(t *testing.T) {
	var paths []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/si/embed_text/":
			require.Contains(t, string(body), `"texts"`)
			_, _ = w.Write([]byte(`{"vectors":[[0.1,0.2]],"embed_dim":2}`))
		case "/si/knn/":
			var got map[string]any
			require.NoError(t, json.Unmarshal(body, &got))
			require.Contains(t, got, "vector")
			require.NotContains(t, got, "text")
			_, _ = w.Write([]byte(`{"hits":[{"chunk_id":"c","distance":0.05}],"k":5,"metric":"cosine"}`))
		default:
			t.Fatalf("unexpected %s", r.URL.Path)
		}
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())
	ctx, cs := mcpSession(t, cli)

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "si_knn",
		Arguments: map[string]any{
			"text":  "find similar patterns",
			"where": "type = 'session-summary'",
			"k":     float64(5),
		},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Equal(t, []string{"/si/embed_text/", "/si/knn/"}, paths)
	txt := res.Content[0].(*mcp.TextContent).Text
	require.Contains(t, txt, `"hits"`)
	require.NotContains(t, txt, `"vectors"`)
	require.NotContains(t, txt, `"query_vector"`)
	require.NotContains(t, txt, "0.1")
}

func TestMCPToolSIKnnRejectsQueryXOR(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("HTTP should not be called")
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())
	ctx, cs := mcpSession(t, cli)

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "si_knn",
		Arguments: map[string]any{
			"text":     "a",
			"chunk_id": "p#h#0",
			"where":    "type = 'note'",
		},
	})
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Content[0].(*mcp.TextContent).Text, "exactly one")
}

func TestMCPToolSIQueryMetadataRejectsOffset(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("HTTP should not be called")
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())
	ctx, cs := mcpSession(t, cli)

	_, err = cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "si_query_metadata",
		Arguments: map[string]any{
			"where":  "type = 'note'",
			"fields": []any{"path"},
			"offset": float64(10),
		},
	})
	// offset is not in the input schema (any→JSON Schema true broke Grok Bot);
	// additionalProperties:false rejects it before the handler runs.
	require.Error(t, err)
	require.Contains(t, err.Error(), "offset")
}

func TestMCPToolSIGetVectorsRejectsMissingType(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("HTTP should not be called")
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())
	ctx, cs := mcpSession(t, cli)

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "si_get_vectors",
		Arguments: map[string]any{"where": "workspace = 'x'"},
	})
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Content[0].(*mcp.TextContent).Text, "corpus-type")
}

func TestMCPToolSIFilterValidate(t *testing.T) {
	var sawPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		require.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sql":"type = 'note'","row_count":1,"sample":[]}`))
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())
	ctx, cs := mcpSession(t, cli)

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "si_filter_validate",
		Arguments: map[string]any{"where": "type = 'note'"},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Equal(t, "/si/filter/validate/", sawPath)
	require.Contains(t, res.Content[0].(*mcp.TextContent).Text, `"row_count"`)
}

func TestMCPToolGetVaultFileSmallNoTruncation(t *testing.T) {
	const body = "# small note\nhello world\n"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Contains(t, r.URL.Path, "small.md")
		require.Equal(t, "text/markdown", r.Header.Get("Accept"))
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())
	ctx, cs := mcpSession(t, cli)

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_vault_file",
		Arguments: map[string]any{"filename": "small.md"},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	txt := res.Content[0].(*mcp.TextContent).Text
	require.Equal(t, body, txt)
	require.NotContains(t, txt, "file too large")
	structured := res.StructuredContent.(map[string]any)
	pagination := structured["pagination"].(map[string]any)
	require.Equal(t, false, pagination["hasMore"])
	require.Equal(t, float64(len(body)), pagination["totalLength"])
}

func TestMCPToolGetVaultFileLargeDefaultTruncation(t *testing.T) {
	large := strings.Repeat("A", 40*1024)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "text/markdown", r.Header.Get("Accept"))
		_, _ = w.Write([]byte(large))
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())
	ctx, cs := mcpSession(t, cli)

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_vault_file",
		Arguments: map[string]any{"filename": "Notes/large.md"},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	txt := res.Content[0].(*mcp.TextContent).Text
	require.Contains(t, txt, "file too large (40960 bytes)")
	require.Contains(t, txt, "startIndex=32768")
	require.Less(t, len(txt), 50*1024)
	require.True(t, strings.HasPrefix(txt, strings.Repeat("A", 32768)))
	structured := res.StructuredContent.(map[string]any)
	pagination := structured["pagination"].(map[string]any)
	require.Equal(t, true, pagination["hasMore"])
	require.Equal(t, float64(40960), pagination["totalLength"])
	require.Equal(t, float64(0), pagination["startIndex"])
	require.Equal(t, float64(32768), pagination["endIndex"])
}

func TestMCPToolGetVaultFileMaxLengthZeroUsesDefault(t *testing.T) {
	large := strings.Repeat("C", 40*1024)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(large))
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())
	ctx, cs := mcpSession(t, cli)

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_vault_file",
		Arguments: map[string]any{
			"filename":  "Notes/zero-max.md",
			"maxLength": float64(0),
		},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	txt := res.Content[0].(*mcp.TextContent).Text
	require.True(t, strings.HasPrefix(txt, strings.Repeat("C", 32768)))
	require.Contains(t, txt, "startIndex=32768")
	require.NotContains(t, txt, "startIndex=0\n")
	require.Less(t, len(txt), 50*1024)
	structured := res.StructuredContent.(map[string]any)
	pagination := structured["pagination"].(map[string]any)
	require.Equal(t, true, pagination["hasMore"])
	require.Equal(t, float64(0), pagination["startIndex"])
	require.Equal(t, float64(32768), pagination["endIndex"])
	require.Equal(t, float64(40960), pagination["totalLength"])
	// Must not return an empty stall page.
	content, _ := structured["content"].(string)
	require.Len(t, content, 32768)
}

func TestMCPToolGetVaultFileSecondPage(t *testing.T) {
	large := strings.Repeat("B", 1000)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(large))
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())
	ctx, cs := mcpSession(t, cli)

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_vault_file",
		Arguments: map[string]any{
			"filename":   "Notes/paged.md",
			"maxLength":  float64(100),
			"startIndex": float64(200),
		},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	txt := res.Content[0].(*mcp.TextContent).Text
	require.True(t, strings.HasPrefix(txt, strings.Repeat("B", 100)))
	require.Contains(t, txt, "startIndex=300")
	structured := res.StructuredContent.(map[string]any)
	pagination := structured["pagination"].(map[string]any)
	require.Equal(t, float64(200), pagination["startIndex"])
	require.Equal(t, float64(300), pagination["endIndex"])
	require.Equal(t, true, pagination["hasMore"])
}

func TestMCPToolGetVaultFileFormatJSONSmall(t *testing.T) {
	note := `{"content":"hi","frontmatter":{},"path":"Notes/j.md","stat":{"ctime":1,"mtime":1,"size":2},"tags":[]}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "application/vnd.olrapi.note+json", r.Header.Get("Accept"))
		w.Header().Set("Content-Type", "application/vnd.olrapi.note+json")
		_, _ = w.Write([]byte(note))
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())
	ctx, cs := mcpSession(t, cli)

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_vault_file",
		Arguments: map[string]any{"filename": "Notes/j.md", "format": "json"},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.NotContains(t, res.Content[0].(*mcp.TextContent).Text, "file too large")
	structured := res.StructuredContent.(map[string]any)
	require.Equal(t, "hi", structured["content"])
	require.Equal(t, "Notes/j.md", structured["path"])
}

func TestMCPToolGetVaultFileFormatJSONLargeTruncates(t *testing.T) {
	bigContent := strings.Repeat("Z", 40*1024)
	noteObj := map[string]any{
		"content":     bigContent,
		"frontmatter": map[string]any{},
		"path":        "Notes/bigjson.md",
		"stat":        map[string]any{"ctime": 1, "mtime": 1, "size": len(bigContent)},
		"tags":        []any{},
	}
	raw, err := json.Marshal(noteObj)
	require.NoError(t, err)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "application/vnd.olrapi.note+json", r.Header.Get("Accept"))
		_, _ = w.Write(raw)
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())
	ctx, cs := mcpSession(t, cli)

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_vault_file",
		Arguments: map[string]any{"filename": "Notes/bigjson.md", "format": "json"},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	txt := res.Content[0].(*mcp.TextContent).Text
	require.Contains(t, txt, "file too large")
	require.Contains(t, txt, "startIndex=")
	require.Less(t, len(txt), 50*1024)
	structured := res.StructuredContent.(map[string]any)
	pagination := structured["pagination"].(map[string]any)
	require.Equal(t, true, pagination["hasMore"])
}

func TestMCPToolPatchVaultFileLargeBodyCapped(t *testing.T) {
	large := strings.Repeat("P", 40*1024)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPatch, r.Method)
		require.Equal(t, "append", r.Header.Get("Operation"))
		require.Equal(t, "heading", r.Header.Get("Target-Type"))
		require.Equal(t, "1", r.Header.Get("Markdown-Patch-Version"))
		_, _ = w.Write([]byte(large))
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())
	ctx, cs := mcpSession(t, cli)

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "patch_vault_file",
		Arguments: map[string]any{
			"filename":   "Notes/patch.md",
			"operation":  "append",
			"targetType": "heading",
			"target":     "Intro",
			"content":    "more",
		},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Len(t, res.Content, 2)
	require.Equal(t, "File patched successfully", res.Content[0].(*mcp.TextContent).Text)
	body := res.Content[1].(*mcp.TextContent).Text
	require.Contains(t, body, "file too large (40960 bytes)")
	require.Contains(t, body, "startIndex=32768")
	require.Less(t, len(body), 50*1024)
}

func TestMCPToolGetVaultFileSchemaIntegers(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())
	ctx, cs := mcpSession(t, cli)

	var schema map[string]any
	for tool, err := range cs.Tools(ctx, nil) {
		require.NoError(t, err)
		if tool.Name != "get_vault_file" {
			continue
		}
		raw, err := json.Marshal(tool.InputSchema)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(raw, &schema))
		break
	}
	require.NotEmpty(t, schema)
	props, ok := schema["properties"].(map[string]any)
	require.True(t, ok)
	for _, name := range []string{"maxLength", "startIndex"} {
		prop, ok := props[name].(map[string]any)
		require.Truef(t, ok, "%s must be a schema object, got %T (%v)", name, props[name], props[name])
		require.Truef(t, schemaPropAllowsInteger(prop), "%s must allow integer (not JSON Schema true), got %v", name, prop)
	}
	_, hasOffset := props["offset"]
	require.False(t, hasOffset, "get_vault_file must not declare offset")
}

func schemaPropAllowsInteger(prop map[string]any) bool {
	switch typ := prop["type"].(type) {
	case string:
		return typ == "integer"
	case []any:
		for _, alt := range typ {
			if alt == "integer" {
				return true
			}
		}
	}
	if anyOf, ok := prop["anyOf"].([]any); ok {
		for _, alt := range anyOf {
			m, _ := alt.(map[string]any)
			if schemaPropAllowsInteger(m) {
				return true
			}
		}
	}
	return false
}
