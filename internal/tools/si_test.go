package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/vgromanov/obsidian-mcp/internal/obsidian"
)

func TestResolveSIQueryXOR(t *testing.T) {
	cli := obsidian.NewClientFromURL(&url.URL{Scheme: "http", Host: "example"}, "k", http.DefaultClient)

	_, _, err := resolveSIQuery(context.Background(), cli, nil, nil, nil)
	require.Error(t, err)

	text := "hi"
	chunk := "a#b#0"
	_, _, err = resolveSIQuery(context.Background(), cli, &text, []float64{1}, &chunk)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exactly one")
}

func TestRequireSIToolCorpusWhere(t *testing.T) {
	require.Error(t, requireSIToolCorpusWhere(""))
	require.Error(t, requireSIToolCorpusWhere("workspace = 'x'"))
	require.NoError(t, requireSIToolCorpusWhere("type = 'note'"))
}

func TestResolveSIQueryTextEmbedsWithoutEcho(t *testing.T) {
	var paths []string
	var knnBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		b, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/si/embed_text/":
			_, _ = w.Write([]byte(`{"vectors":[[0.5,0.25]],"embed_dim":2}`))
		case "/si/knn/":
			require.NoError(t, json.Unmarshal(b, &knnBody))
			_, _ = w.Write([]byte(`{"hits":[{"chunk_id":"x","distance":0.1}],"k":3}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := obsidian.NewClientFromURL(u, "secret", ts.Client())

	text := "query text"
	vec, chunk, err := resolveSIQuery(context.Background(), cli, &text, nil, nil)
	require.NoError(t, err)
	require.Empty(t, chunk)
	require.Equal(t, []float64{0.5, 0.25}, vec)
	require.Equal(t, []string{"/si/embed_text/"}, paths)

	// Ensure knn path uses vector and response has no query vector field.
	paths = nil
	raw, err := cli.SIKnn(context.Background(), obsidian.SIKnnParams{
		Vector: vec,
		Where:  "type = 'note'",
	})
	require.NoError(t, err)
	require.Equal(t, []string{"/si/knn/"}, paths)
	require.Equal(t, []any{0.5, 0.25}, knnBody["vector"])
	require.NotContains(t, string(raw), `"query_vector"`)
	require.NotContains(t, string(raw), `"vectors"`)
}

// TestSIToolInputSchemasAreObjects guards against Go `any` / interface{} fields
// inferring JSON Schema literal `true` (breaks strict MCP clients like Grok Bot).
func TestSIToolInputSchemasAreObjects(t *testing.T) {
	cli := obsidian.NewClientFromURL(&url.URL{Scheme: "http", Host: "example"}, "k", http.DefaultClient)
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	RegisterSi(server, Deps{Client: cli})

	ctx := context.Background()
	t1, t2 := mcp.NewInMemoryTransports()
	_, err := server.Connect(ctx, t1, nil)
	require.NoError(t, err)
	client := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "0"}, nil)
	session, err := client.Connect(ctx, t2, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })

	var names []string
	for tool, err := range session.Tools(ctx, nil) {
		require.NoError(t, err)
		names = append(names, tool.Name)
		require.NotNil(t, tool.InputSchema, tool.Name)
		raw, err := json.Marshal(tool.InputSchema)
		require.NoError(t, err)
		var schema map[string]any
		require.NoError(t, json.Unmarshal(raw, &schema))
		assertSchemaPropertiesAreObjects(t, tool.Name, schema)
		if tool.Name == "si_query_metadata" || tool.Name == "si_get_vectors" {
			props, _ := schema["properties"].(map[string]any)
			_, hasOffset := props["offset"]
			require.False(t, hasOffset, "%s must not advertise unsupported offset (any→schema true)", tool.Name)
		}
	}
	require.ElementsMatch(t, []string{
		"si_health", "si_index_info", "si_embed_text", "si_query_metadata",
		"si_knn", "si_count_neighbors", "si_get_vectors", "si_filter_validate",
	}, names)
}

func assertSchemaPropertiesAreObjects(t *testing.T, toolName string, schema map[string]any) {
	t.Helper()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		return
	}
	for name, raw := range props {
		prop, ok := raw.(map[string]any)
		require.Truef(t, ok, "%s.properties.%s must be a schema object, got %T (%v)", toolName, name, raw, raw)
		_, hasType := prop["type"]
		_, hasRef := prop["$ref"]
		_, hasAnyOf := prop["anyOf"]
		_, hasOneOf := prop["oneOf"]
		_, hasAllOf := prop["allOf"]
		require.Truef(t, hasType || hasRef || hasAnyOf || hasOneOf || hasAllOf,
			"%s.properties.%s schema object missing type/composition: %v", toolName, name, prop)
	}
}
