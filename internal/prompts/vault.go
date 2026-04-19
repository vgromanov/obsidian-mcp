package prompts

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vgromanov/obsidian-mcp/internal/obsidian"
	"github.com/vgromanov/obsidian-mcp/internal/templater"
)

const promptTag = "mcp-tools-prompt"

// Deps for vault-backed prompts.
type Deps struct {
	Client     *obsidian.Client
	PromptsDir string
}

func vaultListPath(dir string) string {
	d := strings.Trim(strings.TrimSpace(dir), "/")
	if d == "" {
		return "/vault/"
	}
	return "/vault/" + d + "/"
}

// ListFromVault implements prompts/list (dynamic), mirroring upstream setupObsidianPrompts.
func ListFromVault(ctx context.Context, d Deps) (*mcp.ListPromptsResult, error) {
	raw, err := d.Client.ListVaultFiles(ctx, d.PromptsDir)
	if err != nil {
		return nil, err
	}
	var dir obsidian.VaultDirectoryList
	if err := json.Unmarshal(raw, &dir); err != nil {
		return nil, fmt.Errorf("list prompts dir: %w", err)
	}
	var prompts []*mcp.Prompt
	for _, filename := range dir.Files {
		if !strings.HasSuffix(strings.ToLower(filename), ".md") {
			continue
		}
		fullPath := path.Join(d.PromptsDir, filename)
		b, err := d.Client.GetVaultFile(ctx, fullPath, true)
		if err != nil {
			continue
		}
		var note obsidian.VaultFileJSON
		if err := json.Unmarshal(b, &note); err != nil {
			continue
		}
		if !slices.Contains(note.Tags, promptTag) {
			continue
		}
		params := templater.ParseParameters(note.Content)
		args := make([]*mcp.PromptArgument, 0, len(params))
		for _, p := range params {
			args = append(args, &mcp.PromptArgument{
				Name:        p.Name,
				Description: p.Description,
				Required:    false,
			})
		}
		desc := ""
		if note.Frontmatter.Description != nil {
			desc = *note.Frontmatter.Description
		}
		prompts = append(prompts, &mcp.Prompt{
			Name:        filename,
			Description: desc,
			Arguments:   args,
		})
	}
	return &mcp.ListPromptsResult{Prompts: prompts}, nil
}

// GetFromVault implements prompts/get for vault-backed Templater prompts.
func GetFromVault(ctx context.Context, d Deps, params *mcp.GetPromptParams) (*mcp.GetPromptResult, error) {
	if params == nil {
		return nil, fmt.Errorf("missing params")
	}
	promptPath := path.Join(d.PromptsDir, params.Name)
	b, err := d.Client.GetVaultFile(ctx, promptPath, true)
	if err != nil {
		return nil, err
	}
	var note obsidian.VaultFileJSON
	if err := json.Unmarshal(b, &note); err != nil {
		return nil, fmt.Errorf("read prompt: %w", err)
	}
	hasTag := slices.Contains(note.Tags, promptTag) || slices.Contains(note.Frontmatter.Tags, promptTag)
	if !hasTag {
		return nil, fmt.Errorf("file %q must include tag %s in note tags or YAML frontmatter tags", params.Name, promptTag)
	}
	desc := ""
	if note.Frontmatter.Description != nil {
		desc = *note.Frontmatter.Description
	}
	template := note.Content
	tparams := templater.ParseParameters(template)
	args, err := mergePromptArgs(tparams, params.Arguments)
	if err != nil {
		return nil, err
	}
	raw, err := d.Client.ExecuteTemplate(ctx, obsidian.TemplateExecutionRequest{
		Name:      promptPath,
		Arguments: args,
	})
	if err != nil {
		return nil, err
	}
	var exec obsidian.TemplateExecutionResponse
	if err := json.Unmarshal(raw, &exec); err != nil {
		return nil, fmt.Errorf("template execute response: %w", err)
	}
	body := stripLeadingFrontmatter(exec.Content)
	return &mcp.GetPromptResult{
		Description: desc,
		Messages: []*mcp.PromptMessage{{
			Role:    mcp.Role("user"),
			Content: &mcp.TextContent{Text: strings.TrimSpace(body)},
		}},
	}, nil
}

func mergePromptArgs(declared []templater.Parameter, got map[string]string) (map[string]string, error) {
	if got == nil {
		got = map[string]string{}
	}
	allowed := make(map[string]struct{})
	for _, p := range declared {
		allowed[p.Name] = struct{}{}
	}
	for k := range got {
		if _, ok := allowed[k]; !ok {
			return nil, fmt.Errorf("invalid argument %q", k)
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

func stripLeadingFrontmatter(content string) string {
	parts := strings.Split(content, "---")
	if len(parts) < 2 {
		return content
	}
	return parts[len(parts)-1]
}
