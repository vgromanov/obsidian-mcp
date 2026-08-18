package obsidian

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCapsForVersion(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name           string
		version        string
		move           bool
		dataview       bool
		periodic       bool
		wantVersionSet bool
	}{
		{"empty", "", false, true, true, false},
		{"garbage", "not-a-version", false, true, true, false},
		{"3.6.1", "3.6.1", false, true, true, true},
		{"3.6.0", "3.6.0", false, true, true, true},
		{"4.0.0", "4.0.0", false, false, true, true},
		{"4.0.2", "4.0.2", false, false, true, true},
		{"4.1.0", "4.1.0", true, false, true, true},
		{"4.1.7", "4.1.7", true, false, true, true},
		{"4.2.0", "4.2.0", true, false, true, true},
		{"5.0.0", "5.0.0", false, true, true, true},
		{"5.1.0", "5.1.0", false, true, true, true},
		{"v4.1.7-beta", "v4.1.7-beta", true, false, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := CapsForVersion(tc.version)
			require.Equal(t, tc.move, c.MoveVaultFile)
			require.Equal(t, tc.dataview, c.RestDataviewDQL)
			require.Equal(t, tc.periodic, c.Periodic)
			if tc.wantVersionSet {
				require.NotEmpty(t, c.Version)
			} else {
				require.Empty(t, c.Version)
			}
		})
	}
}

func TestParseServerInfoVersion(t *testing.T) {
	t.Parallel()
	self := []byte(`{"versions":{"self":"4.1.7","obsidian":"1.8.0"},"manifest":{"version":"4.1.0"}}`)
	v, err := ParseServerInfoVersion(self)
	require.NoError(t, err)
	require.Equal(t, "4.1.7", v)

	manifestOnly := []byte(`{"manifest":{"version":"3.6.1"}}`)
	v, err = ParseServerInfoVersion(manifestOnly)
	require.NoError(t, err)
	require.Equal(t, "3.6.1", v)

	_, err = ParseServerInfoVersion([]byte(`{"status":"ok"}`))
	require.Error(t, err)
}

func TestProbeCapsNilClient(t *testing.T) {
	t.Parallel()
	c := ProbeCaps(context.Background(), nil)
	require.Equal(t, Safe36Caps(), c)
}

func TestProbeCapsFailClosedOn404(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := NewClientFromURL(u, "secret", ts.Client())
	c := ProbeCaps(context.Background(), cli)
	require.Equal(t, Safe36Caps(), c)
}

func TestProbeCapsParsesVersionsSelf(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"versions": map[string]string{"self": "4.1.7"},
		})
	}))
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := NewClientFromURL(u, "secret", ts.Client())
	c := ProbeCaps(context.Background(), cli)
	require.Equal(t, "4.1.7", c.Version)
	require.True(t, c.MoveVaultFile)
	require.False(t, c.RestDataviewDQL)
}
