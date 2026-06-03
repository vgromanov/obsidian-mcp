package tools

import (
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestJSONResultObject(t *testing.T) {
	res := jsonResult([]byte(`{"status":"ok"}`))
	require.NotNil(t, res.StructuredContent)
	structured, ok := res.StructuredContent.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "ok", structured["status"])

	txt := res.Content[0].(*mcp.TextContent).Text
	require.JSONEq(t, `{"status":"ok"}`, txt)
}

func TestJSONResultArrayWrapped(t *testing.T) {
	res := jsonResult([]byte(`[{"filename":"A.md"}]`))
	structured := res.StructuredContent.(map[string]any)
	data, ok := structured["data"].([]any)
	require.True(t, ok)
	require.Len(t, data, 1)
}

func TestJSONResultInvalidJSON(t *testing.T) {
	res := jsonResult([]byte(`not json`))
	require.Nil(t, res.StructuredContent)
	require.Equal(t, "not json", res.Content[0].(*mcp.TextContent).Text)
}

func TestToStructuredObjectPreservesObject(t *testing.T) {
	in := map[string]any{"a": float64(1)}
	out := toStructuredObject(in)
	require.Equal(t, in, out)
}

func TestJSONResultEmpty(t *testing.T) {
	res := jsonResult(nil)
	require.Empty(t, res.Content[0].(*mcp.TextContent).Text)
	require.Nil(t, res.StructuredContent)
}

func TestJSONResultRoundTrip(t *testing.T) {
	raw := []byte(`{"tags":[{"name":"x","count":2}]}`)
	res := jsonResult(raw)
	b, err := json.Marshal(res.StructuredContent)
	require.NoError(t, err)
	require.JSONEq(t, string(raw), string(b))
}
