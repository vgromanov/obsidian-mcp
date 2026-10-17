package tools

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/vgromanov/obsidian-mcp/internal/obsidian"
)

// Deps carries shared dependencies for tool handlers.
type Deps struct {
	Client      *obsidian.Client
	PromptsDir  string
	OmlxBaseURL string
	OmlxAPIKey  string
	OmlxCheck   bool

	// RetrievalDir enables per-host append-only logging of search_vault_local
	// events (empty = disabled). RetrievalRegime is stamped on each event.
	RetrievalDir    string
	RetrievalRegime string

	// RestAPIVersion overrides Local REST plugin semver for capability gating.
	// When set (or via REST_API_VERSION / OBSIDIAN_REST_API_VERSION), GET / is
	// not probed. Empty + live Client → ProbeCaps; nil Client → fail-closed 3.6.
	RestAPIVersion string

	// Caps is the capability snapshot taken by ResolveCaps. Zero value means
	// ResolveCaps has not run yet. Live caps are read from CapsState, which may
	// be upgraded after startup by a lazy re-probe.
	Caps obsidian.Caps

	// CapsState owns the live caps and the lazy re-probe; set by ResolveCaps.
	CapsState *CapsState

	// Log receives capability status lines. Nil discards them.
	Log *slog.Logger

	// ReprobeInterval is the minimum gap between lazy GET / re-probes while the
	// caps are the unknown-version fallback. Zero means DefaultReprobeInterval.
	ReprobeInterval time.Duration

	// GraphPolicy is enforced by graph_traverse before the plugin call.
	// GraphPolicySet distinguishes an explicit policy from the permissive
	// default (zero AllowBody would otherwise deny body inclusion).
	GraphPolicy    GraphPolicy
	GraphPolicySet bool

	capsResolved bool
}

// ResolveCaps populates Caps from RestAPIVersion, env override, or GET / probe.
// Safe to call multiple times; subsequent calls are no-ops once resolved.
// Nil Client never panics. Probed (non-override) caps that come out as the
// unknown-version fallback are re-probed lazily through CapsState.
func ResolveCaps(d Deps) Deps {
	if d.capsResolved {
		return d
	}
	ver := strings.TrimSpace(d.RestAPIVersion)
	if ver == "" {
		ver = strings.TrimSpace(os.Getenv("REST_API_VERSION"))
	}
	if ver == "" {
		ver = strings.TrimSpace(os.Getenv("OBSIDIAN_REST_API_VERSION"))
	}
	pinned := ver != ""
	if pinned {
		d.Caps = obsidian.CapsForVersion(ver)
	} else {
		d.Caps = obsidian.ProbeCaps(context.Background(), d.Client)
	}
	d.CapsState = newCapsState(d.Client, d.Caps, pinned, d.Log)
	if d.ReprobeInterval > 0 {
		d.CapsState.interval = d.ReprobeInterval
	}
	d.capsResolved = true
	return d
}
