package obsidian

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPatchHeadersSendsMarkdownPatchVersion(t *testing.T) {
	t.Parallel()
	h := patchHeaders(PatchParams{Operation: "replace", TargetType: "frontmatter", Target: "author", Content: "Ada"})
	require.Equal(t, "1", h.Get("Markdown-Patch-Version"))
	require.Equal(t, "replace", h.Get("Operation"))
	require.Equal(t, "frontmatter", h.Get("Target-Type"))
	require.Equal(t, "author", h.Get("Target"))
	require.Equal(t, "true", h.Get("Create-Target-If-Missing"))
}

func TestPatchVaultFileSendsMarkdownPatchVersion(t *testing.T) {
	t.Parallel()
	var gotVersion, gotOp, gotType, gotTarget string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPatch, r.Method)
		require.Equal(t, "/vault/Clippings/clip.md", r.URL.EscapedPath())
		gotVersion = r.Header.Get("Markdown-Patch-Version")
		gotOp = r.Header.Get("Operation")
		gotType = r.Header.Get("Target-Type")
		gotTarget = r.Header.Get("Target")
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := NewClientFromURL(u, "secret", ts.Client())
	out, err := cli.PatchVaultFile(context.Background(), "Clippings/clip.md", PatchParams{
		Operation:  "replace",
		TargetType: "frontmatter",
		Target:     "description",
		Content:    "clip text",
	})
	require.NoError(t, err)
	require.Equal(t, "ok", out)
	require.Equal(t, "1", gotVersion)
	require.Equal(t, "replace", gotOp)
	require.Equal(t, "frontmatter", gotType)
	require.Equal(t, "description", gotTarget)
}
