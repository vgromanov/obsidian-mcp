package tools

import (
	"github.com/vgromanov/obsidian-mcp/internal/obsidian"
)

// Deps carries shared dependencies for tool handlers.
type Deps struct {
	Client     *obsidian.Client
	PromptsDir string
}
