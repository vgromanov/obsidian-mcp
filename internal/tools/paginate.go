package tools

import (
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Default maxLength for get_vault_file / patch_vault_file responses.
// Keeps the JSON-wrapped MCP payload under Cursor's ~50 KB inline spill.
const defaultVaultReadMaxLength = 32768

// paginateBytes slices content by byte indices (same model as fetch: Go string
// indexing is byte-oriented). start is clamped to [0, len(content)].
// Non-positive maxLength is treated as defaultVaultReadMaxLength so a zero-width
// page cannot stall agents with hasMore + the same startIndex.
func paginateBytes(content string, maxLength, startIndex int) (slice string, start, end, total int, hasMore bool) {
	total = len(content)
	start = startIndex
	if start < 0 {
		start = 0
	}
	if start > total {
		start = total
	}
	if maxLength <= 0 {
		maxLength = defaultVaultReadMaxLength
	}
	end = start + maxLength
	if end > total {
		end = total
	}
	slice = content[start:end]
	hasMore = end < total
	return slice, start, end, total, hasMore
}

func resolvePaginationBounds(maxLength, startIndex *int, defaultMax int) (maxLen, start int) {
	maxLen = defaultMax
	if maxLength != nil {
		maxLen = *maxLength
	}
	if maxLen <= 0 {
		maxLen = defaultMax
	}
	if startIndex != nil {
		start = *startIndex
	}
	return maxLen, start
}

func vaultFileOverflowNotice(totalBytes, nextStartIndex int) string {
	return fmt.Sprintf("file too large (%d bytes); call again with startIndex=%d", totalBytes, nextStartIndex)
}

func paginationStructured(path string, content string, start, end, total int, hasMore bool) map[string]any {
	return map[string]any{
		"path":    path,
		"content": content,
		"size":    total,
		"pagination": map[string]any{
			"totalLength": total,
			"startIndex":  start,
			"endIndex":    end,
			"hasMore":     hasMore,
		},
	}
}

// paginatedTextResult returns a single text content block, appending an overflow
// notice when hasMore. StructuredContent always includes pagination metadata.
func paginatedTextResult(path, slice string, start, end, total int, hasMore bool) *mcp.CallToolResult {
	text := slice
	if hasMore {
		text += "\n\n" + vaultFileOverflowNotice(total, end)
	}
	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: text}},
		StructuredContent: paginationStructured(path, slice, start, end, total, hasMore),
	}
}

// paginatedTextResult2 is like textResult2 but caps the second body with the
// same pagination / overflow notice as get_vault_file.
func paginatedTextResult2(status, path, body string, maxLength, startIndex *int) *mcp.CallToolResult {
	maxLen, start := resolvePaginationBounds(maxLength, startIndex, defaultVaultReadMaxLength)
	slice, start, end, total, hasMore := paginateBytes(body, maxLen, start)
	bodyOut := slice
	if hasMore {
		bodyOut += "\n\n" + vaultFileOverflowNotice(total, end)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: status},
			&mcp.TextContent{Text: bodyOut},
		},
		StructuredContent: paginationStructured(path, slice, start, end, total, hasMore),
	}
}
