package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vgromanov/obsidian-mcp/internal/obsidian"
)

// graphTraversePluginDefaultLimitNodes is the Local Smart Lookup default for
// omitted limit_nodes on POST /graph/traverse/. Policy compares against it
// when the caller does not set the field.
const graphTraversePluginDefaultLimitNodes = 2000

// GraphPolicy is enforced in this process before POST /graph/traverse/.
// Zero AllowBody / AllowFullExport means deny; use DefaultGraphPolicy for the
// permissive local profile, and set GraphPolicySet on Deps when passing a policy.
type GraphPolicy struct {
	ScopePrefixes        []string
	MaxLimitNodes        int
	MaxLimitNodesInvalid bool
	AllowBody            bool
	AllowFullExport      bool
}

// DefaultGraphPolicy allows every scope, any limit_nodes, $body inclusion,
// and start-less whole-scope export.
func DefaultGraphPolicy() GraphPolicy {
	return GraphPolicy{AllowBody: true, AllowFullExport: true}
}

func (d Deps) graphPolicy() GraphPolicy {
	if d.GraphPolicySet {
		return d.GraphPolicy
	}
	return DefaultGraphPolicy()
}

const graphTraverseDescription = "Read-only link-graph closure over a vault folder (POST /graph/traverse/). " +
	"scope is a path prefix; an empty scope is the whole vault. " +
	"id_field is an optional frontmatter field used as the node id when it is a non-empty scalar. Omit it and the route uses the vault path. " +
	"edges[].source is a frontmatter field, or $body for wikilinks in the note body. " +
	"A $body edge includes section, the heading the link sits under (null above the first heading). " +
	"Optional sections keeps only those headings. Optional embeds includes embedded notes. " +
	"direction is out, in, or both. Omit it and the route uses out. max_depth omitted or null walks until the node cap. " +
	"include projects frontmatter fields onto each node; $body in include returns the raw note body. " +
	"Omitting start exports every node and edge in scope. include [\"$body\"] on a whole-scope export is large. " +
	"Read-only candidate for a public tool allowlist; this server does not change that allowlist. " +
	"Policy env (defaults allow the call): OBSIDIAN_GRAPH_SCOPE_ALLOWLIST, OBSIDIAN_GRAPH_MAX_LIMIT_NODES, OBSIDIAN_GRAPH_ALLOW_BODY, OBSIDIAN_GRAPH_ALLOW_FULL_EXPORT."

// RegisterGraph registers graph_traverse. Callers gate this on PluginGraphTraverse.
func RegisterGraph(s *mcp.Server, d Deps) {
	cli := d.Client
	destructive := false
	mcp.AddTool(s, &mcp.Tool{
		Name:        "graph_traverse",
		Description: graphTraverseDescription,
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:    true,
			DestructiveHint: &destructive,
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in graphTraverseIn) (*mcp.CallToolResult, any, error) {
		req, err := prepareGraphTraverse(in, d.graphPolicy())
		if err != nil {
			return nil, nil, err
		}
		raw, err := cli.GraphTraverse(ctx, req)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(raw), nil, nil
	})
}

type graphEdgeIn struct {
	Source   string   `json:"source"`
	Sections []string `json:"sections,omitempty"`
	Embeds   *bool    `json:"embeds,omitempty"`
}

type graphTraverseIn struct {
	Scope      string        `json:"scope"`
	IDField    *string       `json:"id_field,omitempty"`
	Edges      []graphEdgeIn `json:"edges"`
	Start      *[]string     `json:"start,omitempty"`
	Direction  *string       `json:"direction,omitempty"`
	MaxDepth   *int          `json:"max_depth,omitempty"`
	Include    []string      `json:"include,omitempty"`
	LimitNodes *int          `json:"limit_nodes,omitempty"`
}

func prepareGraphTraverse(in graphTraverseIn, policy GraphPolicy) (obsidian.GraphTraverseRequest, error) {
	var idField string
	if in.IDField != nil {
		idField = strings.TrimSpace(*in.IDField)
		if idField == "" {
			return obsidian.GraphTraverseRequest{}, fmt.Errorf("`id_field` must be a non-empty string")
		}
	}
	if len(in.Edges) == 0 {
		return obsidian.GraphTraverseRequest{}, fmt.Errorf("edges must be a non-empty list")
	}
	edges := make([]obsidian.GraphTraverseEdge, len(in.Edges))
	for i, edge := range in.Edges {
		if strings.TrimSpace(edge.Source) == "" {
			return obsidian.GraphTraverseRequest{}, fmt.Errorf("each edge source must be a non-empty string")
		}
		for _, section := range edge.Sections {
			if strings.TrimSpace(section) == "" {
				return obsidian.GraphTraverseRequest{}, fmt.Errorf("edge sections must be a list of strings")
			}
		}
		edges[i] = obsidian.GraphTraverseEdge{
			Source:   edge.Source,
			Sections: edge.Sections,
			Embeds:   edge.Embeds,
		}
	}
	var direction string
	if in.Direction != nil {
		direction = strings.TrimSpace(*in.Direction)
		switch direction {
		case "out", "in", "both":
		default:
			return obsidian.GraphTraverseRequest{}, fmt.Errorf("direction must be one of: out, in, both")
		}
	}
	if in.MaxDepth != nil && *in.MaxDepth < 0 {
		return obsidian.GraphTraverseRequest{}, fmt.Errorf("`max_depth` must be a non-negative integer or null")
	}
	if in.LimitNodes != nil && *in.LimitNodes < 1 {
		return obsidian.GraphTraverseRequest{}, fmt.Errorf("`limit_nodes` must be a positive integer")
	}
	for _, field := range in.Include {
		if strings.TrimSpace(field) == "" {
			return obsidian.GraphTraverseRequest{}, fmt.Errorf("`include` must be a list of field names")
		}
	}
	var start []string
	if in.Start != nil {
		if len(*in.Start) == 0 {
			return obsidian.GraphTraverseRequest{}, fmt.Errorf("`start` must list at least one id or path")
		}
		for _, item := range *in.Start {
			if strings.TrimSpace(item) == "" {
				return obsidian.GraphTraverseRequest{}, fmt.Errorf("`start` must list at least one id or path")
			}
		}
		start = *in.Start
	}
	if err := policy.reject(in); err != nil {
		return obsidian.GraphTraverseRequest{}, err
	}
	return obsidian.GraphTraverseRequest{
		Scope:      in.Scope,
		IDField:    idField,
		Edges:      edges,
		Start:      start,
		Direction:  direction,
		MaxDepth:   in.MaxDepth,
		Include:    in.Include,
		LimitNodes: in.LimitNodes,
	}, nil
}

func (p GraphPolicy) reject(in graphTraverseIn) error {
	if p.MaxLimitNodesInvalid {
		return fmt.Errorf("OBSIDIAN_GRAPH_MAX_LIMIT_NODES must be a positive integer or 0")
	}
	if !scopeAllowed(in.Scope, p.ScopePrefixes) {
		return fmt.Errorf("scope %q is outside OBSIDIAN_GRAPH_SCOPE_ALLOWLIST", in.Scope)
	}
	if p.MaxLimitNodes > 0 {
		effective := graphTraversePluginDefaultLimitNodes
		omitted := in.LimitNodes == nil
		if !omitted {
			effective = *in.LimitNodes
		}
		if effective > p.MaxLimitNodes {
			if omitted {
				return fmt.Errorf("limit_nodes defaults to %d, which exceeds OBSIDIAN_GRAPH_MAX_LIMIT_NODES (%d)", graphTraversePluginDefaultLimitNodes, p.MaxLimitNodes)
			}
			return fmt.Errorf("limit_nodes %d exceeds OBSIDIAN_GRAPH_MAX_LIMIT_NODES (%d)", effective, p.MaxLimitNodes)
		}
	}
	if !p.AllowBody {
		for _, field := range in.Include {
			if field == "$body" {
				return fmt.Errorf("include $body is disabled by OBSIDIAN_GRAPH_ALLOW_BODY")
			}
		}
	}
	if !p.AllowFullExport && in.Start == nil {
		return fmt.Errorf("omitting start exports the whole scope and is disabled by OBSIDIAN_GRAPH_ALLOW_FULL_EXPORT")
	}
	return nil
}

// scopeAllowed reports whether scope is inside the prefix allowlist.
// An empty prefix list allows every scope, including the whole vault.
// A prefix without a trailing slash matches that folder and its descendants.
// A trailing slash matches descendants only. Matching is on a path-segment boundary.
func scopeAllowed(scope string, prefixes []string) bool {
	if len(prefixes) == 0 {
		return true
	}
	scope = strings.TrimSpace(scope)
	scope = strings.TrimPrefix(scope, "/")
	for _, raw := range prefixes {
		prefix := strings.TrimSpace(raw)
		prefix = strings.TrimPrefix(prefix, "/")
		if prefix == "" {
			continue
		}
		if strings.HasSuffix(prefix, "/") {
			if strings.HasPrefix(scope, prefix) {
				return true
			}
			continue
		}
		trimmed := strings.TrimRight(scope, "/")
		if trimmed == prefix || strings.HasPrefix(scope, prefix+"/") {
			return true
		}
	}
	return false
}
