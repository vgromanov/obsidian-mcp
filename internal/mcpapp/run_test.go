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
)

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
	srv := NewMCPServer(nil, cli, "Prompts")
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
	require.Contains(t, txt, `"status": "ok"`)
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
	srv := NewMCPServer(nil, cli, "Prompts")
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
	srv := NewMCPServer(nil, cli, "Prompts")
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
	require.Contains(t, txt, `"name": "project"`)
	require.Contains(t, txt, `"count": 3`)
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
	srv := NewMCPServer(nil, cli, "Prompts")
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
	srv := NewMCPServer(nil, cli, "Prompts")
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
	srv := NewMCPServer(nil, cli, "Prompts")
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
	srv := NewMCPServer(nil, cli, "Prompts")
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
	require.Contains(t, res.Content[0].(*mcp.TextContent).Text, `"filename": "Notes/A.md"`)
}
