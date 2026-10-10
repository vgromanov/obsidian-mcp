// Package config loads CLI flags and environment for obsidian-mcp.
package config

import (
	"flag"
	"os"
	"strconv"
	"strings"
)

// Config holds runtime configuration.
type Config struct {
	APIKey       string
	Host         string
	UseHTTP      bool
	Transport    string // "stdio" or "http"
	HTTPAddr     string // host:port for streamable HTTP
	PromptsDir   string
	OmlxBaseURL  string
	OmlxAPIKey   string
	OmlxCheck    bool
	PrintVersion bool

	// RetrievalDir, when set, enables best-effort append-only logging of
	// search_vault_local events to <RetrievalDir>/<hostname>.jsonl (one shard
	// per machine). Point it inside the synced vault so the dream-cycle judge
	// can consume it across workstations. Empty disables logging.
	RetrievalDir string
	// RetrievalRegime is an opaque retriever/reranker version tag stamped on
	// every logged event so downstream scoring never compares incomparable regimes.
	RetrievalRegime string

	// GraphTraverse is server-side policy for graph_traverse, applied before
	// the plugin call. Load leaves it permissive unless the env vars are set.
	GraphTraverse GraphTraverse
}

// GraphTraverse is the graph_traverse policy loaded from the environment.
// An empty ScopeAllowlist allows every scope. MaxLimitNodes 0 means no extra
// cap. AllowBody and AllowFullExport default to true.
type GraphTraverse struct {
	ScopeAllowlist       []string
	MaxLimitNodes        int
	MaxLimitNodesInvalid bool
	AllowBody            bool
	AllowFullExport      bool
}

func envString(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	case "":
		return def
	default:
		return def
	}
}

// Load parses flags and environment. Call from main() after flag.Parse().
func Load() *Config {
	c := &Config{
		APIKey:      envString("OBSIDIAN_API_KEY", ""),
		Host:        envString("OBSIDIAN_HOST", "127.0.0.1"),
		UseHTTP:     envBool("OBSIDIAN_USE_HTTP", false),
		Transport:   "stdio",
		HTTPAddr:    "127.0.0.1:8765",
		PromptsDir:  envString("OBSIDIAN_PROMPTS_DIR", "Prompts"),
		OmlxBaseURL: envString("OMLX_BASE_URL", "http://127.0.0.1:8000/v1"),
		OmlxAPIKey:  envString("OMLX_API_KEY", ""),
		OmlxCheck:   envBool("OBSIDIAN_OMLX_CHECK", true),

		RetrievalDir:    envString("OBSIDIAN_RETRIEVAL_DIR", ""),
		RetrievalRegime: envString("OBSIDIAN_RETRIEVAL_REGIME", ""),
		GraphTraverse:   loadGraphTraverse(),
	}

	flag.StringVar(&c.Transport, "transport", envString("OBSIDIAN_MCP_TRANSPORT", "stdio"), "MCP transport: stdio or http")
	flag.StringVar(&c.HTTPAddr, "addr", envString("OBSIDIAN_MCP_ADDR", "127.0.0.1:8765"), "Listen address for --transport=http (streamable HTTP /mcp)")
	flag.StringVar(&c.PromptsDir, "prompts-dir", c.PromptsDir, "Vault subfolder scanned for MCP prompts (tag mcp-tools-prompt)")
	flag.BoolVar(&c.PrintVersion, "version", false, "Print version and exit")
	flag.Parse()

	// Allow overriding host/key after flag.Parse from env (flags win if set on command line — simplified: env always applied for secrets if flag empty)
	if c.APIKey == "" {
		c.APIKey = envString("OBSIDIAN_API_KEY", "")
	}
	return c
}

func loadGraphTraverse() GraphTraverse {
	maxNodes, invalid := parseMaxLimitNodes(os.Getenv("OBSIDIAN_GRAPH_MAX_LIMIT_NODES"))
	return GraphTraverse{
		ScopeAllowlist:       splitCSV(os.Getenv("OBSIDIAN_GRAPH_SCOPE_ALLOWLIST")),
		MaxLimitNodes:        maxNodes,
		MaxLimitNodesInvalid: invalid,
		AllowBody:            envBool("OBSIDIAN_GRAPH_ALLOW_BODY", true),
		AllowFullExport:      envBool("OBSIDIAN_GRAPH_ALLOW_FULL_EXPORT", true),
	}
}

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

// parseMaxLimitNodes treats empty and 0 as "no extra cap". Any other
// non-integer or negative value is invalid.
func parseMaxLimitNodes(raw string) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "0" {
		return 0, false
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0, true
	}
	return n, false
}
