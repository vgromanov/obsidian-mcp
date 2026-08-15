package obsidian

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSIHealth(t *testing.T) {
	var sawPath, sawAuth, sawMethod string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		sawAuth = r.Header.Get("Authorization")
		sawMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"chunks":10}`))
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := NewClientFromURL(u, "secret", ts.Client())

	raw, err := cli.SIHealth(context.Background())
	require.NoError(t, err)
	require.Equal(t, http.MethodGet, sawMethod)
	require.Equal(t, "/si/health/", sawPath)
	require.Equal(t, "Bearer secret", sawAuth)
	require.Contains(t, string(raw), `"ok":true`)
}

func TestSIIndexInfo(t *testing.T) {
	var sawPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		require.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"embed_dim":2560,"si_applies_rerank":false}`))
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := NewClientFromURL(u, "secret", ts.Client())

	raw, err := cli.SIIndexInfo(context.Background())
	require.NoError(t, err)
	require.Equal(t, "/si/index_info/", sawPath)
	require.Contains(t, string(raw), `"si_applies_rerank":false`)
}

func TestSIEmbedText(t *testing.T) {
	var sawPath, body string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		require.Equal(t, http.MethodPost, r.Method)
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"vectors":[[0.1,0.2]],"embed_dim":2}`))
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := NewClientFromURL(u, "secret", ts.Client())

	raw, err := cli.SIEmbedText(context.Background(), map[string]any{
		"texts":     []string{"hello"},
		"normalize": true,
	})
	require.NoError(t, err)
	require.Equal(t, "/si/embed_text/", sawPath)
	require.Contains(t, body, `"texts"`)
	require.Contains(t, string(raw), `"vectors"`)
}

func TestSIQueryMetadata(t *testing.T) {
	var sawPath, body string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		require.Equal(t, http.MethodPost, r.Method)
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"rows":[],"next_cursor":null}`))
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := NewClientFromURL(u, "secret", ts.Client())

	limit := 10
	raw, err := cli.SIQueryMetadata(context.Background(), SIQueryMetadataParams{
		Where:  "type = 'note'",
		Fields: []string{"path", "type"},
		Limit:  &limit,
	})
	require.NoError(t, err)
	require.Equal(t, "/si/query_metadata/", sawPath)
	require.NotContains(t, body, `"offset"`)
	require.Contains(t, string(raw), `"rows"`)
}

func TestSIKnnRequiresTypeWhere(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("should not call HTTP without type filter")
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := NewClientFromURL(u, "secret", ts.Client())

	_, err = cli.SIKnn(context.Background(), SIKnnParams{
		Vector: []float64{0.1},
		Where:  "workspace = 'x'",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "corpus-type")
}

func TestSIKnnPostsVector(t *testing.T) {
	var sawPath string
	var got map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		require.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":[],"k":5,"metric":"cosine"}`))
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := NewClientFromURL(u, "secret", ts.Client())

	k := 5
	raw, err := cli.SIKnn(context.Background(), SIKnnParams{
		Vector: []float64{0.1, 0.2},
		K:      &k,
		Where:  "type = 'session-summary'",
		Metric: "cosine",
	})
	require.NoError(t, err)
	require.Equal(t, "/si/knn/", sawPath)
	require.Equal(t, "type = 'session-summary'", got["where"])
	require.Contains(t, string(raw), `"hits"`)
}

func TestSICountNeighborsRequiresTypeWhere(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("should not call HTTP")
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := NewClientFromURL(u, "secret", ts.Client())

	_, err = cli.SICountNeighbors(context.Background(), SICountNeighborsParams{
		ChunkID:   "a#b#0",
		Threshold: 0.2,
		GroupBy:   "uuid",
		Where:     "",
	})
	require.Error(t, err)
}

func TestSICountNeighborsPosts(t *testing.T) {
	var sawPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		require.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"total_hits":1,"counts":{"u":1}}`))
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := NewClientFromURL(u, "secret", ts.Client())

	raw, err := cli.SICountNeighbors(context.Background(), SICountNeighborsParams{
		ChunkID:   "p#h#0",
		Threshold: 0.18,
		GroupBy:   "uuid",
		Where:     "type = 'session-transcript'",
	})
	require.NoError(t, err)
	require.Equal(t, "/si/count_neighbors/", sawPath)
	require.Contains(t, string(raw), `"total_hits"`)
}

func TestSIGetVectorsRequiresTypeWhere(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("should not call HTTP")
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := NewClientFromURL(u, "secret", ts.Client())

	_, err = cli.SIGetVectors(context.Background(), SIGetVectorsParams{Where: "project = 'x'"})
	require.Error(t, err)
}

func TestSIGetVectorsPosts(t *testing.T) {
	var sawPath string
	var got map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		require.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"next_cursor":null}`))
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := NewClientFromURL(u, "secret", ts.Client())

	limit := 50
	cursor := "chunk-1"
	raw, err := cli.SIGetVectors(context.Background(), SIGetVectorsParams{
		Where:       "type = 'session-summary'",
		IncludeText: true,
		Limit:       &limit,
		Cursor:      &cursor,
	})
	require.NoError(t, err)
	require.Equal(t, "/si/get_vectors/", sawPath)
	require.Equal(t, true, got["include_text"])
	require.Equal(t, float64(50), got["limit"])
	require.Equal(t, "chunk-1", got["cursor"])
	require.Contains(t, string(raw), `"items"`)
}

func TestSIFilterValidate(t *testing.T) {
	var sawPath string
	var got map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		require.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sql":"type = 'note'","row_count":3,"sample":[]}`))
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := NewClientFromURL(u, "secret", ts.Client())

	where := "type = 'note'"
	limit := 5
	raw, err := cli.SIFilterValidate(context.Background(), SIFilterValidateParams{
		Where:  &where,
		Filter: map[string]any{"eq": []any{"type", "note"}},
		Limit:  &limit,
	})
	require.NoError(t, err)
	require.Equal(t, "/si/filter/validate/", sawPath)
	require.Equal(t, "type = 'note'", got["where"])
	require.Equal(t, float64(5), got["limit"])
	require.Contains(t, got, "filter")
	require.Contains(t, string(raw), `"row_count"`)
}

func TestSIQueryMetadataWithCursor(t *testing.T) {
	var got map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"rows":[]}`))
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := NewClientFromURL(u, "secret", ts.Client())

	cur := "abc"
	_, err = cli.SIQueryMetadata(context.Background(), SIQueryMetadataParams{
		Where:  "type = 'note'",
		Fields: []string{"path"},
		Cursor: &cur,
	})
	require.NoError(t, err)
	require.Equal(t, "abc", got["cursor"])
}

func TestSIKnnWithChunkIDAndThreshold(t *testing.T) {
	var got map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":[]}`))
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := NewClientFromURL(u, "secret", ts.Client())

	th := 0.25
	_, err = cli.SIKnn(context.Background(), SIKnnParams{
		ChunkID:   "p#h#0",
		Threshold: &th,
		Where:     "type = 'note'",
	})
	require.NoError(t, err)
	require.Equal(t, "p#h#0", got["chunk_id"])
	require.Equal(t, 0.25, got["threshold"])
}

func TestSICountNeighborsWithVectorAndMetric(t *testing.T) {
	var got map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"total_hits":0}`))
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := NewClientFromURL(u, "secret", ts.Client())

	_, err = cli.SICountNeighbors(context.Background(), SICountNeighborsParams{
		Vector:    []float64{0.1},
		Threshold: 0.2,
		Metric:    "cosine",
		GroupBy:   "path",
		Where:     "type = 'note'",
	})
	require.NoError(t, err)
	require.Equal(t, "cosine", got["metric"])
	require.Equal(t, "path", got["group_by"])
}
