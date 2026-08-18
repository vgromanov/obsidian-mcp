package tools

import (
	"context"
	"os"
	"strings"

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

	// Caps is resolved before RegisterAll (see ResolveCaps). Zero value means
	// ResolveCaps has not run yet.
	Caps obsidian.Caps

	capsResolved bool
}

// ResolveCaps populates Caps from RestAPIVersion, env override, or GET / probe.
// Safe to call multiple times; subsequent calls are no-ops once resolved.
// Nil Client never panics.
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
	if ver != "" {
		d.Caps = obsidian.CapsForVersion(ver)
		d.capsResolved = true
		return d
	}
	d.Caps = obsidian.ProbeCaps(context.Background(), d.Client)
	d.capsResolved = true
	return d
}
