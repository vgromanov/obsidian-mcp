// Command obsidian-mcp is a Go port of https://github.com/jacksteamdev/obsidian-mcp-tools (MCP server only).
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/vgromanov/obsidian-mcp/internal/config"
	"github.com/vgromanov/obsidian-mcp/internal/mcpapp"
	"github.com/vgromanov/obsidian-mcp/internal/version"
)

func main() {
	cfg := config.Load()
	if cfg.PrintVersion {
		fmt.Printf("%s %s\n", version.Name, version.Version)
		os.Exit(0)
	}
	if cfg.APIKey == "" {
		slog.Error("OBSIDIAN_API_KEY is required")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := mcpapp.Run(ctx, cfg); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
