package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterCommands registers Local REST API command tools.
func RegisterCommands(s *mcp.Server, d Deps) {
	cli := d.Client

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_commands",
		Description: "List Obsidian commands available to the Local REST API (id + display name).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, *void, error) {
		raw, err := cli.ListCommands(ctx)
		if err != nil {
			return nil, nil, err
		}
		return textResult(prettyJSONBytes(raw)), nil, nil
	})

	type execCommandIn struct {
		CommandID string `json:"commandId"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "execute_command",
		Description: "Execute an Obsidian command by id (POST /commands/{commandId}/). This runs inside the Obsidian UI and may open panes, modify editor state, or trigger plugin actions — use deliberately.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in execCommandIn) (*mcp.CallToolResult, *void, error) {
		if err := cli.ExecuteCommand(ctx, in.CommandID); err != nil {
			return nil, nil, err
		}
		return textResult("Command executed successfully"), nil, nil
	})
}
