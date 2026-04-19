package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterTags registers Local REST API tag tools.
func RegisterTags(s *mcp.Server, d Deps) {
	cli := d.Client

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_tags",
		Description: "List all tags in the vault with usage counts (inline #tags and frontmatter). Response uses `name` and `count` fields per tag.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, *void, error) {
		raw, err := cli.ListTags(ctx)
		if err != nil {
			return nil, nil, err
		}
		return textResult(prettyJSONBytes(raw)), nil, nil
	})

	type tagFilesIn struct {
		Tag string `json:"tag"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_tag_files",
		Description: "List vault files containing the given tag. Implemented as a JsonLogic search (POST /search/ with `{\"in\":[<tag>,{\"var\":\"tags\"}]}`) because upstream Local REST API has no per-tag route. Tag may include or omit a leading `#`.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in tagFilesIn) (*mcp.CallToolResult, *void, error) {
		raw, err := cli.GetTagFiles(ctx, in.Tag)
		if err != nil {
			return nil, nil, err
		}
		return textResult(prettyJSONBytes(raw)), nil, nil
	})
}
