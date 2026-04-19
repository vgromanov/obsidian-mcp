package fetch

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsLikelyHTML(t *testing.T) {
	require.True(t, IsLikelyHTML("<html><body>x</body></html>", "text/plain"))
	require.True(t, IsLikelyHTML("hello", "text/html"))
	require.True(t, IsLikelyHTML("plain", "")) // upstream treats missing Content-Type as HTML-ish
	require.False(t, IsLikelyHTML(`{"a":1}`, "application/json"))
}

func TestHTMLToMarkdown_basic(t *testing.T) {
	md, err := HTMLToMarkdown("<p>Hello <strong>world</strong></p>")
	require.NoError(t, err)
	require.Contains(t, md, "Hello")
	require.Contains(t, md, "world")
}
