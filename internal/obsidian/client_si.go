package obsidian

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// requireSICorpusWhere matches dreamcycle HttpSIClient: knn / count_neighbors /
// get_vectors need an explicit corpus type filter in where.
func requireSICorpusWhere(op, where string) error {
	if where == "" || !strings.Contains(where, "type") {
		return fmt.Errorf("%s requires an explicit corpus-type where filter", op)
	}
	return nil
}

func (c *Client) postSIJSON(ctx context.Context, path string, body any) (json.RawMessage, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	h := http.Header{}
	h.Set("Content-Type", mimeJSON)
	opt := RequestOptions{Method: http.MethodPost, Path: path, BodyString: string(raw), Headers: h}
	_, b, err := c.Do(ctx, opt)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

// SIHealth GET /si/health/ (Local Smart Lookup semantic-index extension).
func (c *Client) SIHealth(ctx context.Context) (json.RawMessage, error) {
	opt := RequestOptions{Method: http.MethodGet, Path: "/si/health/"}
	_, b, err := c.Do(ctx, opt)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

// SIIndexInfo GET /si/index_info/.
func (c *Client) SIIndexInfo(ctx context.Context) (json.RawMessage, error) {
	opt := RequestOptions{Method: http.MethodGet, Path: "/si/index_info/"}
	_, b, err := c.Do(ctx, opt)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

// SIEmbedText POST /si/embed_text/ — body typically {texts, normalize}.
func (c *Client) SIEmbedText(ctx context.Context, body any) (json.RawMessage, error) {
	return c.postSIJSON(ctx, "/si/embed_text/", body)
}

// SIQueryMetadataParams is the request for POST /si/query_metadata/.
// Numeric offset is not supported (keyset cursor only).
type SIQueryMetadataParams struct {
	Where  string   `json:"where"`
	Fields []string `json:"fields"`
	Limit  *int     `json:"limit,omitempty"`
	Cursor *string  `json:"cursor,omitempty"`
}

// SIQueryMetadata POST /si/query_metadata/.
func (c *Client) SIQueryMetadata(ctx context.Context, p SIQueryMetadataParams) (json.RawMessage, error) {
	body := map[string]any{
		"where":  p.Where,
		"fields": p.Fields,
	}
	if p.Limit != nil {
		body["limit"] = *p.Limit
	}
	if p.Cursor != nil && *p.Cursor != "" {
		body["cursor"] = *p.Cursor
	}
	return c.postSIJSON(ctx, "/si/query_metadata/", body)
}

// SIKnnParams is the request for POST /si/knn/ (exactly one of Vector or ChunkID).
type SIKnnParams struct {
	Vector    []float64 `json:"vector,omitempty"`
	ChunkID   string    `json:"chunk_id,omitempty"`
	K         *int      `json:"k,omitempty"`
	Threshold *float64  `json:"threshold,omitempty"`
	Metric    string    `json:"metric,omitempty"`
	Where     string    `json:"where"`
}

// SIKnn POST /si/knn/. Requires where containing "type".
func (c *Client) SIKnn(ctx context.Context, p SIKnnParams) (json.RawMessage, error) {
	if err := requireSICorpusWhere("knn", p.Where); err != nil {
		return nil, err
	}
	body := map[string]any{"where": p.Where}
	if len(p.Vector) > 0 {
		body["vector"] = p.Vector
	}
	if p.ChunkID != "" {
		body["chunk_id"] = p.ChunkID
	}
	if p.K != nil {
		body["k"] = *p.K
	}
	if p.Threshold != nil {
		body["threshold"] = *p.Threshold
	}
	if p.Metric != "" {
		body["metric"] = p.Metric
	}
	return c.postSIJSON(ctx, "/si/knn/", body)
}

// SICountNeighborsParams is the request for POST /si/count_neighbors/.
type SICountNeighborsParams struct {
	Vector    []float64 `json:"vector,omitempty"`
	ChunkID   string    `json:"chunk_id,omitempty"`
	Threshold float64   `json:"threshold"`
	Metric    string    `json:"metric,omitempty"`
	GroupBy   string    `json:"group_by"`
	Where     string    `json:"where"`
}

// SICountNeighbors POST /si/count_neighbors/. Requires where containing "type".
func (c *Client) SICountNeighbors(ctx context.Context, p SICountNeighborsParams) (json.RawMessage, error) {
	if err := requireSICorpusWhere("count_neighbors", p.Where); err != nil {
		return nil, err
	}
	body := map[string]any{
		"threshold": p.Threshold,
		"group_by":  p.GroupBy,
		"where":     p.Where,
	}
	if len(p.Vector) > 0 {
		body["vector"] = p.Vector
	}
	if p.ChunkID != "" {
		body["chunk_id"] = p.ChunkID
	}
	if p.Metric != "" {
		body["metric"] = p.Metric
	}
	return c.postSIJSON(ctx, "/si/count_neighbors/", body)
}

// SIGetVectorsParams is the request for POST /si/get_vectors/.
type SIGetVectorsParams struct {
	Where       string  `json:"where"`
	IncludeText bool    `json:"include_text,omitempty"`
	Limit       *int    `json:"limit,omitempty"`
	Cursor      *string `json:"cursor,omitempty"`
}

// SIGetVectors POST /si/get_vectors/. Requires where containing "type".
func (c *Client) SIGetVectors(ctx context.Context, p SIGetVectorsParams) (json.RawMessage, error) {
	if err := requireSICorpusWhere("get_vectors", p.Where); err != nil {
		return nil, err
	}
	body := map[string]any{
		"where":        p.Where,
		"include_text": p.IncludeText,
	}
	if p.Limit != nil {
		body["limit"] = *p.Limit
	}
	if p.Cursor != nil && *p.Cursor != "" {
		body["cursor"] = *p.Cursor
	}
	return c.postSIJSON(ctx, "/si/get_vectors/", body)
}

// SIFilterValidateParams is the request for POST /si/filter/validate/.
type SIFilterValidateParams struct {
	Where  *string        `json:"where,omitempty"`
	Filter map[string]any `json:"filter,omitempty"`
	Limit  *int           `json:"limit,omitempty"`
}

// SIFilterValidate POST /si/filter/validate/.
func (c *Client) SIFilterValidate(ctx context.Context, p SIFilterValidateParams) (json.RawMessage, error) {
	body := map[string]any{}
	if p.Where != nil {
		body["where"] = *p.Where
	}
	if len(p.Filter) > 0 {
		body["filter"] = p.Filter
	}
	if p.Limit != nil {
		body["limit"] = *p.Limit
	}
	return c.postSIJSON(ctx, "/si/filter/validate/", body)
}
