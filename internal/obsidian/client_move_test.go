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

func TestEncodeVaultRelativePath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"Knowledge/DECISION.md", "Knowledge/DECISION.md"},
		{"/Knowledge/DECISION.md", "Knowledge/DECISION.md"},
		{"Knowledge/DECISION.md/", "Knowledge/DECISION.md"},
		{"note.md", "note.md"},
		{"a/b/c.md", "a/b/c.md"},
		{"dir/résumé.md", "dir/r%C3%A9sum%C3%A9.md"},
		{"", ""},
		{"  ", ""},
		{"/", ""},
	}
	for _, tc := range cases {
		got := EncodeVaultRelativePath(tc.in)
		require.Equal(t, tc.want, got, tc.in)
		require.NotContains(t, got, "%2F", tc.in)
		require.NotContains(t, got, "%2f", tc.in)
	}
}

func TestEncodeVaultRelativeDestination(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"archive/todo.md", "archive/todo.md", false},
		{"archive/", "archive/", false},
		{"résumé.md", "r%C3%A9sum%C3%A9.md", false},
		{"dir/résumé.md", "dir/r%C3%A9sum%C3%A9.md", false},
		{"/abs.md", "", true},
		{"", "", true},
		{"  ", "", true},
	}
	for _, tc := range cases {
		got, err := EncodeVaultRelativeDestination(tc.in)
		if tc.wantErr {
			require.Error(t, err, tc.in)
			continue
		}
		require.NoError(t, err, tc.in)
		require.Equal(t, tc.want, got)
		require.NotContains(t, got, "%2F", tc.in)
	}
}

func TestMoveVaultFileHeaders(t *testing.T) {
	t.Parallel()
	var gotMethod, gotEscapedPath, gotDest, gotOverwrite string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotEscapedPath = r.URL.EscapedPath()
		gotDest = r.Header.Get("Destination")
		gotOverwrite = r.Header.Get("Allow-Overwrite")
		require.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		require.NotContains(t, gotEscapedPath, "%2F")
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Location", "archive/todo.md")
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := NewClientFromURL(u, "secret", ts.Client())

	loc, err := cli.MoveVaultFile(context.Background(), "notes/todo.md", "archive/", false)
	require.NoError(t, err)
	require.Equal(t, "archive/todo.md", loc)
	require.Equal(t, "MOVE", gotMethod)
	require.Equal(t, "/vault/notes/todo.md", gotEscapedPath)
	require.Equal(t, "archive/", gotDest)
	require.Equal(t, "false", gotOverwrite)
}

func TestMoveVaultFileAllowOverwrite(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "true", r.Header.Get("Allow-Overwrite"))
		require.Equal(t, "r%C3%A9sum%C3%A9.md", r.Header.Get("Destination"))
		w.Header().Set("Content-Location", "r%C3%A9sum%C3%A9.md")
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := NewClientFromURL(u, "secret", ts.Client())
	_, err = cli.MoveVaultFile(context.Background(), "old.md", "résumé.md", true)
	require.NoError(t, err)
}

func TestMoveVaultFileRejectsAbsoluteDestination(t *testing.T) {
	t.Parallel()
	cli := NewClientFromURL(&url.URL{Scheme: "http", Host: "example"}, "k", http.DefaultClient)
	_, err := cli.MoveVaultFile(context.Background(), "a.md", "/abs.md", false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "vault-relative")
}
