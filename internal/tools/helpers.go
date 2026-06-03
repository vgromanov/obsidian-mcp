package tools

import (
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func textResult(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}
}

func textResult2(a, b string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: a}, &mcp.TextContent{Text: b}}}
}

// jsonResult returns JSON API payloads in MCP structuredContent (required to be
// a JSON object) with compact JSON text in Content for backward compatibility.
func jsonResult(raw []byte) *mcp.CallToolResult {
	if len(raw) == 0 {
		return textResult("")
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return textResult(string(raw))
	}
	structured := toStructuredObject(v)
	text, err := json.Marshal(structured)
	if err != nil {
		return textResult(string(raw))
	}
	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: string(text)}},
		StructuredContent: structured,
	}
}

func toStructuredObject(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{"data": v}
}
