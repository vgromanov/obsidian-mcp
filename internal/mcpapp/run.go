package mcpapp

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vgromanov/obsidian-mcp/internal/config"
	"github.com/vgromanov/obsidian-mcp/internal/obsidian"
	"github.com/vgromanov/obsidian-mcp/internal/prompts"
	"github.com/vgromanov/obsidian-mcp/internal/tools"
	"github.com/vgromanov/obsidian-mcp/internal/version"
)

// NewMCPServer builds the Obsidian MCP server (tools + vault prompts). Exposed for tests.
func NewMCPServer(log *slog.Logger, cli *obsidian.Client, promptsDir string) *mcp.Server {
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	}
	pd := strings.Trim(strings.TrimSpace(promptsDir), "/")
	if pd == "" {
		pd = "Prompts"
	}
	srv := mcp.NewServer(&mcp.Implementation{Name: version.Name, Version: version.Version}, &mcp.ServerOptions{
		Logger:       log,
		HasPrompts:   true,
		Instructions: "Obsidian MCP: Local REST API bridge with Smart Connections + Templater routes from the obsidian-mcp-tools Obsidian plugin. See README for prerequisites.",
	})
	tools.RegisterAll(srv, tools.Deps{Client: cli, PromptsDir: pd})
	srv.AddReceivingMiddleware(prompts.DynamicVaultMiddleware(prompts.Deps{Client: cli, PromptsDir: pd}))
	return srv
}

// Run starts stdio or HTTP transport until ctx is cancelled.
func Run(ctx context.Context, cfg *config.Config) error {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cli, err := obsidian.NewClient(cfg.Host, cfg.UseHTTP, cfg.APIKey)
	if err != nil {
		return err
	}

	srv := NewMCPServer(log, cli, cfg.PromptsDir)

	t := strings.ToLower(strings.TrimSpace(cfg.Transport))
	switch t {
	case "http":
		log.Info("streamable HTTP", "addr", cfg.HTTPAddr, "path", "/mcp")
		return RunStreamableHTTP(ctx, srv, cfg.HTTPAddr)
	default:
		return srv.Run(ctx, &mcp.StdioTransport{})
	}
}
