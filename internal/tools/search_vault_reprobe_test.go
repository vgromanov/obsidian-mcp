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
	withRoute   atomic.Bool
	noVersion   bool // GET / works but carries no versions.self
	rejectDQL   bool // POST /search/ answers DQL with 40012 (Local REST 5.x)
	pluginFail  atomic.Bool
	searchGate  atomic.Pointer[chan struct{}] // when set, POST /search/ blocks until it is closed

	mu    sync.Mutex
	calls []string
	types map[string]string // "METHOD path" -> last Content-Type
}

func newFakeObsidian(t *testing.T, version string, withRoute bool) (*fakeObsidian, *obsidian.Client) {
	t.Helper()
	f := &fakeObsidian{t: t, restVersion: version, rejectDQL: strings.HasPrefix(version, "5."), types: map[string]string{}}
	f.withRoute.Store(withRoute)
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
		info := map[string]any{"status": "OK"}
		if !f.noVersion {
			info["versions"] = map[string]string{"self": f.restVersion}
		}
		if f.withRoute.Load() {
			info["apiExtensions"] = []any{map[string]any{"routes": []any{map[string]any{"path": "/dataview/query/"}}}}
		}
		_ = json.NewEncoder(w).Encode(info)
	case "POST /search/":
		if g := f.searchGate.Load(); g != nil {
			<-*g
		}
		if r.Header.Get("Content-Type") == dqlContentType && f.rejectDQL {
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
		if f.pluginFail.Load() {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"DQL parse error near LIST"}`))
			return
		}
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

func newReprobeHarness(t *testing.T, cli *obsidian.Client, opts ...func(*Deps)) *reprobeHarness {
	t.Helper()
	t.Setenv("REST_API_VERSION", "")
	t.Setenv("OBSIDIAN_REST_API_VERSION", "")
	h := &reprobeHarness{listed: make(chan struct{}, 16)}
	srv := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	h.clock = newFakeClock()
	d := Deps{Client: cli, Now: h.clock.Now}
	for _, o := range opts {
		o(&d)
	}
	d = ResolveCaps(d)
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
	// The SDK debounces the notification for the initial AddTool calls by 10 ms
	// and delivers it to sessions connected by then. Let it land, then count only
	// notifications caused by caps changes.
	time.Sleep(60 * time.Millisecond)
	h.notifyCnt.Store(0)
	for len(h.listed) > 0 {
		<-h.listed
	}
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

// gateProbe makes the probe block until release is closed, and signals entered.
func gateProbe(h *reprobeHarness) (entered <-chan struct{}, release func()) {
	in := make(chan struct{}, 8)
	gate := make(chan struct{})
	orig := h.d.CapsState.probe
	h.d.CapsState.probe = func(ctx context.Context, c *obsidian.Client) obsidian.Caps {
		in <- struct{}{}
		<-gate
		return orig(ctx, c)
	}
	var once sync.Once
	return in, func() { once.Do(func() { close(gate) }) }
}

func (h *reprobeHarness) waitFlight(t *testing.T) {
	t.Helper()
	require.Eventually(t, func() bool {
		h.d.CapsState.mu.RLock()
		defer h.d.CapsState.mu.RUnlock()
		return h.d.CapsState.flight != nil
	}, 2*time.Second, time.Millisecond)
}

type searchOutcome struct {
	res *mcp.CallToolResult
	err error
}

func (h *reprobeHarness) searchAsync(t *testing.T, n int, queryType, query string) <-chan searchOutcome {
	t.Helper()
	out := make(chan searchOutcome, n)
	for range n {
		go func() {
			res, err := h.search(t, queryType, query)
			out <- searchOutcome{res, err}
		}()
	}
	return out
}

func requireAllSucceed(t *testing.T, out <-chan searchOutcome, n int) {
	t.Helper()
	for range n {
		select {
		case o := <-out:
			require.NoError(t, o.err)
			require.False(t, o.res.IsError, resultText(o.res))
			require.Contains(t, resultText(o.res), "Projects/x.md")
		case <-time.After(5 * time.Second):
			t.Fatal("search_vault call did not finish")
		}
	}
}

func TestConcurrentDataviewCallsWaitForLazyProbe(t *testing.T) {
	f, cli := newFakeObsidian(t, "5.1.0", true)
	h := newReprobeHarness(t, cli)
	entered, release := gateProbe(h)
	t.Cleanup(release)
	f.up.Store(true)
	h.clock.Advance(DefaultReprobeInterval)

	out := h.searchAsync(t, 1, "dataview", `LIST`)
	<-entered
	h.waitFlight(t)
	rest := h.searchAsync(t, 3, "dataview", `LIST`)
	require.Eventually(t, func() bool { return h.d.CapsState.waiters.Load() == 3 }, 2*time.Second, time.Millisecond)
	release()

	requireAllSucceed(t, out, 1)
	requireAllSucceed(t, rest, 3)
	require.Zero(t, f.count("POST /search/"), "no caller may fall through to /search/ while the probe runs")
	require.Equal(t, 4, f.count("POST /dataview/query/"))
	require.Equal(t, 2, f.count("GET /"), "startup probe plus exactly one re-probe")
}

func TestConcurrentDataviewCallsRecoverFrom40012WhileRecheckRuns(t *testing.T) {
	f, cli := newFakeObsidian(t, "5.1.0", true)
	h := newReprobeHarness(t, cli)
	entered, release := gateProbe(h)
	t.Cleanup(release)
	f.up.Store(true)
	h.clock.Advance(5 * time.Second) // inside the 30 s lazy window: only the 40012 drives a recheck

	// Hold all three calls inside POST /search/ so they reach Recheck together.
	gate := make(chan struct{})
	f.searchGate.Store(&gate)
	out := h.searchAsync(t, 3, "dataview", `LIST`)
	require.Eventually(t, func() bool { return f.count("POST /search/") == 3 }, 2*time.Second, time.Millisecond)
	close(gate)
	<-entered
	require.Eventually(t, func() bool { return h.d.CapsState.waiters.Load() == 2 }, 2*time.Second, time.Millisecond,
		"the two losing callers must wait for the winner's probe")
	release()

	requireAllSucceed(t, out, 3)
	require.Equal(t, 3, f.count("POST /dataview/query/"), "every caller retries once on the plugin route")
	require.Equal(t, 2, f.count("GET /"), "one shared recheck")
}

func TestDataviewRetriesWhenProbeSucceedsWithoutVersion(t *testing.T) {
	f, cli := newFakeObsidian(t, "5.1.0", true)
	f.noVersion = true
	h := newReprobeHarness(t, cli)
	f.up.Store(true)
	// No clock advance: the startup probe is under 2 s old, which must not suppress the recheck.

	res, err := h.search(t, "dataview", `LIST`)
	require.NoError(t, err)
	require.False(t, res.IsError, resultText(res))
	require.Contains(t, resultText(res), "Projects/x.md")
	caps := h.d.CapsState.Caps()
	require.Equal(t, "", caps.Version)
	require.True(t, caps.RestDataviewDQL, "caps stay the fallback, yet the retry happened")
	require.True(t, caps.PluginDataviewQuery)
	require.Equal(t, 1, f.count("POST /search/"))
	require.Equal(t, 1, f.count("POST /dataview/query/"))
}

func TestDataviewRetryFailureSurfacesPluginError(t *testing.T) {
	f, cli := newFakeObsidian(t, "5.1.0", true)
	h := newReprobeHarness(t, cli)
	f.up.Store(true)
	f.pluginFail.Store(true)

	res, err := h.search(t, "dataview", `LIST FROM`)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, resultText(res), "DQL parse error near LIST")
	require.Equal(t, 1, f.count("POST /search/"))
	require.Equal(t, 1, f.count("POST /dataview/query/"), "the plugin route is tried once, not looped")
}

func TestKnownCapsWithoutRouteRecheckWhenRouteAppearsLater(t *testing.T) {
	f, cli := newFakeObsidian(t, "5.1.0", false)
	h := newReprobeHarness(t, cli)
	f.up.Store(true)
	h.clock.Advance(DefaultReprobeInterval)
	names := h.tools(t) // lazy probe: known 5.1.0, no Local Smart Lookup route yet
	require.Contains(t, names["search_vault"].Description, "install Local Smart Lookup")
	require.Eventually(t, func() bool { return h.notifyCnt.Load() >= 1 }, 2*time.Second, time.Millisecond)

	res, err := h.search(t, "dataview", `LIST`)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, resultText(res), "not supported")
	require.Zero(t, f.count("POST /dataview/query/"))
	probesBefore := f.count("GET /")

	_, _ = h.search(t, "dataview", `LIST`)
	require.Equal(t, probesBefore, f.count("GET /"), "the second call inside the 2 s floor does not probe again")

	f.withRoute.Store(true) // Local Smart Lookup finishes loading
	h.clock.Advance(3 * time.Second)
	res, err = h.search(t, "dataview", `LIST`)
	require.NoError(t, err)
	require.False(t, res.IsError, resultText(res))
	require.Contains(t, resultText(res), "Projects/x.md")
	require.Zero(t, f.count("POST /search/"), "5.x caps never send DQL to /search/")
	require.Equal(t, 1, f.count("POST /dataview/query/"))

	require.Eventually(t, func() bool { return h.notifyCnt.Load() >= 2 }, 2*time.Second, time.Millisecond,
		"the second caps change sends a second list_changed")
	require.Contains(t, h.tools(t)["search_vault"].Description, "or Dataview DQL (POST /dataview/query/")
}

func TestVersionOverridePinsCapsAtServerLevel(t *testing.T) {
	f, cli := newFakeObsidian(t, "5.1.0", true)
	f.up.Store(true)
	h := newReprobeHarness(t, cli, func(d *Deps) { d.RestAPIVersion = "3.6.1" })

	for range 3 {
		h.clock.Advance(time.Hour)
		require.Len(t, h.tools(t), 37)
		_, _ = h.search(t, "jsonlogic", `{"==":[1,1]}`)
		res, err := h.search(t, "dataview", `LIST`) // 3.6 caps send DQL to /search/ -> 40012, no recheck while pinned
		require.NoError(t, err)
		require.True(t, res.IsError)
	}
	require.Zero(t, f.count("GET /"), "an override never probes, at startup or later")
	require.Zero(t, f.count("POST /dataview/query/"))
	require.Zero(t, h.notifyCnt.Load(), "no re-registration while pinned")
	require.Equal(t, CapsSourceOverride, h.d.CapsState.Status().Source)
}

func TestRegisterLocalRESTWithoutRegisterAllUsesLiveCaps(t *testing.T) {
	t.Setenv("REST_API_VERSION", "")
	t.Setenv("OBSIDIAN_REST_API_VERSION", "")
	f, cli := newFakeObsidian(t, "5.1.0", true)
	clk := newFakeClock()
	d := ResolveCaps(Deps{Client: cli, Now: clk.Now})
	srv := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	RegisterLocalREST(srv, d) // no middleware, no onChange: handlers still read live caps

	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	_, err := srv.Connect(ctx, st, nil)
	require.NoError(t, err)
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "c", Version: "0"}, nil).Connect(ctx, ct, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })

	f.up.Store(true)
	clk.Advance(DefaultReprobeInterval)
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "search_vault", Arguments: map[string]any{"queryType": "dataview", "query": "LIST"}})
	require.NoError(t, err)
	require.False(t, res.IsError, resultText(res))
	require.Zero(t, f.count("POST /search/"))
	require.False(t, d.CapsState.Status().Fallback)
}
