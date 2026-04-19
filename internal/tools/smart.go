package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterSmartConnections registers semantic search (Smart Connections via obsidian-mcp-tools plugin route).
func RegisterSmartConnections(s *mcp.Server, d Deps) {
	type filterIn struct {
		Folders        []string `json:"folders,omitempty"`
		ExcludeFolders []string `json:"excludeFolders,omitempty"`
		Limit          *int     `json:"limit,omitempty"`
	}
	type smartIn struct {
		Query  string    `json:"query"`
		Filter *filterIn `json:"filter,omitempty"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "search_vault_smart",
		Description: "Search for documents semantically matching a text string.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in smartIn) (*mcp.CallToolResult, *void, error) {
		body := map[string]any{"query": in.Query}
		if in.Filter != nil {
			f := map[string]any{}
			if len(in.Filter.Folders) > 0 {
				f["folders"] = in.Filter.Folders
			}
			if len(in.Filter.ExcludeFolders) > 0 {
				f["excludeFolders"] = in.Filter.ExcludeFolders
			}
			if in.Filter.Limit != nil {
				f["limit"] = *in.Filter.Limit
			}
			if len(f) > 0 {
				body["filter"] = f
			}
		}
		raw, err := d.Client.SearchVaultSmart(ctx, body)
		if err != nil {
			return nil, nil, err
		}
		return textResult(prettyJSONBytes(raw)), nil, nil
	})
}
