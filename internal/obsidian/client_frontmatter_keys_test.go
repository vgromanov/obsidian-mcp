package obsidian

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListFrontmatterKeys(t *testing.T) {
	var sawPath, sawAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		sawAuth = r.Header.Get("Authorization")
		require.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"name":"workspace","count":2,"type":"text"}]`))
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := NewClientFromURL(u, "secret", ts.Client())

	raw, err := cli.ListFrontmatterKeys(context.Background())
	require.NoError(t, err)
	require.Equal(t, "/frontmatter_keys/", sawPath)
	require.Equal(t, "Bearer secret", sawAuth)
	require.Contains(t, string(raw), `"workspace"`)
}

func TestGetFrontmatterKeyFiles(t *testing.T) {
	var sawPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		require.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"filename":"A.md"}]`))
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := NewClientFromURL(u, "secret", ts.Client())

	raw, err := cli.GetFrontmatterKeyFiles(context.Background(), " workspace ")
	require.NoError(t, err)
	require.Equal(t, "/frontmatter_keys/workspace/", sawPath)
	require.Contains(t, string(raw), `"filename"`)

	_, err = cli.GetFrontmatterKeyFiles(context.Background(), "  ")
	require.Error(t, err)
}
