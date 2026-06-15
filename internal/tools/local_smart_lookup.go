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
		Description: "Question-first hybrid search over the vault via the Local Smart Lookup plugin: local embeddings (oMLX) " +
			"over a LanceDB vector index plus a BM25 full-text leg, reranked by a local cross-encoder and fused with " +
			"Reciprocal Rank Fusion. Returns chunk-level hits already ordered by fusedScore; each hit includes path, text, " +
			"score (vector), and optional rerankScore, ftsScore (BM25), fusedScore, and per-leg ranks. Trust the result order " +
			"rather than any single score (rerankScore saturates near 1.0 for clearly-relevant hits). Narrow results with " +
			"tags, frontmatter, where (LanceDB metadata), or Dataview source/query.",
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
		logRetrieval(d.RetrievalDir, d.RetrievalRegime, in.Query, body, raw)
		return jsonResult(raw), nil, nil
	})
}
