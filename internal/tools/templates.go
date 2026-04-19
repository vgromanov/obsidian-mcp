package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vgromanov/obsidian-mcp/internal/obsidian"
	"github.com/vgromanov/obsidian-mcp/internal/templater"
)

// RegisterTemplater registers execute_template (parity with upstream templates feature).
func RegisterTemplater(s *mcp.Server, d Deps) {
	type execIn struct {
		Name       string            `json:"name"`
		Arguments  map[string]string `json:"arguments,omitempty"`
		CreateFile *string           `json:"createFile,omitempty"`
		TargetPath *string           `json:"targetPath,omitempty"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "execute_template",
		Description: "Execute a Templater template with the given arguments",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in execIn) (*mcp.CallToolResult, *void, error) {
		b, err := d.Client.GetVaultFile(ctx, in.Name, true)
		if err != nil {
			return nil, nil, err
		}
		var note obsidian.VaultFileJSON
		if err := json.Unmarshal(b, &note); err != nil {
			return nil, nil, fmt.Errorf("parse template note json: %w", err)
		}
		params := templater.ParseParameters(note.Content)
		args, err := mergeTemplateArgs(params, in.Arguments)
		if err != nil {
			return nil, nil, err
		}
		create := in.CreateFile != nil && strings.EqualFold(*in.CreateFile, "true")
		req := obsidian.TemplateExecutionRequest{
			Name:       in.Name,
			Arguments:  args,
			CreateFile: create,
		}
		if in.TargetPath != nil {
			req.TargetPath = *in.TargetPath
		}
		raw, err := d.Client.ExecuteTemplate(ctx, req)
		if err != nil {
			return nil, nil, err
		}
		return textResult(prettyJSONBytes(raw)), nil, nil
	})
}

func mergeTemplateArgs(declared []templater.Parameter, got map[string]string) (map[string]string, error) {
	if got == nil {
		got = map[string]string{}
	}
	allowed := make(map[string]struct{})
	for _, p := range declared {
		allowed[p.Name] = struct{}{}
	}
	for k := range got {
		if _, ok := allowed[k]; !ok {
			return nil, fmt.Errorf("unknown template argument %q", k)
		}
	}
	out := make(map[string]string)
	for _, p := range declared {
		if v, ok := got[p.Name]; ok {
			out[p.Name] = v
		} else {
			out[p.Name] = ""
		}
	}
	return out, nil
}
