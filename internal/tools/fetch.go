package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vgromanov/obsidian-mcp/internal/fetch"
)

const fetchUserAgent = "obsidian-mcp/0.1 (+https://github.com/vgromanov/obsidian-mcp)"

// RegisterFetch registers the fetch tool (parity with upstream fetch feature).
func RegisterFetch(s *mcp.Server) {
	type fetchIn struct {
		URL        string `json:"url"`
		MaxLength  *int   `json:"maxLength,omitempty"`
		StartIndex *int   `json:"startIndex,omitempty"`
		Raw        *bool  `json:"raw,omitempty"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "fetch",
		Description: "Reads and returns the content of any web page. Returns the content in Markdown format by default, or can return raw HTML if raw=true parameter is set. Supports pagination through maxLength and startIndex parameters.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in fetchIn) (*mcp.CallToolResult, *void, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, in.URL, nil)
		if err != nil {
			return nil, nil, err
		}
		req.Header.Set("User-Agent", fetchUserAgent)
		hc := &http.Client{Timeout: 60 * time.Second}
		resp, err := hc.Do(req)
		if err != nil {
			return nil, nil, err
		}
		defer resp.Body.Close()
		textBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, nil, fmt.Errorf("failed to fetch %s - status code %d", in.URL, resp.StatusCode)
		}
		ct := resp.Header.Get("Content-Type")
		body := string(textBody)
		isHTML := fetch.IsLikelyHTML(body, ct)
		raw := in.Raw != nil && *in.Raw
		prefix := ""
		var content string
		if isHTML && !raw {
			md, err := fetch.HTMLToMarkdown(body)
			if err != nil {
				return nil, nil, fmt.Errorf("html to markdown: %w", err)
			}
			content = md
		} else {
			content = body
			prefix = fmt.Sprintf("Content type %s cannot be simplified to markdown, but here is the raw content:\n", ct)
		}
		maxLen := 5000
		if in.MaxLength != nil {
			maxLen = *in.MaxLength
		}
		start := 0
		if in.StartIndex != nil {
			start = *in.StartIndex
		}
		totalLen := len(content)
		if totalLen > maxLen {
			end := start + maxLen
			if end > len(content) {
				end = len(content)
			}
			if start > len(content) {
				start = len(content)
			}
			content = content[start:end]
			content += fmt.Sprintf("\n\n<error>Content truncated. Call the fetch tool with a startIndex of %d to get more content.</error>", start+maxLen)
		}
		first := prefix + "Contents of " + in.URL + ":\n" + content
		// Upstream always sets hasMore: true in pagination JSON (quirky); mirror it.
		pag := fmt.Sprintf(`Pagination: {"totalLength":%d,"startIndex":%d,"endIndex":%d,"hasMore":true}`,
			totalLen, start, start+len(content))
		return textResult2(strings.TrimSpace(first), pag), nil, nil
	})
}
