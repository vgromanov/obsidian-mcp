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

func TestGraphTraversePassthrough(t *testing.T) {
	t.Parallel()
	var sawMethod, sawPath, sawType, sawBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawMethod = r.Method
		sawPath = r.URL.Path
		sawType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		sawBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"nodes":[{"id":"a","path":"Notes/a.md","depth":0,"fields":{}}],"edges":[],"unresolved":[],"conflicts":[],"cycles":[],"truncated":false,"index_ready":true}`))
	}))
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := NewClientFromURL(u, "secret", ts.Client())

	limit := 10
	depth := 2
	embeds := false
	raw, err := cli.GraphTraverse(context.Background(), GraphTraverseRequest{
		Scope:   "Notes",
		IDField: "id",
		Edges: []GraphTraverseEdge{{
			Source:   "depends_on",
			Sections: []string{"Definition"},
			Embeds:   &embeds,
		}},
		Start:      []string{"n1"},
		Direction:  "in",
		MaxDepth:   &depth,
		Include:    []string{"status"},
		LimitNodes: &limit,
	})
	require.NoError(t, err)
	require.Equal(t, http.MethodPost, sawMethod)
	require.Equal(t, "/graph/traverse/", sawPath)
	require.Equal(t, "application/json", sawType)
	require.JSONEq(t, `{
		"scope":"Notes",
		"id_field":"id",
		"edges":[{"source":"depends_on","sections":["Definition"],"embeds":false}],
		"start":["n1"],
		"direction":"in",
		"max_depth":2,
		"include":["status"],
		"limit_nodes":10
	}`, sawBody)
	require.Contains(t, string(raw), `"index_ready":true`)
}

func TestGraphTraverseOmitsOptionalFields(t *testing.T) {
	t.Parallel()
	var sawBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		sawBody = string(b)
		_, _ = w.Write([]byte(`{"nodes":[],"edges":[],"truncated":false,"index_ready":true}`))
	}))
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	cli := NewClientFromURL(u, "secret", ts.Client())

	_, err = cli.GraphTraverse(context.Background(), GraphTraverseRequest{
		Scope:     "",
		IDField:   "id",
		Edges:     []GraphTraverseEdge{{Source: "$body"}},
		Direction: "out",
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"scope":"","id_field":"id","edges":[{"source":"$body"}],"direction":"out"}`, sawBody)
}

func TestGraphTraverse400Passthrough(t *testing.T) {
	t.Parallel()
	const parser = "scope has no notes"
	cases := []struct {
		name string
		body string
		want string
	}{
		{"error field", `{"ok":false,"error":"` + parser + `","status":400}`, parser},
		{"canned message", `{"message":"Bad Request\n` + parser + `","errorCode":40000}`, parser},
		{"plain message", `{"message":"` + parser + `"}`, parser},
		{"raw text", parser, parser},
		{"empty", ``, "graph traverse rejected"},
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
			_, err = cli.GraphTraverse(context.Background(), GraphTraverseRequest{
				Scope: "Notes", IDField: "id", Edges: []GraphTraverseEdge{{Source: "depends_on"}}, Direction: "out",
			})
			require.EqualError(t, err, tc.want)
		})
	}
}
