package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vgromanov/obsidian-mcp/internal/omlx"
)

// RegisterLocalSmartLookup registers semantic search via the local-smart-lookup Obsidian plugin.
func RegisterLocalSmartLookup(s *mcp.Server, d Deps) {
	type localSearchIn struct {
		Query          string         `json:"query"`
		Limit          *int           `json:"limit,omitempty"`
		DataviewSource *string        `json:"dataviewSource,omitempty"`
		DataviewQuery  *string        `json:"dataviewQuery,omitempty"`
		Where          *string        `json:"where,omitempty"`
		Tags           []string       `json:"tags,omitempty"`
		Frontmatter    map[string]any `json:"frontmatter,omitempty"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name: "search_vault_local",
		Description: "Question-first semantic search over the vault using local embeddings (oMLX), LanceDB vector index, " +
			"and optional reranking in the Local Smart Lookup plugin. Returns chunk-level hits with path, text, score, " +
			"and optional rerankScore. Narrow results with tags, frontmatter, where (LanceDB metadata), or Dataview source/query.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in localSearchIn) (*mcp.CallToolResult, any, error) {
		if d.OmlxCheck {
			if err := omlx.Check(ctx, d.OmlxBaseURL, d.OmlxAPIKey); err != nil {
				return nil, nil, fmt.Errorf("%w — start oMLX and align the plugin Embedding server URL with OMLX_BASE_URL (%s)",
					err, d.OmlxBaseURL)
			}
		}
		body := map[string]any{"query": in.Query}
		if in.Limit != nil {
			body["limit"] = *in.Limit
		}
		if in.DataviewSource != nil && *in.DataviewSource != "" {
			body["dataviewSource"] = *in.DataviewSource
		}
		if in.DataviewQuery != nil && *in.DataviewQuery != "" {
			body["dataviewQuery"] = *in.DataviewQuery
		}
		if in.Where != nil && *in.Where != "" {
			body["where"] = *in.Where
		}
		if len(in.Tags) > 0 {
			body["tags"] = in.Tags
		}
		if len(in.Frontmatter) > 0 {
			body["frontmatter"] = in.Frontmatter
		}
		raw, err := d.Client.SearchVaultLocal(ctx, body)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(raw), nil, nil
	})
}
