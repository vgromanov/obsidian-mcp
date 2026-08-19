package obsidian

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Caps is the Local REST API capability matrix used to gate MCP tools/list.
type Caps struct {
	// Version is the parsed plugin semver (empty when unknown / probe failed).
	Version string
	// MoveVaultFile is true when MOVE /vault/{path} is available (REST ≥4.1.0).
	MoveVaultFile bool
	// RestDataviewDQL is true when POST /search/ accepts Dataview DQL (REST <4.0).
	RestDataviewDQL bool
	// Periodic is true when /periodic/ routes are advertised (hidden only when known gone).
	Periodic bool
}

// Safe36Caps is the fail-closed catalog: current 3.6-safe intersection (no MOVE,
// Dataview DQL allowed, periodic present). Used for unknown versions and probe
// failures only (never advertise 5.x-only tools).
func Safe36Caps() Caps {
	return Caps{
		Version:         "",
		MoveVaultFile:   false,
		RestDataviewDQL: true,
		Periodic:        true,
	}
}

// CapsForVersion maps a Local REST plugin semver to the capability matrix.
// Unknown / empty / unparseable → Safe36Caps. Major ≥5 → 4.1 ∩ 5.x intersection
// (MOVE on, REST Dataview DQL off, periodic off — 5.x has no /periodic/).
func CapsForVersion(version string) Caps {
	v := strings.TrimSpace(version)
	if v == "" {
		return Safe36Caps()
	}
	maj, min, _, ok := parseSemver(v)
	if !ok {
		return Safe36Caps()
	}
	if maj >= 5 {
		// 5.x (live 5.0.3): 4.1 ∩ 5.0.3 — MOVE still present; no /periodic/; no REST DQL.
		return Caps{
			Version:         v,
			MoveVaultFile:   true,
			RestDataviewDQL: false,
			Periodic:        false,
		}
	}
	c := Caps{
		Version:         v,
		MoveVaultFile:   false,
		RestDataviewDQL: true,
		Periodic:        true,
	}
	switch {
	case maj < 4:
		// 3.x (and older): full current surface minus MOVE.
		return c
	case maj == 4 && min == 0:
		c.RestDataviewDQL = false
		return c
	default:
		// 4.1.x+ (target 4.1.7): MOVE + no REST Dataview DQL; still /periodic/.
		c.RestDataviewDQL = false
		c.MoveVaultFile = true
		return c
	}
}

// serverInfoRoot is the subset of GET / used for version detection.
type serverInfoRoot struct {
	Versions *struct {
		Self string `json:"self"`
	} `json:"versions"`
	Manifest *struct {
		Version string `json:"version"`
	} `json:"manifest"`
}

// ParseServerInfoVersion extracts the plugin semver from a GET / JSON body.
// Prefer versions.self, then manifest.version.
func ParseServerInfoVersion(raw []byte) (string, error) {
	var root serverInfoRoot
	if err := json.Unmarshal(raw, &root); err != nil {
		return "", fmt.Errorf("parse GET /: %w", err)
	}
	if root.Versions != nil {
		if v := strings.TrimSpace(root.Versions.Self); v != "" {
			return v, nil
		}
	}
	if root.Manifest != nil {
		if v := strings.TrimSpace(root.Manifest.Version); v != "" {
			return v, nil
		}
	}
	return "", fmt.Errorf("GET /: no versions.self or manifest.version")
}

// ProbeCaps calls GET / and returns CapsForVersion. On any failure returns Safe36Caps.
// Nil client is safe and returns Safe36Caps without panicking.
func ProbeCaps(ctx context.Context, c *Client) Caps {
	if c == nil {
		return Safe36Caps()
	}
	raw, err := c.GetServerInfo(ctx)
	if err != nil {
		return Safe36Caps()
	}
	ver, err := ParseServerInfoVersion(raw)
	if err != nil {
		return Safe36Caps()
	}
	return CapsForVersion(ver)
}

// parseSemver parses a leading major.minor.patch (optional pre-release suffix).
func parseSemver(v string) (major, minor, patch int, ok bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if v == "" {
		return 0, 0, 0, false
	}
	// Strip pre-release / build metadata.
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) < 2 {
		return 0, 0, 0, false
	}
	maj, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, false
	}
	min, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, false
	}
	pat := 0
	if len(parts) >= 3 {
		pat, err = strconv.Atoi(parts[2])
		if err != nil {
			return 0, 0, 0, false
		}
	}
	return maj, min, pat, true
}
