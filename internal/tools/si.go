package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vgromanov/obsidian-mcp/internal/obsidian"
)

// RegisterSi registers Local Smart Lookup Semantic Index (/si/*) MCP tools.
func RegisterSi(s *mcp.Server, d Deps) {
	cli := d.Client

	mcp.AddTool(s, &mcp.Tool{
		Name: "si_health",
		Description: "Local Smart Lookup Semantic Index liveness probe (GET /si/health/). " +
			"Returns ok, version, schema_ver, chunks, and indexReady. Does not rerank.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		raw, err := cli.SIHealth(ctx)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(raw), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "si_index_info",
		Description: "Local Smart Lookup Semantic Index regime stamp (GET /si/index_info/): " +
			"embed_model, embed_dim, schema_ver, metric, counts. SI never applies the cross-encoder reranker " +
			"(si_applies_rerank is always false). Thresholds elsewhere are cosine distance (<=).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		raw, err := cli.SIIndexInfo(ctx)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(raw), nil, nil
	})

	type embedIn struct {
		Texts     []string `json:"texts"`
		Normalize *bool    `json:"normalize,omitempty"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name: "si_embed_text",
		Description: "Embed strings with the live index embedding model (POST /si/embed_text/). " +
			"Body: texts (required), normalize (default true). Returns vectors and per-item errors. " +
			"Prefer corpus-scoped neighbor tools over exporting vectors on public gateways.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in embedIn) (*mcp.CallToolResult, any, error) {
		body := map[string]any{"texts": in.Texts}
		if in.Normalize != nil {
			body["normalize"] = *in.Normalize
		}
		raw, err := cli.SIEmbedText(ctx, body)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(raw), nil, nil
	})

	type queryMetaIn struct {
		Where  string   `json:"where"`
		Fields []string `json:"fields"`
		Limit  *int     `json:"limit,omitempty"`
		Cursor *string  `json:"cursor,omitempty"`
		Offset any      `json:"offset,omitempty"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name: "si_query_metadata",
		Description: "Metadata-only keyset scan over the LanceDB index (POST /si/query_metadata/). " +
			"Requires where and fields. Use cursor for paging — numeric offset is rejected. No embeddings, no rerank.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in queryMetaIn) (*mcp.CallToolResult, any, error) {
		if in.Offset != nil {
			return nil, nil, fmt.Errorf("numeric offset is not supported; use keyset cursor")
		}
		raw, err := cli.SIQueryMetadata(ctx, obsidian.SIQueryMetadataParams{
			Where:  in.Where,
			Fields: in.Fields,
			Limit:  in.Limit,
			Cursor: in.Cursor,
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(raw), nil, nil
	})

	type knnIn struct {
		Text      *string   `json:"text,omitempty"`
		Vector    []float64 `json:"vector,omitempty"`
		ChunkID   *string   `json:"chunk_id,omitempty"`
		K         *int      `json:"k,omitempty"`
		Threshold *float64  `json:"threshold,omitempty"`
		Metric    *string   `json:"metric,omitempty"`
		Where     string    `json:"where"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name: "si_knn",
		Description: "Flat cosine k-NN over the Semantic Index (POST /si/knn/). No rerank. " +
			"where must include a corpus type filter (substring \"type\"). Provide exactly one of text, vector, or chunk_id. " +
			"If text is set, the tool embeds via /si/embed_text/ first and does not return the query vector. " +
			"threshold is cosine distance (inclusive <=).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in knnIn) (*mcp.CallToolResult, any, error) {
		vector, chunkID, err := resolveSIQuery(ctx, cli, in.Text, in.Vector, in.ChunkID)
		if err != nil {
			return nil, nil, err
		}
		p := obsidian.SIKnnParams{
			Vector:    vector,
			ChunkID:   chunkID,
			K:         in.K,
			Threshold: in.Threshold,
			Where:     in.Where,
		}
		if in.Metric != nil {
			p.Metric = *in.Metric
		}
		raw, err := cli.SIKnn(ctx, p)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(raw), nil, nil
	})

	type countIn struct {
		Text      *string   `json:"text,omitempty"`
		Vector    []float64 `json:"vector,omitempty"`
		ChunkID   *string   `json:"chunk_id,omitempty"`
		Threshold float64   `json:"threshold"`
		Metric    *string   `json:"metric,omitempty"`
		GroupBy   string    `json:"group_by"`
		Where     string    `json:"where"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name: "si_count_neighbors",
		Description: "Exact grouped neighbor counts within a cosine-distance threshold (POST /si/count_neighbors/). " +
			"where must include a corpus type filter. Provide exactly one of text, vector, or chunk_id. " +
			"If text is set, embeds via /si/embed_text/ and does not return the query vector. " +
			"group_by: uuid | project | workspace | date_bucket | path. Threshold is inclusive (<=).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in countIn) (*mcp.CallToolResult, any, error) {
		vector, chunkID, err := resolveSIQuery(ctx, cli, in.Text, in.Vector, in.ChunkID)
		if err != nil {
			return nil, nil, err
		}
		p := obsidian.SICountNeighborsParams{
			Vector:    vector,
			ChunkID:   chunkID,
			Threshold: in.Threshold,
			GroupBy:   in.GroupBy,
			Where:     in.Where,
		}
		if in.Metric != nil {
			p.Metric = *in.Metric
		}
		raw, err := cli.SICountNeighbors(ctx, p)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(raw), nil, nil
	})

	type getVectorsIn struct {
		Where       string  `json:"where"`
		IncludeText bool    `json:"include_text,omitempty"`
		Limit       *int    `json:"limit,omitempty"`
		Cursor      *string `json:"cursor,omitempty"`
		Offset      any     `json:"offset,omitempty"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name: "si_get_vectors",
		Description: "Cursor-paged vector export for offline clustering (POST /si/get_vectors/). " +
			"where must include a corpus type filter. Numeric offset is rejected — use cursor. " +
			"Do not expose this tool on public MCP gateways without an allowlist.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getVectorsIn) (*mcp.CallToolResult, any, error) {
		if in.Offset != nil {
			return nil, nil, fmt.Errorf("numeric offset is not supported; use keyset cursor")
		}
		raw, err := cli.SIGetVectors(ctx, obsidian.SIGetVectorsParams{
			Where:       in.Where,
			IncludeText: in.IncludeText,
			Limit:       in.Limit,
			Cursor:      in.Cursor,
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(raw), nil, nil
	})

	type filterValidateIn struct {
		Where  *string        `json:"where,omitempty"`
		Filter map[string]any `json:"filter,omitempty"`
		Limit  *int           `json:"limit,omitempty"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name: "si_filter_validate",
		Description: "Compile and sample a Semantic Index filter against the live index (POST /si/filter/validate/). " +
			"Body: where and/or filter, optional limit. Returns sql, row_count, and sample.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in filterValidateIn) (*mcp.CallToolResult, any, error) {
		raw, err := cli.SIFilterValidate(ctx, obsidian.SIFilterValidateParams{
			Where:  in.Where,
			Filter: in.Filter,
			Limit:  in.Limit,
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(raw), nil, nil
	})
}

// resolveSIQuery enforces exactly one of text | vector | chunk_id.
// When text is set, embeds via /si/embed_text/ and returns the vector (caller must not echo it).
func resolveSIQuery(ctx context.Context, cli *obsidian.Client, text *string, vector []float64, chunkID *string) ([]float64, string, error) {
	hasText := text != nil && strings.TrimSpace(*text) != ""
	hasVector := len(vector) > 0
	hasChunk := chunkID != nil && strings.TrimSpace(*chunkID) != ""

	n := 0
	if hasText {
		n++
	}
	if hasVector {
		n++
	}
	if hasChunk {
		n++
	}
	if n != 1 {
		return nil, "", fmt.Errorf("provide exactly one of text, vector, or chunk_id")
	}

	if hasChunk {
		return nil, strings.TrimSpace(*chunkID), nil
	}
	if hasVector {
		return vector, "", nil
	}

	raw, err := cli.SIEmbedText(ctx, map[string]any{
		"texts":     []string{strings.TrimSpace(*text)},
		"normalize": true,
	})
	if err != nil {
		return nil, "", err
	}
	var emb struct {
		Vectors [][]float64 `json:"vectors"`
		Errors  []any       `json:"errors"`
	}
	if err := json.Unmarshal(raw, &emb); err != nil {
		return nil, "", fmt.Errorf("embed_text response: %w", err)
	}
	if len(emb.Vectors) == 0 || emb.Vectors[0] == nil || len(emb.Vectors[0]) == 0 {
		return nil, "", fmt.Errorf("embed_text returned no vector for query text")
	}
	return emb.Vectors[0], "", nil
}
