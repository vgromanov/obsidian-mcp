// Package config loads CLI flags and environment for obsidian-mcp.
package config

import (
	"flag"
	"os"
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
	PrintVersion bool
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
		APIKey:     envString("OBSIDIAN_API_KEY", ""),
		Host:       envString("OBSIDIAN_HOST", "127.0.0.1"),
		UseHTTP:    envBool("OBSIDIAN_USE_HTTP", false),
		Transport:  "stdio",
		HTTPAddr:   "127.0.0.1:8765",
		PromptsDir: envString("OBSIDIAN_PROMPTS_DIR", "Prompts"),
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
