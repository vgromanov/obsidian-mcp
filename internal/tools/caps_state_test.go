package tools

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/vgromanov/obsidian-mcp/internal/obsidian"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock { return &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func testCapsState(t *testing.T, start obsidian.Caps, pinned bool, probe func() obsidian.Caps) (*CapsState, *fakeClock, *atomic.Int32) {
	t.Helper()
	cli := obsidian.NewClientFromURL(nil, "k", nil)
	st := newCapsState(cli, start, pinned, nil)
	clk := newFakeClock()
	st.now = clk.Now
	if !pinned {
		st.lastProbe = clk.Now()
	}
	var n atomic.Int32
	st.probe = func(context.Context, *obsidian.Client) obsidian.Caps {
		n.Add(1)
		return probe()
	}
	return st, clk, &n
}

func TestCapsStateFallbackUpgradesAfterInterval(t *testing.T) {
	up := false
	st, clk, probes := testCapsState(t, obsidian.Safe36Caps(), false, func() obsidian.Caps {
		if !up {
			return obsidian.Safe36Caps()
		}
		c := obsidian.CapsForVersion("5.1.0")
		c.PluginDataviewQuery = true
		return c
	})
	var gotOld, gotNext obsidian.Caps
	st.SetOnChange(func(old, next obsidian.Caps) { gotOld, gotNext = old, next })

	require.Equal(t, CapsSourceFallback, st.Status().Source)
	require.True(t, st.Status().Fallback)

	up = true
	clk.Advance(DefaultReprobeInterval - time.Second)
	require.Equal(t, "", st.Refresh(context.Background()).Version, "inside the interval: no probe")
	require.EqualValues(t, 0, probes.Load())

	clk.Advance(time.Second)
	caps := st.Refresh(context.Background())
	require.EqualValues(t, 1, probes.Load())
	require.Equal(t, "5.1.0", caps.Version)
	require.True(t, caps.PluginDataviewQuery)
	require.Equal(t, "", gotOld.Version)
	require.Equal(t, "5.1.0", gotNext.Version)

	status := st.Status()
	require.False(t, status.Fallback)
	require.Equal(t, CapsSourceProbe, status.Source)
	require.Equal(t, "5.1.0", status.RestAPIVersion)
	require.Equal(t, 2, status.Probes)
	require.NotEmpty(t, status.LastProbeAt)

	clk.Advance(time.Hour)
	st.Refresh(context.Background())
	require.EqualValues(t, 1, probes.Load(), "known version is never re-probed")
}

func TestCapsStateNoReprobeStormWhileObsidianDown(t *testing.T) {
	st, clk, probes := testCapsState(t, obsidian.Safe36Caps(), false, obsidian.Safe36Caps)
	changed := 0
	st.SetOnChange(func(_, _ obsidian.Caps) { changed++ })

	for range 500 {
		st.Refresh(context.Background())
		clk.Advance(50 * time.Millisecond) // 25 s total
	}
	require.EqualValues(t, 0, probes.Load())

	clk.Advance(6 * time.Second)
	for range 500 {
		st.Refresh(context.Background())
	}
	require.EqualValues(t, 1, probes.Load(), "one probe per interval even with many calls")

	clk.Advance(DefaultReprobeInterval)
	st.Refresh(context.Background())
	require.EqualValues(t, 2, probes.Load())
	require.Zero(t, changed, "a failed probe leaves the caps unchanged")
}

func TestCapsStatePinnedAndNilClientNeverProbe(t *testing.T) {
	pinned, clk, probes := testCapsState(t, obsidian.CapsForVersion("3.6.1"), true, obsidian.Safe36Caps)
	clk.Advance(time.Hour)
	pinned.Refresh(context.Background())
	pinned.Recheck(context.Background())
	require.Zero(t, probes.Load())
	require.Equal(t, CapsSourceOverride, pinned.Status().Source)

	nilClient := newCapsState(nil, obsidian.Safe36Caps(), false, nil)
	nilClient.probe = func(context.Context, *obsidian.Client) obsidian.Caps {
		t.Fatal("probe with nil client")
		return obsidian.Caps{}
	}
	nilClient.Refresh(context.Background())
	nilClient.Recheck(context.Background())
	require.True(t, nilClient.Status().Fallback)
	require.Empty(t, nilClient.Status().LastProbeAt)
}

func TestCapsStateRecheckHonoursFloorAndKeepsKnownCaps(t *testing.T) {
	known := obsidian.CapsForVersion("5.0.3")
	fail := false
	st, clk, probes := testCapsState(t, known, false, func() obsidian.Caps {
		if fail {
			return obsidian.Safe36Caps()
		}
		c := obsidian.CapsForVersion("5.1.0")
		c.PluginDataviewQuery = true
		return c
	})

	fail = true
	require.Equal(t, "5.0.3", st.Recheck(context.Background()).Version, "failed probe must not downgrade known caps")
	require.EqualValues(t, 1, probes.Load(), "the startup probe does not suppress the first forced probe")

	fail = false
	require.Equal(t, "5.0.3", st.Recheck(context.Background()).Version)
	require.EqualValues(t, 1, probes.Load(), "inside the forced floor")

	clk.Advance(forcedReprobeFloor)
	require.Equal(t, "5.1.0", st.Recheck(context.Background()).Version)
	require.EqualValues(t, 2, probes.Load())
}

func TestCapsStateProbeSurvivesCancelledRequestContext(t *testing.T) {
	st, clk, probes := testCapsState(t, obsidian.Safe36Caps(), false, obsidian.Safe36Caps)
	st.probe = func(ctx context.Context, _ *obsidian.Client) obsidian.Caps {
		probes.Add(1)
		if ctx.Err() != nil {
			return obsidian.Safe36Caps()
		}
		return obsidian.CapsForVersion("5.1.0")
	}
	clk.Advance(DefaultReprobeInterval)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.Equal(t, "5.1.0", st.Refresh(ctx).Version, "a cancelled caller must not fail the probe")
}

func TestCapsStateWaitersReturnInFlightResult(t *testing.T) {
	release := make(chan struct{})
	st, clk, _ := testCapsState(t, obsidian.Safe36Caps(), false, obsidian.Safe36Caps)
	st.probe = func(context.Context, *obsidian.Client) obsidian.Caps {
		<-release
		return obsidian.CapsForVersion("5.1.0")
	}
	clk.Advance(DefaultReprobeInterval)

	results := make(chan string, 3)
	go func() { results <- st.Refresh(context.Background()).Version }()
	require.Eventually(t, func() bool {
		st.mu.RLock()
		defer st.mu.RUnlock()
		return st.flight != nil
	}, time.Second, time.Millisecond)
	for range 2 {
		go func() { results <- st.Recheck(context.Background()).Version }()
	}
	require.Eventually(t, func() bool { return st.waiters.Load() == 2 }, time.Second, time.Millisecond)
	close(release)
	for range 3 {
		require.Equal(t, "5.1.0", <-results, "every caller sees the in-flight probe's caps")
	}
}

func TestCapsStateWaiterHonoursContext(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	st, clk, _ := testCapsState(t, obsidian.Safe36Caps(), false, obsidian.Safe36Caps)
	st.probe = func(context.Context, *obsidian.Client) obsidian.Caps {
		<-release
		return obsidian.Safe36Caps()
	}
	clk.Advance(DefaultReprobeInterval)
	go st.Refresh(context.Background())
	require.Eventually(t, func() bool {
		st.mu.RLock()
		defer st.mu.RUnlock()
		return st.flight != nil
	}, time.Second, time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { st.Refresh(ctx); close(done) }()
	require.Eventually(t, func() bool { return st.waiters.Load() == 1 }, time.Second, time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("waiter ignored its cancelled context")
	}
}

func TestCapsStateConcurrentRefreshSingleFlight(t *testing.T) {
	release := make(chan struct{})
	st, clk, _ := testCapsState(t, obsidian.Safe36Caps(), false, obsidian.Safe36Caps)
	var n atomic.Int32
	st.probe = func(context.Context, *obsidian.Client) obsidian.Caps {
		n.Add(1)
		<-release
		return obsidian.CapsForVersion("5.1.0")
	}
	clk.Advance(DefaultReprobeInterval)

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			st.Refresh(context.Background())
		}()
	}
	require.Eventually(t, func() bool { return n.Load() == 1 }, time.Second, time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	close(release)
	wg.Wait()
	require.EqualValues(t, 1, n.Load())
	require.Equal(t, "5.1.0", st.Caps().Version)
}

func TestWithCapsStatus(t *testing.T) {
	out := withCapsStatus([]byte(`{"status":"OK"}`), CapsStatus{RestAPIVersion: "5.1.0", Source: CapsSourceProbe})
	require.JSONEq(t, `{"status":"OK","obsidianMcp":{"restApiVersion":"5.1.0","capsSource":"probe","fallback":false,"pluginDataviewQuery":false,"pluginGraphTraverse":false,"probes":0}}`, string(out))
	require.Equal(t, `[1]`, string(withCapsStatus([]byte(`[1]`), CapsStatus{})))
	require.Equal(t, `nope`, string(withCapsStatus([]byte(`nope`), CapsStatus{})))
}
