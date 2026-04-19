package templater

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseParameters_variants(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []Parameter
	}{
		{
			name: "basic",
			in:   `<% tp.mcpTools.prompt("topic", "Topic text") %>`,
			want: []Parameter{{Name: "topic", Description: "Topic text"}},
		},
		{
			name: "templater_star",
			in:   `<%* tp.mcpTools.prompt("a", "b") %>`,
			want: []Parameter{{Name: "a", Description: "b"}},
		},
		{
			name: "templater_dash",
			in:   `<%- tp.mcpTools.prompt("x", "y") _%>`,
			want: []Parameter{{Name: "x", Description: "y"}},
		},
		{
			name: "single_arg",
			in:   `<% tp.mcpTools.prompt("only") %>`,
			want: []Parameter{{Name: "only", Description: ""}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseParameters(tc.in)
			require.Equal(t, tc.want, got)
		})
	}
}
