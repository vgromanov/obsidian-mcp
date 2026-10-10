package obsidian

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

// graphTraversePath is the Local Smart Lookup read-only link-graph route.
const graphTraversePath = "/graph/traverse/"

// GraphTraverseEdge is one edge source on POST /graph/traverse/.
// Source is a frontmatter field name, or $body for wikilinks in the note body.
type GraphTraverseEdge struct {
	Source   string   `json:"source"`
	Sections []string `json:"sections,omitempty"`
	Embeds   *bool    `json:"embeds,omitempty"`
}

// GraphTraverseRequest is the JSON body for POST /graph/traverse/.
// Empty IDField and Direction are omitted so the route applies its defaults
// (vault-path identity, and direction out). A nil CycleSources is omitted so
// the route computes cycles over every edge source except $body. Omitted start
// (nil or empty after the tool's own check) asks the route for the whole scope.
type GraphTraverseRequest struct {
	Scope        string              `json:"scope"`
	IDField      string              `json:"id_field,omitempty"`
	Edges        []GraphTraverseEdge `json:"edges"`
	CycleSources *[]string           `json:"cycle_sources,omitempty"`
	Start        []string            `json:"start,omitempty"`
	Direction    string              `json:"direction,omitempty"`
	MaxDepth     *int                `json:"max_depth,omitempty"`
	Include      []string            `json:"include,omitempty"`
	LimitNodes   *int                `json:"limit_nodes,omitempty"`
}

// GraphTraverse POSTs /graph/traverse/. HTTP 400 bodies are returned as the
// error text: a JSON error string, or the plugin text after a canned status
// line in message.
func (c *Client) GraphTraverse(ctx context.Context, req GraphTraverseRequest) (json.RawMessage, error) {
	raw, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	h := http.Header{}
	h.Set("Content-Type", mimeJSON)
	opt := RequestOptions{Method: http.MethodPost, Path: graphTraversePath, BodyString: string(raw), Headers: h}
	status, b, err := c.Do(ctx, opt)
	if status == http.StatusBadRequest {
		return nil, errors.New(pluginErrorMessage(b, "graph traverse rejected"))
	}
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}
