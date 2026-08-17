package tools

import (
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestPaginateBytesFullSlice(t *testing.T) {
	slice, start, end, total, hasMore := paginateBytes("hello", 32, 0)
	require.Equal(t, "hello", slice)
	require.Equal(t, 0, start)
	require.Equal(t, 5, end)
	require.Equal(t, 5, total)
	require.False(t, hasMore)
}

func TestPaginateBytesWindow(t *testing.T) {
	content := "abcdefghijklmnopqrstuvwxyz"
	slice, start, end, total, hasMore := paginateBytes(content, 5, 10)
	require.Equal(t, "klmno", slice)
	require.Equal(t, 10, start)
	require.Equal(t, 15, end)
	require.Equal(t, 26, total)
	require.True(t, hasMore)
}

func TestPaginateBytesClampsStart(t *testing.T) {
	slice, start, end, total, hasMore := paginateBytes("abc", 10, 100)
	require.Equal(t, "", slice)
	require.Equal(t, 3, start)
	require.Equal(t, 3, end)
	require.Equal(t, 3, total)
	require.False(t, hasMore)
}

func TestPaginateBytesNegativeBounds(t *testing.T) {
	slice, start, end, total, hasMore := paginateBytes("abcd", -5, -2)
	require.Equal(t, "", slice)
	require.Equal(t, 0, start)
	require.Equal(t, 0, end)
	require.Equal(t, 4, total)
	require.True(t, hasMore)
}

func TestPaginatedTextResultNotice(t *testing.T) {
	res := paginatedTextResult("Notes/big.md", "abcd", 0, 4, 100, true)
	txt := res.Content[0].(*mcp.TextContent).Text
	require.Contains(t, txt, "abcd")
	require.Contains(t, txt, "file too large (100 bytes)")
	require.Contains(t, txt, "startIndex=4")
	structured := res.StructuredContent.(map[string]any)
	require.Equal(t, "Notes/big.md", structured["path"])
	pagination := structured["pagination"].(map[string]any)
	require.Equal(t, 100, pagination["totalLength"])
	require.Equal(t, true, pagination["hasMore"])
}

func TestPaginatedTextResult2CapsBody(t *testing.T) {
	body := strings.Repeat("x", defaultVaultReadMaxLength+100)
	maxLen := 64
	start := 0
	res := paginatedTextResult2("File patched successfully", "A.md", body, &maxLen, &start)
	require.Equal(t, "File patched successfully", res.Content[0].(*mcp.TextContent).Text)
	second := res.Content[1].(*mcp.TextContent).Text
	require.Contains(t, second, "file too large")
	require.Less(t, len(second), 50*1024)
}
