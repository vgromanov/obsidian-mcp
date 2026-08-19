package obsidian

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVaultCRUDNestedPathsUseSegmentEscape(t *testing.T) {
	t.Parallel()

	type hit struct {
		method string
		path   string
	}
	var hits []hit

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escaped := r.URL.EscapedPath()
		require.NotContains(t, escaped, "%2F", "REST 5.x rejects full-path PathEscape")
		require.Equal(t, "/vault/Knowledge/DECISION.md", escaped)
		hits = append(hits, hit{method: r.Method, path: escaped})
		_, _ = io.Copy(io.Discard, r.Body)
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "text/markdown")
			_, _ = w.Write([]byte("# ok\n"))
		case http.MethodPatch:
			w.Header().Set("Content-Type", "text/markdown")
			_, _ = w.Write([]byte("patched\n"))
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := NewClientFromURL(u, "secret", ts.Client())
	ctx := context.Background()
	filename := "Knowledge/DECISION.md"

	body, err := cli.GetVaultFile(ctx, filename, false)
	require.NoError(t, err)
	require.Equal(t, "# ok\n", string(body))

	require.NoError(t, cli.CreateVaultFile(ctx, filename, "x"))
	require.NoError(t, cli.AppendVaultFile(ctx, filename, "y"))
	out, err := cli.PatchVaultFile(ctx, filename, PatchParams{Operation: "append", TargetType: "heading", Target: "H", Content: "z"})
	require.NoError(t, err)
	require.Equal(t, "patched\n", out)
	require.NoError(t, cli.DeleteVaultFile(ctx, filename))

	require.Equal(t, []hit{
		{http.MethodGet, "/vault/Knowledge/DECISION.md"},
		{http.MethodPut, "/vault/Knowledge/DECISION.md"},
		{http.MethodPost, "/vault/Knowledge/DECISION.md"},
		{http.MethodPatch, "/vault/Knowledge/DECISION.md"},
		{http.MethodDelete, "/vault/Knowledge/DECISION.md"},
	}, hits)
}

func TestVaultCRUDUnicodeSegmentStillEscaped(t *testing.T) {
	t.Parallel()
	var gotEscaped string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEscaped = r.URL.EscapedPath()
		require.NotContains(t, gotEscaped, "%2F")
		w.Header().Set("Content-Type", "text/markdown")
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := NewClientFromURL(u, "k", ts.Client())
	_, err = cli.GetVaultFile(context.Background(), "dir/résumé.md", false)
	require.NoError(t, err)
	require.Equal(t, "/vault/dir/r%C3%A9sum%C3%A9.md", gotEscaped)
}

func TestListVaultFilesNestedDirectoryUnchanged(t *testing.T) {
	t.Parallel()
	var gotEscaped string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEscaped = r.URL.EscapedPath()
		require.NotContains(t, gotEscaped, "%2F")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"files":[]}`))
	}))
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := NewClientFromURL(u, "k", ts.Client())
	_, err = cli.ListVaultFiles(context.Background(), "Knowledge/subdir")
	require.NoError(t, err)
	require.Equal(t, "/vault/Knowledge/subdir/", gotEscaped)
}
