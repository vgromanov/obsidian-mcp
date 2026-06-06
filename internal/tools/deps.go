package tools

import (
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
}
