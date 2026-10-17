package tools

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/vgromanov/obsidian-mcp/internal/obsidian"
)

const dqlContentType = "application/vnd.olrapi.dataview.dql+txt"

// fakeObsidian emulates Local REST: GET / fails until up is set, then reports
// restVersion (plus the dataview route when withRoute). POST /search/ rejects
// DQL with 40012 like Local REST 5.x.
type fakeObsidian struct {
	t           *testing.T
	up          atomic.Bool
	restVersion string
	withRoute   bool

	mu    sync.Mutex
	calls []string
	types map[string]string // "METHOD path" -> last Content-Type
}

func newFakeObsidian(t *testing.T, version string, withRoute bool) (*fakeObsidian, *obsidian.Client) {
	t.Helper()
	f := &fakeObsidian{t: t, restVersion: version, withRoute: withRoute, types: map[string]string{}}
	ts := httptest.NewServer(f)
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	return f, obsidian.NewClientFromURL(u, "secret", ts.Client())
}

func (f *fakeObsidian) count(key string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if c == key {
			n++
		}
	}
	return n
}

func (f *fakeObsidian) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := r.Method + " " + r.URL.Path
	f.mu.Lock()
	f.calls = append(f.calls, key)
	f.types[key] = r.Header.Get("Content-Type")
	f.mu.Unlock()
	body, _ := io.ReadAll(r.Body)
	w.Header().Set("Content-Type", "application/json")
	switch key {
	case "GET /":
		if !f.up.Load() {
			http.Error(w, "starting", http.StatusServiceUnavailable)
			return
		}
		info := map[string]any{"status": "OK", "versions": map[string]string{"self": f.restVersion}}
		if f.withRoute {
			info["apiExtensions"] = []any{map[string]any{"routes": []any{map[string]any{"path": "/dataview/query/"}}}}
		}
		_ = json.NewEncoder(w).Encode(info)
	case "POST /search/":
		if r.Header.Get("Content-Type") == dqlContentType && strings.HasPrefix(f.restVersion, "5.") {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"errorCode":40012,"message":"Unknown or invalid Content-Type."}`))
			return
		}
		_, _ = w.Write([]byte(`[{"filename":"a.md","result":true}]`))
	case "MOVE /vault/a.md":
		w.Header().Set("Content-Location", "/vault/b.md")
		w.WriteHeader(http.StatusNoContent)
	case "POST /dataview/query/":
		require.Contains(f.t, string(body), `"query"`)
		_, _ = w.Write([]byte(`[{"path":"Projects/x.md"}]`))
	default:
		http.NotFound(w, r)
	}
}

type reprobeHarness struct {
	cs        *mcp.ClientSession
	srv       *mcp.Server
	d         Deps
	clock     *fakeClock
	listed    chan struct{}
	notifyCnt atomic.Int32
}

func newReprobeHarness(t *testing.T, cli *obsidian.Client) *reprobeHarness {
	t.Helper()
	t.Setenv("REST_API_VERSION", "")
	t.Setenv("OBSIDIAN_REST_API_VERSION", "")
	h := &reprobeHarness{listed: make(chan struct{}, 16)}
	srv := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	d := ResolveCaps(Deps{Client: cli})
	h.clock = newFakeClock()
	d.CapsState.now = h.clock.Now
	d.CapsState.lastProbe = h.clock.Now()
	RegisterAll(srv, d)
	h.srv, h.d = srv, d

	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	_, err := srv.Connect(ctx, st, nil)
	require.NoError(t, err)
	c := mcp.NewClient(&mcp.Implementation{Name: "c", Version: "0"}, &mcp.ClientOptions{
		ToolListChangedHandler: func(context.Context, *mcp.ToolListChangedRequest) {
			h.notifyCnt.Add(1)
			h.listed <- struct{}{}
		},
	})
	h.cs, err = c.Connect(ctx, ct, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = h.cs.Close() })
	return h
}

func (h *reprobeHarness) tools(t *testing.T) map[string]*mcp.Tool {
	t.Helper()
	out := map[string]*mcp.Tool{}
	for tool, err := range h.cs.Tools(context.Background(), nil) {
		require.NoError(t, err)
		out[tool.Name] = tool
	}
	return out
}

func (h *reprobeHarness) search(t *testing.T, queryType, query string) (*mcp.CallToolResult, error) {
	t.Helper()
	return h.cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "search_vault",
		Arguments: map[string]any{"queryType": queryType, "query": query},
	})
}

func resultText(res *mcp.CallToolResult) string {
	var sb strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}

func TestStartBeforeObsidianUpgradesCapsWithoutRestart(t *testing.T) {
	f, cli := newFakeObsidian(t, "5.1.0", true)
	h := newReprobeHarness(t, cli)

	require.True(t, h.d.CapsState.Status().Fallback)
	before := h.tools(t)
	require.Len(t, before, 37)
	require.Contains(t, before["search_vault"].Description, "either Dataview DQL or JsonLogic")
	require.NotContains(t, before, "move_vault_file")
	require.Contains(t, before, "get_periodic_note")

	f.up.Store(true)
	h.clock.Advance(DefaultReprobeInterval)

	res, err := h.search(t, "dataview", `TABLE file.name FROM "Projects"`)
	require.NoError(t, err)
	require.False(t, res.IsError, resultText(res))
	require.Contains(t, resultText(res), "Projects/x.md")
	require.Zero(t, f.count("POST /search/"), "dataview must not go to POST /search/ once the caps are known")
	require.Equal(t, 1, f.count("POST /dataview/query/"))

	st := h.d.CapsState.Status()
	require.False(t, st.Fallback)
	require.Equal(t, "5.1.0", st.RestAPIVersion)

	select {
	case <-h.listed:
	case <-time.After(2 * time.Second):
		t.Fatal("no notifications/tools/list_changed after the caps changed")
	}
	after := h.tools(t)
	require.Contains(t, after["search_vault"].Description, "POST /dataview/query/")
	require.NotContains(t, after["search_vault"].Description, "either Dataview DQL or JsonLogic")
	require.Contains(t, after, "move_vault_file")
	require.NotContains(t, after, "get_periodic_note")
	require.Len(t, after, 33)
}

func TestToolsListReprobes(t *testing.T) {
	f, cli := newFakeObsidian(t, "5.1.0", true)
	h := newReprobeHarness(t, cli)
	f.up.Store(true)
	h.clock.Advance(DefaultReprobeInterval)

	after := h.tools(t) // tools/list itself triggers the lazy probe
	require.Contains(t, after["search_vault"].Description, "POST /dataview/query/")
	require.Contains(t, after, "move_vault_file")
}

func TestNoReprobeStormWhileObsidianStaysDown(t *testing.T) {
	f, cli := newFakeObsidian(t, "5.1.0", true)
	h := newReprobeHarness(t, cli)

	for range 40 {
		h.tools(t)
		_, _ = h.search(t, "jsonlogic", `{"==":[1,1]}`)
		h.clock.Advance(500 * time.Millisecond) // 40 s of ticks, 2 probe windows at most
	}
	probes := f.count("GET /")
	require.LessOrEqual(t, probes, 2, "at most one GET / per 30 s window, got %d", probes)
	require.True(t, h.d.CapsState.Status().Fallback)
}

func TestDataviewRetriesOnPluginRouteAfter40012(t *testing.T) {
	f, cli := newFakeObsidian(t, "5.1.0", true)
	h := newReprobeHarness(t, cli)

	// Obsidian comes up, but the next lazy probe is still inside the 30 s
	// interval, so the caps are still the stale 3.6 fallback when the call arrives.
	f.up.Store(true)
	h.clock.Advance(5 * time.Second)

	res, err := h.search(t, "dataview", `LIST FROM #research`)
	require.NoError(t, err)
	require.False(t, res.IsError, resultText(res))
	require.Contains(t, resultText(res), "Projects/x.md")
	require.Equal(t, 1, f.count("POST /search/"), "first attempt on /search/")
	require.Equal(t, dqlContentType, f.types["POST /search/"])
	require.Equal(t, 1, f.count("POST /dataview/query/"), "exactly one retry on the plugin route")
	require.Equal(t, 1, f.count("GET /")-1, "40012 triggers one fresh probe")
	require.False(t, h.d.CapsState.Status().Fallback)
}

func TestDataviewNoRetryWhenPluginRouteUnavailable(t *testing.T) {
	f, cli := newFakeObsidian(t, "5.1.0", false)
	h := newReprobeHarness(t, cli)
	f.up.Store(true)
	h.clock.Advance(5 * time.Second)

	res, err := h.search(t, "dataview", `LIST FROM #research`)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, resultText(res), "40012")
	require.Zero(t, f.count("POST /dataview/query/"))
	require.Equal(t, 1, f.count("POST /search/"))
}

func TestSearchVaultRejectsUnknownQueryType(t *testing.T) {
	f, cli := newFakeObsidian(t, "3.6.1", false)
	h := newReprobeHarness(t, cli)

	for _, qt := range []string{"dql", "sql", "", "json-logic"} {
		res, err := h.search(t, qt, `{"==":[1,1]}`)
		require.NoError(t, err)
		require.True(t, res.IsError, "queryType %q", qt)
		msg := resultText(res)
		require.Contains(t, msg, "invalid queryType", qt)
		require.Contains(t, msg, `"dataview"`)
		require.Contains(t, msg, `"jsonlogic"`)
	}
	res, err := h.search(t, "dql", `LIST`)
	require.NoError(t, err)
	require.Contains(t, resultText(res), `use "dataview"`)
	require.Zero(t, f.count("POST /search/"), "invalid queryType must not reach Obsidian")
}

func TestSearchVaultQueryTypeCaseInsensitive(t *testing.T) {
	f, cli := newFakeObsidian(t, "3.6.1", false)
	f.up.Store(true)
	h := newReprobeHarness(t, cli)

	res, err := h.search(t, " JsonLogic ", `{"==":[1,1]}`)
	require.NoError(t, err)
	require.False(t, res.IsError, resultText(res))
	res, err = h.search(t, "DataView", `LIST`)
	require.NoError(t, err)
	require.False(t, res.IsError, resultText(res))
	require.Equal(t, dqlContentType, f.types["POST /search/"])
}

func TestGetServerInfoReportsCapsStatus(t *testing.T) {
	f, cli := newFakeObsidian(t, "5.1.0", true)
	f.up.Store(true)
	h := newReprobeHarness(t, cli)
	require.False(t, h.d.CapsState.Status().Fallback)

	res, err := h.cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_server_info"})
	require.NoError(t, err)
	require.False(t, res.IsError)
	m, ok := res.StructuredContent.(map[string]any)
	require.True(t, ok)
	diag, ok := m["obsidianMcp"].(map[string]any)
	require.True(t, ok, "obsidianMcp status missing: %v", m)
	require.Equal(t, "5.1.0", diag["restApiVersion"])
	require.Equal(t, false, diag["fallback"])
	require.Equal(t, "probe", diag["capsSource"])
}

func TestMoveVaultFileAppearsAfterUpgradeAndWorks(t *testing.T) {
	f, cli := newFakeObsidian(t, "5.1.0", true)
	h := newReprobeHarness(t, cli)
	require.NotContains(t, h.tools(t), "move_vault_file")

	f.up.Store(true)
	h.clock.Advance(DefaultReprobeInterval)
	require.Contains(t, h.tools(t), "move_vault_file")

	call := func(args map[string]any) *mcp.CallToolResult {
		res, err := h.cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "move_vault_file", Arguments: args})
		require.NoError(t, err)
		return res
	}
	res := call(map[string]any{"filename": "a.md", "destination": "b.md"})
	require.False(t, res.IsError, resultText(res))
	require.Contains(t, resultText(res), "File moved successfully")
	require.Equal(t, 1, f.count("MOVE /vault/a.md"))

	res = call(map[string]any{"filename": "a.md", "destination": "b.md", "updateLinks": false})
	require.True(t, res.IsError)
	require.Contains(t, resultText(res), "updateLinks=false is not supported")
}

func TestSyncCapsToolsRemovesToolsWhenCapsShrink(t *testing.T) {
	f, cli := newFakeObsidian(t, "5.1.0", true)
	f.up.Store(true)
	h := newReprobeHarness(t, cli)
	require.Contains(t, h.tools(t), "move_vault_file")

	syncCapsTools(h.srv, h.d, obsidian.CapsForVersion("5.1.0"), obsidian.Safe36Caps())
	names := h.tools(t)
	require.NotContains(t, names, "move_vault_file")
	require.Contains(t, names, "get_periodic_note")

	withGraph := obsidian.CapsForVersion("5.1.0")
	withGraph.PluginGraphTraverse = true
	syncCapsTools(h.srv, h.d, obsidian.Safe36Caps(), withGraph)
	names = h.tools(t)
	require.Contains(t, names, "graph_traverse")
	require.NotContains(t, names, "get_periodic_note")
	syncCapsTools(h.srv, h.d, withGraph, obsidian.CapsForVersion("5.1.0"))
	require.NotContains(t, h.tools(t), "graph_traverse")
}

func TestCapsStateLogStatusAndChangeLogged(t *testing.T) {
	var buf strings.Builder
	log := slog.New(slog.NewTextHandler(&buf, nil))
	st, clk, _ := testCapsState(t, obsidian.Safe36Caps(), false, func() obsidian.Caps { return obsidian.CapsForVersion("5.1.0") })
	st.log = log
	st.LogStatus("local REST capabilities")
	require.Contains(t, buf.String(), "fallback=true")
	require.Contains(t, buf.String(), "rest_api_version=unknown")
	clk.Advance(DefaultReprobeInterval)
	st.Refresh(context.Background())
	require.Contains(t, buf.String(), "local REST capabilities changed")
	require.Contains(t, buf.String(), "to_version=5.1.0")
}
