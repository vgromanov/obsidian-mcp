package omlx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/models", r.URL.Path)
		require.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(ts.Close)

	err := Check(context.Background(), ts.URL+"/v1", "tok")
	require.NoError(t, err)
}

func TestCheckNoAuthWhenKeyEmpty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Empty(t, r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(ts.Close)

	err := Check(context.Background(), ts.URL+"/v1", "")
	require.NoError(t, err)
}

func TestCheckUnreachable(t *testing.T) {
	err := Check(context.Background(), "http://127.0.0.1:1/v1", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot reach oMLX")
}

func TestCheckBadStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(ts.Close)

	err := Check(context.Background(), ts.URL+"/v1", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "returned HTTP 503")
}
