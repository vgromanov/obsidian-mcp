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

func TestQueryDataview(t *testing.T) {
	t.Parallel()
	var sawMethod, sawPath, sawType, sawBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawMethod = r.Method
		sawPath = r.URL.Path
		sawType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		sawBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"type":"list","items":["Notes/a.md"],"truncated":false,"index_ready":true}`))
	}))
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := NewClientFromURL(u, "secret", ts.Client())

	limit := 25
	raw, err := cli.QueryDataview(context.Background(), `LIST FROM "Notes"`, &limit)
	require.NoError(t, err)
	require.Equal(t, http.MethodPost, sawMethod)
	require.Equal(t, "/dataview/query/", sawPath)
	require.Equal(t, "application/json", sawType)
	require.JSONEq(t, `{"query":"LIST FROM \"Notes\"","limit":25}`, sawBody)
	require.Contains(t, string(raw), `"type":"list"`)
}

func TestQueryDataviewOmitsLimit(t *testing.T) {
	t.Parallel()
	var sawBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		sawBody = string(b)
		_, _ = w.Write([]byte(`{"type":"table","headers":[],"rows":[]}`))
	}))
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := NewClientFromURL(u, "secret", ts.Client())

	_, err = cli.QueryDataview(context.Background(), "TABLE file.name", nil)
	require.NoError(t, err)
	require.JSONEq(t, `{"query":"TABLE file.name"}`, sawBody)
}

func TestQueryDataview400Passthrough(t *testing.T) {
	t.Parallel()
	const parser = "Expected one of: TABLE, LIST, TASK, CALENDAR at line 1, column 4"
	cases := []struct {
		name string
		body string
		want string
	}{
		{"error field", `{"ok":false,"error":"` + parser + `","status":400}`, parser},
		{"canned message", `{"message":"Bad Request\n` + parser + `","errorCode":40000}`, parser},
		{"plain message", `{"message":"` + parser + `"}`, parser},
		{"raw text", parser, parser},
		{"empty", ``, "Dataview query rejected"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(ts.Close)
			u, err := url.Parse(ts.URL)
			require.NoError(t, err)
			cli := NewClientFromURL(u, "secret", ts.Client())
			_, err = cli.QueryDataview(context.Background(), "nope", nil)
			require.EqualError(t, err, tc.want)
		})
	}
}
