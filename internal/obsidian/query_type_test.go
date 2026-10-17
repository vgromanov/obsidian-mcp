package obsidian

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseQueryType(t *testing.T) {
	for in, want := range map[string]QueryType{
		"dataview":    QueryTypeDataview,
		"DataView":    QueryTypeDataview,
		"  DATAVIEW ": QueryTypeDataview,
		"jsonlogic":   QueryTypeJSONLogic,
		"JsonLogic":   QueryTypeJSONLogic,
	} {
		got, err := ParseQueryType(in)
		require.NoError(t, err, in)
		require.Equal(t, want, got, in)
	}
	for _, in := range []string{"", "dql", "DQL", "sql", "json"} {
		_, err := ParseQueryType(in)
		require.Error(t, err, in)
		require.Contains(t, err.Error(), `"dataview"`)
		require.Contains(t, err.Error(), `"jsonlogic"`)
	}
}

func TestSearchVaultRejectsInvalidQueryTypeAndSurfacesAPIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errorCode":40012,"message":"Unknown or invalid Content-Type."}`))
	}))
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := NewClientFromURL(u, "k", ts.Client())

	_, err = cli.SearchVault(context.Background(), "dql", "LIST")
	require.ErrorContains(t, err, "invalid queryType")

	_, err = cli.SearchVault(context.Background(), "dataview", "LIST")
	require.Error(t, err)
	require.True(t, IsUnsupportedContentType(err))
	var ae *APIError
	require.True(t, errors.As(err, &ae))
	require.Equal(t, http.StatusBadRequest, ae.Status)
	require.Equal(t, 40012, ae.Code)
	require.Contains(t, err.Error(), "POST /search/: ")
	require.Contains(t, err.Error(), "40012")

	require.False(t, IsUnsupportedContentType(errors.New("boom")))
	require.False(t, IsUnsupportedContentType(newAPIError("GET", "/", 500, []byte("not json"))))
}
