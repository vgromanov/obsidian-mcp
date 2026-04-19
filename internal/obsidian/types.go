package obsidian

import (
	"fmt"
	"strings"
)

// PatchParams mirrors Local REST API PATCH headers + body (see upstream ApiPatchParameters).
type PatchParams struct {
	Operation            string  `json:"operation"`
	TargetType           string  `json:"targetType"`
	Target               string  `json:"target"`
	TargetDelimiter      *string `json:"targetDelimiter,omitempty"`
	TrimTargetWhitespace *bool   `json:"trimTargetWhitespace,omitempty"`
	Content              string  `json:"content"`
	ContentType          *string `json:"contentType,omitempty"`
}

// NoteJSON is application/vnd.olrapi.note+json for vault/active file reads.
type NoteJSON struct {
	Content     string         `json:"content"`
	Frontmatter map[string]any `json:"frontmatter"`
	Path        string         `json:"path"`
	Stat        NoteStat       `json:"stat"`
	Tags        []string       `json:"tags"`
}

// NoteStat is filesystem metadata on a note JSON envelope.
type NoteStat struct {
	Ctime int64 `json:"ctime"`
	Mtime int64 `json:"mtime"`
	Size  int64 `json:"size"`
}

// VaultFileJSON is the olrapi note+json shape returned for vault file reads (includes nested frontmatter.tags in plugin schema).
type VaultFileJSON struct {
	Frontmatter VaultFrontmatter `json:"frontmatter"`
	Content     string           `json:"content"`
	Path        string           `json:"path"`
	Stat        NoteStat         `json:"stat"`
	Tags        []string         `json:"tags"`
}

// VaultFrontmatter subset used by MCP prompts.
type VaultFrontmatter struct {
	Tags        []string `json:"tags"`
	Description *string  `json:"description,omitempty"`
}

// VaultDirectoryList is GET /vault/{dir}/
type VaultDirectoryList struct {
	Files []string `json:"files"`
}

// TemplateExecutionRequest POST /templates/execute
type TemplateExecutionRequest struct {
	Name       string            `json:"name"`
	Arguments  map[string]string `json:"arguments"`
	CreateFile bool              `json:"createFile,omitempty"`
	TargetPath string            `json:"targetPath,omitempty"`
}

// TemplateExecutionResponse from /templates/execute
type TemplateExecutionResponse struct {
	Message string `json:"message"`
	Content string `json:"content"`
}

// TagEntry is one element of GET /tags/ (Local REST API returns name + count).
type TagEntry struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// TagListResponse is the JSON body of GET /tags/.
type TagListResponse struct {
	Tags []TagEntry `json:"tags"`
}

// Command is one entry in GET /commands/.
type Command struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CommandListResponse is the JSON body of GET /commands/.
type CommandListResponse struct {
	Commands []Command `json:"commands"`
}

// PeriodicPeriod is the path segment for /periodic/{period}/ (current note only).
type PeriodicPeriod string

const (
	PeriodDaily     PeriodicPeriod = "daily"
	PeriodWeekly    PeriodicPeriod = "weekly"
	PeriodMonthly   PeriodicPeriod = "monthly"
	PeriodQuarterly PeriodicPeriod = "quarterly"
	PeriodYearly    PeriodicPeriod = "yearly"
)

// ParsePeriodicPeriod validates and normalizes a period string for /periodic/{period}/.
func ParsePeriodicPeriod(raw string) (PeriodicPeriod, error) {
	p := PeriodicPeriod(strings.ToLower(strings.TrimSpace(raw)))
	switch p {
	case PeriodDaily, PeriodWeekly, PeriodMonthly, PeriodQuarterly, PeriodYearly:
		return p, nil
	default:
		return "", fmt.Errorf("invalid period %q: want daily, weekly, monthly, quarterly, or yearly", raw)
	}
}
