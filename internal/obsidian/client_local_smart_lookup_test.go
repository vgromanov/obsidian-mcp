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

func TestSearchVaultLocal(t *testing.T) {
	tests := []struct {
		name string
		body map[string]any
		want string
	}{
		{
			name: "baseline",
			body: map[string]any{"query": "foo", "limit": 5},
			want: `{"limit":5,"query":"foo"}`,
		},
		{
			name: "tags",
			body: map[string]any{"query": "q", "tags": []string{"research", "ai"}, "limit": 5},
			want: `{"limit":5,"query":"q","tags":["research","ai"]}`,
		},
		{
			name: "frontmatter",
			body: map[string]any{
				"query": "q",
				"frontmatter": map[string]any{"status": "active", "project": "Local Search"},
			},
			want: `{"frontmatter":{"project":"Local Search","status":"active"},"query":"q"}`,
		},
		{
			name: "dataviewSource",
			body: map[string]any{"query": "q", "dataviewSource": `#research or "Projects"`},
			want: `{"dataviewSource":"#research or \"Projects\"","query":"q"}`,
		},
		{
			name: "dataviewQuery",
			body: map[string]any{"query": "q", "dataviewQuery": `LIST FROM #research WHERE status = "active"`},
			want: `{"dataviewQuery":"LIST FROM #research WHERE status = \"active\"","query":"q"}`,
		},
		{
			name: "where",
			body: map[string]any{"query": "q", "where": "type = 'note'"},
			want: `{"query":"q","where":"type = 'note'"}`,
		},
		{
			name: "combined",
			body: map[string]any{
				"query":       "q",
				"tags":        []string{"research"},
				"frontmatter": map[string]any{"status": "active"},
				"where":       "type = 'note'",
			},
			want: `{"frontmatter":{"status":"active"},"query":"q","tags":["research"],"where":"type = 'note'"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var sawPath, sawAuth, sawCT, sawBody string
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				sawPath = r.URL.Path
				sawAuth = r.Header.Get("Authorization")
				sawCT = r.Header.Get("Content-Type")
				b, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				sawBody = string(b)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"results":[]}`))
			}))
			t.Cleanup(ts.Close)

			u, err := url.Parse(ts.URL)
			require.NoError(t, err)
			cli := NewClientFromURL(u, "secret", ts.Client())

			raw, err := cli.SearchVaultLocal(context.Background(), tc.body)
			require.NoError(t, err)
			require.JSONEq(t, `{"results":[]}`, string(raw))

			require.Equal(t, "/local-smart-lookup/search/", sawPath)
			require.Equal(t, "Bearer secret", sawAuth)
			require.Equal(t, mimeJSON, sawCT)

			var got map[string]any
			require.NoError(t, json.Unmarshal([]byte(sawBody), &got))
			var want map[string]any
			require.NoError(t, json.Unmarshal([]byte(tc.want), &want))
			require.Equal(t, want, got)
		})
	}
}
