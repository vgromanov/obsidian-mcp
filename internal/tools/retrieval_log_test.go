package tools

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLogRetrievalAppendsEvent(t *testing.T) {
	dir := t.TempDir()
	raw := json.RawMessage(`{"results":[{"path":"A.md","score":0.9,"rerankScore":0.5},{"path":"B.md","score":0.7}]}`)
	body := map[string]any{"query": "q", "limit": 5, "tags": []string{"x"}}

	logRetrieval(dir, "regime-v1", "q", body, raw)

	shard := filepath.Join(dir, shortHost()+".jsonl")
	data, err := os.ReadFile(shard)
	require.NoError(t, err)

	var ev retrievalEvent
	require.NoError(t, json.Unmarshal(data, &ev))
	require.Equal(t, "q", ev.Query)
	require.Equal(t, "regime-v1", ev.Regime)
	require.NotEmpty(t, ev.Host)
	require.Len(t, ev.Returned, 2)
	require.Equal(t, "A.md", ev.Returned[0].Path)
	require.Equal(t, 0, ev.Returned[0].Rank)
	require.NotNil(t, ev.Returned[0].RerankScore)
	require.Nil(t, ev.Returned[1].RerankScore)
	// query must not be duplicated into params
	_, hasQuery := ev.Params["query"]
	require.False(t, hasQuery)
	require.Equal(t, float64(5), ev.Params["limit"])
}

func TestLogRetrievalDisabledWhenDirEmpty(t *testing.T) {
	// Must not panic or error with an empty dir; simply a no-op.
	logRetrieval("", "", "q", map[string]any{"query": "q"}, json.RawMessage(`{"results":[]}`))
}

func TestLogRetrievalAppendsMultiple(t *testing.T) {
	dir := t.TempDir()
	raw := json.RawMessage(`{"results":[]}`)
	logRetrieval(dir, "", "first", nil, raw)
	logRetrieval(dir, "", "second", nil, raw)

	shard := filepath.Join(dir, shortHost()+".jsonl")
	data, err := os.ReadFile(shard)
	require.NoError(t, err)

	dec := json.NewDecoder(bytes.NewReader(data))
	var count int
	for dec.More() {
		var ev retrievalEvent
		require.NoError(t, dec.Decode(&ev))
		count++
	}
	require.Equal(t, 2, count)
}
