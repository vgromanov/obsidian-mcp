package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vgromanov/obsidian-mcp/internal/obsidian"
)

// RegisterPeriodic registers Local REST API periodic note tools (current period only).
func RegisterPeriodic(s *mcp.Server, d Deps) {
	cli := d.Client

	type periodFmtIn struct {
		Period string  `json:"period"`
		Format *string `json:"format,omitempty"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_periodic_note",
		Description: "Get the current periodic note for daily, weekly, monthly, quarterly, or yearly period (GET /periodic/{period}/). Requires the Periodic Notes community plugin configured in Obsidian. Markdown or note+json.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in periodFmtIn) (*mcp.CallToolResult, *void, error) {
		p, err := obsidian.ParsePeriodicPeriod(in.Period)
		if err != nil {
			return nil, nil, err
		}
		asJSON := in.Format != nil && *in.Format == "json"
		b, err := cli.GetPeriodicNote(ctx, p, asJSON)
		if err != nil {
			return nil, nil, err
		}
		if asJSON {
			return textResult(prettyJSONBytes(b)), nil, nil
		}
		return textResult(string(b)), nil, nil
	})

	type periodContentIn struct {
		Period  string `json:"period"`
		Content string `json:"content"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_periodic_note",
		Description: "Replace the entire content of the current periodic note for the given period (PUT /periodic/{period}/). Creates the note if needed per Local REST API behavior.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in periodContentIn) (*mcp.CallToolResult, *void, error) {
		p, err := obsidian.ParsePeriodicPeriod(in.Period)
		if err != nil {
			return nil, nil, err
		}
		if err := cli.UpdatePeriodicNote(ctx, p, in.Content); err != nil {
			return nil, nil, err
		}
		return textResult("Periodic note updated successfully"), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "append_to_periodic_note",
		Description: "Append markdown to the end of the current periodic note for the given period (POST /periodic/{period}/).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in periodContentIn) (*mcp.CallToolResult, *void, error) {
		p, err := obsidian.ParsePeriodicPeriod(in.Period)
		if err != nil {
			return nil, nil, err
		}
		if err := cli.AppendPeriodicNote(ctx, p, in.Content); err != nil {
			return nil, nil, err
		}
		return textResult("Content appended to periodic note successfully"), nil, nil
	})

	type patchPeriodicIn struct {
		Period string `json:"period"`
		obsidian.PatchParams
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "patch_periodic_note",
		Description: "Patch the current periodic note relative to a heading, block reference, or frontmatter field (PATCH /periodic/{period}/). Same Operation / Target-Type / Target semantics as patch_vault_file.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in patchPeriodicIn) (*mcp.CallToolResult, *void, error) {
		p, err := obsidian.ParsePeriodicPeriod(in.Period)
		if err != nil {
			return nil, nil, err
		}
		body, err := cli.PatchPeriodicNote(ctx, p, in.PatchParams)
		if err != nil {
			return nil, nil, err
		}
		return textResult2("Periodic note patched successfully", body), nil, nil
	})

	type periodOnlyIn struct {
		Period string `json:"period"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_periodic_note",
		Description: "Delete the current periodic note file for the given period (DELETE /periodic/{period}/).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in periodOnlyIn) (*mcp.CallToolResult, *void, error) {
		p, err := obsidian.ParsePeriodicPeriod(in.Period)
		if err != nil {
			return nil, nil, err
		}
		if err := cli.DeletePeriodicNote(ctx, p); err != nil {
			return nil, nil, err
		}
		return textResult("Periodic note deleted successfully"), nil, nil
	})
}
