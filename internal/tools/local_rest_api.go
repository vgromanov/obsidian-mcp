package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vgromanov/obsidian-mcp/internal/obsidian"
)

// RegisterLocalREST registers Local REST API bridge tools (parity with upstream local-rest-api).
func RegisterLocalREST(s *mcp.Server, d Deps) {
	cli := d.Client

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_server_info",
		Description: "Returns basic details about the Obsidian Local REST API and authentication status. This is the only API request that does not require authentication.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, *void, error) {
		raw, err := cli.GetServerInfo(ctx)
		if err != nil {
			return nil, nil, err
		}
		return textResult(prettyJSONBytes(raw)), nil, nil
	})

	type getActiveIn struct {
		Format *string `json:"format,omitempty"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_active_file",
		Description: "Returns the content of the currently active file in Obsidian. Can return either markdown content or a JSON representation including parsed tags and frontmatter.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getActiveIn) (*mcp.CallToolResult, *void, error) {
		asJSON := in.Format != nil && *in.Format == "json"
		b, err := cli.GetActiveFile(ctx, asJSON)
		if err != nil {
			return nil, nil, err
		}
		if asJSON {
			return textResult(prettyJSONBytes(b)), nil, nil
		}
		return textResult(string(b)), nil, nil
	})

	type contentIn struct {
		Content string `json:"content"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_active_file",
		Description: "Update the content of the active file open in Obsidian.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in contentIn) (*mcp.CallToolResult, *void, error) {
		if err := cli.UpdateActiveFile(ctx, in.Content); err != nil {
			return nil, nil, err
		}
		return textResult("File updated successfully"), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "append_to_active_file",
		Description: "Append content to the end of the currently-open note.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in contentIn) (*mcp.CallToolResult, *void, error) {
		if err := cli.AppendActiveFile(ctx, in.Content); err != nil {
			return nil, nil, err
		}
		return textResult("Content appended successfully"), nil, nil
	})

	type patchIn struct {
		obsidian.PatchParams
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "patch_active_file",
		Description: "Insert or modify content in the currently-open note relative to a heading, block reference, or frontmatter field.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in patchIn) (*mcp.CallToolResult, *void, error) {
		body, err := cli.PatchActiveFile(ctx, in.PatchParams)
		if err != nil {
			return nil, nil, err
		}
		return textResult2("File patched successfully", body), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_active_file",
		Description: "Delete the currently-active file in Obsidian.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, *void, error) {
		if err := cli.DeleteActiveFile(ctx); err != nil {
			return nil, nil, err
		}
		return textResult("File deleted successfully"), nil, nil
	})

	type openIn struct {
		Filename string `json:"filename"`
		NewLeaf  *bool  `json:"newLeaf,omitempty"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "show_file_in_obsidian",
		Description: "Open a document in the Obsidian UI. Creates a new document if it doesn't exist. Returns a confirmation if the file was opened successfully.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in openIn) (*mcp.CallToolResult, *void, error) {
		nl := in.NewLeaf != nil && *in.NewLeaf
		if err := cli.ShowFileInObsidian(ctx, in.Filename, nl); err != nil {
			return nil, nil, err
		}
		return textResult("File opened successfully"), nil, nil
	})

	type searchVaultIn struct {
		QueryType string `json:"queryType"`
		Query     string `json:"query"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "search_vault",
		Description: "Search for documents matching a specified query using either Dataview DQL or JsonLogic.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in searchVaultIn) (*mcp.CallToolResult, *void, error) {
		raw, err := cli.SearchVault(ctx, in.QueryType, in.Query)
		if err != nil {
			return nil, nil, err
		}
		return textResult(prettyJSONBytes(raw)), nil, nil
	})

	type searchSimpleIn struct {
		Query         string `json:"query"`
		ContextLength *int   `json:"contextLength,omitempty"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "search_vault_simple",
		Description: "Search for documents matching a text query.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in searchSimpleIn) (*mcp.CallToolResult, *void, error) {
		raw, err := cli.SearchVaultSimple(ctx, in.Query, in.ContextLength)
		if err != nil {
			return nil, nil, err
		}
		return textResult(prettyJSONBytes(raw)), nil, nil
	})

	type listIn struct {
		Directory *string `json:"directory,omitempty"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_vault_files",
		Description: "List files in the root directory or a specified subdirectory of your vault.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listIn) (*mcp.CallToolResult, *void, error) {
		dir := ""
		if in.Directory != nil {
			dir = *in.Directory
		}
		raw, err := cli.ListVaultFiles(ctx, dir)
		if err != nil {
			return nil, nil, err
		}
		return textResult(prettyJSONBytes(raw)), nil, nil
	})

	type getVaultIn struct {
		Filename string  `json:"filename"`
		Format   *string `json:"format,omitempty"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_vault_file",
		Description: "Get the content of a file from your vault.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getVaultIn) (*mcp.CallToolResult, *void, error) {
		asJSON := in.Format != nil && *in.Format == "json"
		b, err := cli.GetVaultFile(ctx, in.Filename, asJSON)
		if err != nil {
			return nil, nil, err
		}
		if asJSON {
			return textResult(prettyJSONBytes(b)), nil, nil
		}
		return textResult(string(b)), nil, nil
	})

	type putVaultIn struct {
		Filename string `json:"filename"`
		Content  string `json:"content"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_vault_file",
		Description: "Create a new file in your vault or update an existing one.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in putVaultIn) (*mcp.CallToolResult, *void, error) {
		if err := cli.CreateVaultFile(ctx, in.Filename, in.Content); err != nil {
			return nil, nil, err
		}
		return textResult("File created successfully"), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "append_to_vault_file",
		Description: "Append content to a new or existing file.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in putVaultIn) (*mcp.CallToolResult, *void, error) {
		if err := cli.AppendVaultFile(ctx, in.Filename, in.Content); err != nil {
			return nil, nil, err
		}
		return textResult("Content appended successfully"), nil, nil
	})

	type patchVaultIn struct {
		Filename string `json:"filename"`
		obsidian.PatchParams
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "patch_vault_file",
		Description: "Insert or modify content in a file relative to a heading, block reference, or frontmatter field.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in patchVaultIn) (*mcp.CallToolResult, *void, error) {
		body, err := cli.PatchVaultFile(ctx, in.Filename, in.PatchParams)
		if err != nil {
			return nil, nil, err
		}
		return textResult2("File patched successfully", body), nil, nil
	})

	type delVaultIn struct {
		Filename string `json:"filename"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_vault_file",
		Description: "Delete a file from your vault.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in delVaultIn) (*mcp.CallToolResult, *void, error) {
		if err := cli.DeleteVaultFile(ctx, in.Filename); err != nil {
			return nil, nil, err
		}
		return textResult("File deleted successfully"), nil, nil
	})
}
