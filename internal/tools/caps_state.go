package tools

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vgromanov/obsidian-mcp/internal/obsidian"
)

const (
	// DefaultReprobeInterval is the minimum gap between lazy GET / probes while
	// the caps are the unknown-version fallback.
	DefaultReprobeInterval = 30 * time.Second
	// forcedReprobeFloor bounds probes triggered by a stale-caps signal (a 40012
	// on POST /search/) so a burst of failing calls cannot hammer GET /.
	forcedReprobeFloor = 2 * time.Second
	reprobeTimeout     = 5 * time.Second
)

// CapsSource says where the live caps came from.
type CapsSource string

const (
	// CapsSourceOverride: REST_API_VERSION / Deps.RestAPIVersion; never re-probed.
	CapsSourceOverride CapsSource = "override"
	// CapsSourceProbe: GET / returned a parseable plugin version.
	CapsSourceProbe CapsSource = "probe"
	// CapsSourceFallback: version unknown (Obsidian down at startup, or GET /
	// unparseable); caps are the 3.6-safe profile and are re-probed lazily.
	CapsSourceFallback CapsSource = "fallback"
)

// CapsStatus is a point-in-time diagnostic view of CapsState.
type CapsStatus struct {
	RestAPIVersion      string     `json:"restApiVersion"`
	Source              CapsSource `json:"capsSource"`
	Fallback            bool       `json:"fallback"`
	PluginDataviewQuery bool       `json:"pluginDataviewQuery"`
	PluginGraphTraverse bool       `json:"pluginGraphTraverse"`
	Probes              int        `json:"probes"`
	LastProbeAt         string     `json:"lastProbeAt,omitempty"`
}

// CapsState holds the live Local REST capability matrix. Caps start from the
// startup probe (or a version override); while they are the unknown-version
// fallback, Refresh re-probes GET / lazily and swaps in the real caps.
type CapsState struct {
	client   *obsidian.Client
	log      *slog.Logger
	interval time.Duration
	pinned   bool
	now      func() time.Time
	probe    func(context.Context, *obsidian.Client) obsidian.Caps

	mu         sync.RWMutex
	caps       obsidian.Caps
	lastProbe  time.Time // any probe: startup, lazy or forced; gates the lazy interval
	lastForced time.Time // forced probes only; gates the forced floor
	probes     int
	flight     *probeFlight // non-nil while a probe is running
	onChange   func(old, next obsidian.Caps)
	waiters    atomic.Int32 // callers currently waiting on a flight (tests)
}

// probeFlight is one in-flight probe. Callers that arrive meanwhile wait on
// done instead of using caps the probe is about to replace.
type probeFlight struct{ done chan struct{} }

func newCapsState(client *obsidian.Client, caps obsidian.Caps, pinned bool, log *slog.Logger) *CapsState {
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	s := &CapsState{
		client:   client,
		log:      log,
		interval: DefaultReprobeInterval,
		pinned:   pinned,
		now:      time.Now,
		probe:    obsidian.ProbeCaps,
		caps:     caps,
	}
	if !pinned && client != nil {
		s.lastProbe = s.now()
		s.probes = 1
	}
	return s
}

// Caps returns the current capability matrix.
func (s *CapsState) Caps() obsidian.Caps {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.caps
}

// Status reports the probed version and whether the caps are the fallback.
func (s *CapsState) Status() CapsStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := CapsStatus{
		RestAPIVersion:      s.caps.Version,
		Source:              s.sourceLocked(),
		Fallback:            s.caps.Version == "",
		PluginDataviewQuery: s.caps.PluginDataviewQuery,
		PluginGraphTraverse: s.caps.PluginGraphTraverse,
		Probes:              s.probes,
	}
	if !s.lastProbe.IsZero() {
		st.LastProbeAt = s.lastProbe.UTC().Format(time.RFC3339)
	}
	return st
}

func (s *CapsState) sourceLocked() CapsSource {
	switch {
	case s.caps.Version == "":
		return CapsSourceFallback
	case s.pinned:
		return CapsSourceOverride
	default:
		return CapsSourceProbe
	}
}

// SetOnChange registers the hook run (outside the state lock) after a probe
// changes the caps. RegisterAll uses it to re-register version-dependent tools.
func (s *CapsState) SetOnChange(fn func(old, next obsidian.Caps)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onChange = fn
}

// Refresh re-probes GET / when the caps are the unknown-version fallback and
// the last probe is at least DefaultReprobeInterval old. It is a no-op for
// override-pinned caps, known versions, and nil clients. A caller that arrives
// while a probe is running waits for it and returns the caps it produced.
func (s *CapsState) Refresh(ctx context.Context) obsidian.Caps {
	return s.reprobe(ctx, false)
}

// Recheck re-probes GET / even when the version is known, because a request
// showed the caps are stale. Bounded to one probe per forcedReprobeFloor, which
// is measured against earlier forced probes only: a startup or lazy probe that
// ran moments ago does not suppress it. Waits for a running probe like Refresh.
func (s *CapsState) Recheck(ctx context.Context) obsidian.Caps {
	return s.reprobe(ctx, true)
}

func (s *CapsState) reprobe(ctx context.Context, forced bool) obsidian.Caps {
	if s.pinned || s.client == nil {
		return s.Caps()
	}

	s.mu.Lock()
	if f := s.flight; f != nil {
		s.mu.Unlock()
		s.waiters.Add(1)
		defer s.waiters.Add(-1)
		select {
		case <-f.done:
		case <-ctx.Done():
		}
		return s.Caps()
	}
	now := s.now()
	if forced {
		if !s.lastForced.IsZero() && now.Sub(s.lastForced) < forcedReprobeFloor {
			cur := s.caps
			s.mu.Unlock()
			return cur
		}
	} else if s.caps.Version != "" || (!s.lastProbe.IsZero() && now.Sub(s.lastProbe) < s.interval) {
		cur := s.caps
		s.mu.Unlock()
		return cur
	}
	f := &probeFlight{done: make(chan struct{})}
	s.flight = f
	s.lastProbe = now
	if forced {
		s.lastForced = now
	}
	s.probes++
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.flight = nil
		s.mu.Unlock()
		close(f.done)
	}()

	// The probe belongs to the server, not to the request that triggered it: a
	// cancelled call must not turn into a failed probe that burns the interval.
	pctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), reprobeTimeout)
	next := s.probe(pctx, s.client)
	cancel()

	s.mu.Lock()
	s.lastProbe = s.now()
	old := s.caps
	// A failed probe yields the unknown-version fallback; never downgrade known caps with it.
	if next.Version == "" && old.Version != "" {
		next = old
	}
	s.caps = next
	onChange := s.onChange
	s.mu.Unlock()

	if next != old {
		s.log.Info("local REST capabilities changed",
			"from_version", versionOrUnknown(old), "to_version", versionOrUnknown(next),
			"fallback", next.Version == "", "plugin_dataview_query", next.PluginDataviewQuery,
			"plugin_graph_traverse", next.PluginGraphTraverse)
		if onChange != nil {
			onChange(old, next)
		}
	}

	return next
}

// LogStatus writes the current caps status to the server log.
func (s *CapsState) LogStatus(msg string) {
	st := s.Status()
	s.log.Info(msg, "rest_api_version", versionOrUnknown(obsidian.Caps{Version: st.RestAPIVersion}),
		"caps_source", string(st.Source), "fallback", st.Fallback,
		"plugin_dataview_query", st.PluginDataviewQuery, "plugin_graph_traverse", st.PluginGraphTraverse)
}

func versionOrUnknown(c obsidian.Caps) string {
	if c.Version == "" {
		return "unknown"
	}
	return c.Version
}

// capsRefreshMiddleware re-probes on tools/list and tools/call so a process that
// started before Obsidian picks up the real caps on the next request.
func capsRefreshMiddleware(st *CapsState) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method == "tools/list" || method == "tools/call" {
				st.Refresh(ctx)
			}
			return next(ctx, method, req)
		}
	}
}
