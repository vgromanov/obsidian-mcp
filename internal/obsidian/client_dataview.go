package obsidian

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// dataviewQueryPath is the Local Smart Lookup read-only DQL route.
const dataviewQueryPath = "/dataview/query/"

type dataviewQueryRequest struct {
	Query string `json:"query"`
	Limit *int   `json:"limit,omitempty"`
}

// QueryDataview POST /dataview/query/ with {query, limit}. Limit is omitted
// when nil. HTTP 400 bodies are returned as the error text: a JSON error
// string, or the plugin text after a canned status line in message.
func (c *Client) QueryDataview(ctx context.Context, query string, limit *int) (json.RawMessage, error) {
	raw, err := json.Marshal(dataviewQueryRequest{Query: query, Limit: limit})
	if err != nil {
		return nil, err
	}
	h := http.Header{}
	h.Set("Content-Type", mimeJSON)
	opt := RequestOptions{Method: http.MethodPost, Path: dataviewQueryPath, BodyString: string(raw), Headers: h}
	status, b, err := c.Do(ctx, opt)
	if status == http.StatusBadRequest {
		return nil, errors.New(pluginErrorMessage(b, "Dataview query rejected"))
	}
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

// pluginErrorMessage extracts a plugin 400 body.
// Preference: JSON "error" (plugin fallback body), then the text after the
// first newline of JSON "message" (Local REST canned "Bad Request\n…"), then
// the whole message, then the raw body, then fallback.
func pluginErrorMessage(body []byte, fallback string) string {
	var payload struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		if msg := strings.TrimSpace(payload.Error); msg != "" {
			return msg
		}
		if msg := strings.TrimSpace(payload.Message); msg != "" {
			if i := strings.Index(msg, "\n"); i >= 0 {
				if rest := strings.TrimSpace(msg[i+1:]); rest != "" {
					return rest
				}
			}
			return msg
		}
	}
	if msg := strings.TrimSpace(string(body)); msg != "" {
		return msg
	}
	if msg := strings.TrimSpace(fallback); msg != "" {
		return msg
	}
	return "request rejected"
}
