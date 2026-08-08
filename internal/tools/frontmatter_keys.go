package tools

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterFrontmatterKeys registers Local Smart Lookup frontmatter/Properties tools.
func RegisterFrontmatterKeys(s *mcp.Server, d Deps) {
	cli := d.Client

	mcp.AddTool(s, &mcp.Tool{
		Name: "list_frontmatter_keys",
		Description: "List vault Obsidian Properties (YAML frontmatter keys) with note counts and types. " +
			"Calls Local Smart Lookup GET /frontmatter_keys/ (not /si/*). Use before inventing a new property key; prefer reusing an existing high-count name.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		raw, err := cli.ListFrontmatterKeys(ctx)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(raw), nil, nil
	})

	type keyFilesIn struct {
		Name string `json:"name"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name: "get_frontmatter_key_files",
		Description: "List vault note paths that have the given frontmatter/Properties key set. " +
			"Calls Local Smart Lookup GET /frontmatter_keys/{name}/. Unknown keys return an empty list.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in keyFilesIn) (*mcp.CallToolResult, any, error) {
		raw, err := cli.GetFrontmatterKeyFiles(ctx, strings.TrimSpace(in.Name))
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(raw), nil, nil
	})
}
