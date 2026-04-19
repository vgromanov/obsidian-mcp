// Package obsidian is an HTTP client for Obsidian Local REST API and MCP-tools extension routes.
package obsidian

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	mimeMarkdown     = "text/markdown"
	mimeNoteJSON     = "application/vnd.olrapi.note+json"
	mimeDataviewDQL  = "application/vnd.olrapi.dataview.dql+txt"
	mimeJSONLogic    = "application/vnd.olrapi.jsonlogic+json"
	mimeJSON         = "application/json"
	defaultUserAgent = "obsidian-mcp/0.1 (+https://github.com/vgromanov/obsidian-mcp)"
)

// Client talks to the Local REST API plugin (and routes registered by obsidian-mcp-tools plugin).
type Client struct {
	BaseURL *url.URL
	APIKey  string
	HTTP    *http.Client
}

// NewClient builds a client for the given host, TLS vs plain HTTP, and API key.
func NewClient(host string, useHTTP bool, apiKey string) (*Client, error) {
	scheme := "https"
	port := "27124"
	if useHTTP {
		scheme = "http"
		port = "27123"
	}
	u := &url.URL{
		Scheme: scheme,
		Host:   netJoinHostPort(host, port),
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if scheme == "https" {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // local self-signed cert, matches upstream NODE_TLS_REJECT_UNAUTHORIZED=0
	}
	return NewClientFromURL(u, apiKey, &http.Client{
		Timeout:   120 * time.Second,
		Transport: tr,
	}), nil
}

// NewClientFromURL builds a client for tests or custom deployments (any base URL).
func NewClientFromURL(u *url.URL, apiKey string, hc *http.Client) *Client {
	if hc == nil {
		hc = &http.Client{Timeout: 120 * time.Second}
	}
	return &Client{BaseURL: u, APIKey: apiKey, HTTP: hc}
}

func netJoinHostPort(host, port string) string {
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		return "[" + host + "]:" + port
	}
	return host + ":" + port
}

// RequestOptions configures a single HTTP call.
type RequestOptions struct {
	Method     string
	Path       string // begins with /
	Body       io.Reader
	BodyString string
	Headers    http.Header
	Expect204  bool
	AcceptJSON bool // decode JSON body regardless of Content-Type
}

// Do executes a request and returns status, response body bytes, and error.
func (c *Client) Do(ctx context.Context, opt RequestOptions) (int, []byte, error) {
	if !strings.HasPrefix(opt.Path, "/") {
		return 0, nil, fmt.Errorf("path must start with /: %q", opt.Path)
	}
	full := strings.TrimRight(c.BaseURL.String(), "/") + opt.Path

	method := opt.Method
	if method == "" {
		method = http.MethodGet
	}
	var body io.Reader = opt.Body
	if body == nil && opt.BodyString != "" {
		body = strings.NewReader(opt.BodyString)
	}

	req, err := http.NewRequestWithContext(ctx, method, full, body)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	if opt.Headers != nil {
		for k, vs := range opt.Headers {
			for _, v := range vs {
				req.Header.Add(k, v)
			}
		}
	}
	if method != http.MethodGet && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", mimeMarkdown)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", defaultUserAgent)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	if opt.Expect204 && resp.StatusCode == http.StatusNoContent {
		return resp.StatusCode, nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, b, fmt.Errorf("%s %s: %s", method, opt.Path, summarizeErrorBody(b))
	}
	return resp.StatusCode, b, nil
}

func summarizeErrorBody(b []byte) string {
	const max = 2048
	s := string(b)
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

// --- Typed helpers used by tools ---

// GetServerInfo GET /
func (c *Client) GetServerInfo(ctx context.Context) (json.RawMessage, error) {
	opt := RequestOptions{Method: http.MethodGet, Path: "/"}
	status, b, err := c.Do(ctx, opt)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("GET /: %d %s", status, string(b))
	}
	return json.RawMessage(b), nil
}

// GetActiveFile GET /active/
func (c *Client) GetActiveFile(ctx context.Context, asJSON bool) ([]byte, error) {
	h := http.Header{}
	if asJSON {
		h.Set("Accept", mimeNoteJSON)
	} else {
		h.Set("Accept", mimeMarkdown)
	}
	opt := RequestOptions{Method: http.MethodGet, Path: "/active/", Headers: h}
	_, b, err := c.Do(ctx, opt)
	return b, err
}

// UpdateActiveFile PUT /active/
func (c *Client) UpdateActiveFile(ctx context.Context, content string) error {
	h := http.Header{}
	h.Set("Content-Type", mimeMarkdown)
	opt := RequestOptions{Method: http.MethodPut, Path: "/active/", BodyString: content, Headers: h, Expect204: true}
	_, _, err := c.Do(ctx, opt)
	return err
}

// AppendActiveFile POST /active/
func (c *Client) AppendActiveFile(ctx context.Context, content string) error {
	h := http.Header{}
	h.Set("Content-Type", mimeMarkdown)
	opt := RequestOptions{Method: http.MethodPost, Path: "/active/", BodyString: content, Headers: h, Expect204: true}
	_, _, err := c.Do(ctx, opt)
	return err
}

// PatchActiveFile PATCH /active/
func (c *Client) PatchActiveFile(ctx context.Context, p PatchParams) (string, error) {
	h := patchHeaders(p)
	opt := RequestOptions{Method: http.MethodPatch, Path: "/active/", BodyString: p.Content, Headers: h}
	_, b, err := c.Do(ctx, opt)
	return string(b), err
}

// DeleteActiveFile DELETE /active/
func (c *Client) DeleteActiveFile(ctx context.Context) error {
	opt := RequestOptions{Method: http.MethodDelete, Path: "/active/", Expect204: true}
	_, _, err := c.Do(ctx, opt)
	return err
}

// ShowFileInObsidian POST /open/{filename}
func (c *Client) ShowFileInObsidian(ctx context.Context, filename string, newLeaf bool) error {
	q := ""
	if newLeaf {
		q = "?newLeaf=true"
	}
	path := "/open/" + url.PathEscape(filename) + q
	opt := RequestOptions{Method: http.MethodPost, Path: path, Expect204: true}
	_, _, err := c.Do(ctx, opt)
	return err
}

// SearchVault POST /search/
func (c *Client) SearchVault(ctx context.Context, queryType, query string) (json.RawMessage, error) {
	h := http.Header{}
	if queryType == "dataview" {
		h.Set("Content-Type", mimeDataviewDQL)
	} else {
		h.Set("Content-Type", mimeJSONLogic)
	}
	opt := RequestOptions{Method: http.MethodPost, Path: "/search/", BodyString: query, Headers: h}
	_, b, err := c.Do(ctx, opt)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

// SearchVaultSimple POST /search/simple/?query=...
func (c *Client) SearchVaultSimple(ctx context.Context, query string, contextLength *int) (json.RawMessage, error) {
	q := url.Values{"query": {query}}
	if contextLength != nil {
		q.Set("contextLength", strconv.Itoa(*contextLength))
	}
	path := "/search/simple/?" + q.Encode()
	opt := RequestOptions{Method: http.MethodPost, Path: path}
	_, b, err := c.Do(ctx, opt)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

// ListVaultFiles GET /vault/ or /vault/{dir}/
func (c *Client) ListVaultFiles(ctx context.Context, directory string) (json.RawMessage, error) {
	path := "/vault/"
	if directory != "" {
		path = "/vault/" + strings.Trim(strings.TrimSpace(directory), "/") + "/"
	}
	opt := RequestOptions{Method: http.MethodGet, Path: path}
	_, b, err := c.Do(ctx, opt)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

// GetVaultFile GET /vault/{filename}
func (c *Client) GetVaultFile(ctx context.Context, filename string, asJSON bool) ([]byte, error) {
	h := http.Header{}
	if asJSON {
		h.Set("Accept", mimeNoteJSON)
	} else {
		h.Set("Accept", mimeMarkdown)
	}
	path := "/vault/" + url.PathEscape(filename)
	opt := RequestOptions{Method: http.MethodGet, Path: path, Headers: h}
	_, b, err := c.Do(ctx, opt)
	return b, err
}

// CreateVaultFile PUT /vault/{filename}
func (c *Client) CreateVaultFile(ctx context.Context, filename, content string) error {
	h := http.Header{}
	h.Set("Content-Type", mimeMarkdown)
	path := "/vault/" + url.PathEscape(filename)
	opt := RequestOptions{Method: http.MethodPut, Path: path, BodyString: content, Headers: h, Expect204: true}
	_, _, err := c.Do(ctx, opt)
	return err
}

// AppendVaultFile POST /vault/{filename}
func (c *Client) AppendVaultFile(ctx context.Context, filename, content string) error {
	h := http.Header{}
	h.Set("Content-Type", mimeMarkdown)
	path := "/vault/" + url.PathEscape(filename)
	opt := RequestOptions{Method: http.MethodPost, Path: path, BodyString: content, Headers: h, Expect204: true}
	_, _, err := c.Do(ctx, opt)
	return err
}

// PatchVaultFile PATCH /vault/{filename}
func (c *Client) PatchVaultFile(ctx context.Context, filename string, p PatchParams) (string, error) {
	h := patchHeaders(p)
	path := "/vault/" + url.PathEscape(filename)
	opt := RequestOptions{Method: http.MethodPatch, Path: path, BodyString: p.Content, Headers: h}
	_, b, err := c.Do(ctx, opt)
	return string(b), err
}

// DeleteVaultFile DELETE /vault/{filename}
func (c *Client) DeleteVaultFile(ctx context.Context, filename string) error {
	path := "/vault/" + url.PathEscape(filename)
	opt := RequestOptions{Method: http.MethodDelete, Path: path, Expect204: true}
	_, _, err := c.Do(ctx, opt)
	return err
}

// SearchVaultSmart POST /search/smart (registered by obsidian-mcp-tools Obsidian plugin).
func (c *Client) SearchVaultSmart(ctx context.Context, body any) (json.RawMessage, error) {
	inner, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	// obsidian-mcp-tools validates the body with jsonSearchRequest:
	// type("string.json.parse").to(searchRequest) — Express must receive a JSON string
	// that parses to { "query", "filter?" }, not a raw JSON object.
	outer, err := json.Marshal(string(inner))
	if err != nil {
		return nil, err
	}
	h := http.Header{}
	h.Set("Content-Type", mimeJSON)
	opt := RequestOptions{Method: http.MethodPost, Path: "/search/smart", BodyString: string(outer), Headers: h}
	_, b, err := c.Do(ctx, opt)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

// ExecuteTemplate POST /templates/execute
func (c *Client) ExecuteTemplate(ctx context.Context, params TemplateExecutionRequest) (json.RawMessage, error) {
	raw, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	h := http.Header{}
	h.Set("Content-Type", mimeJSON)
	opt := RequestOptions{Method: http.MethodPost, Path: "/templates/execute", BodyString: string(raw), Headers: h}
	_, b, err := c.Do(ctx, opt)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

// NormalizeTagName trims whitespace and a leading # from a tag argument.
func NormalizeTagName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "#")
	return strings.TrimSpace(s)
}

// ListTags GET /tags/
func (c *Client) ListTags(ctx context.Context) (json.RawMessage, error) {
	opt := RequestOptions{Method: http.MethodGet, Path: "/tags/"}
	_, b, err := c.Do(ctx, opt)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

// GetTagFiles returns vault files containing the given tag.
// Implemented on top of POST /search/ (JsonLogic) since upstream Local REST API
// does not expose a /tags/{tagname}/ route. The tag is normalized (leading `#` stripped).
func (c *Client) GetTagFiles(ctx context.Context, tagname string) (json.RawMessage, error) {
	t := NormalizeTagName(tagname)
	if t == "" {
		return nil, fmt.Errorf("tag name is required")
	}
	tagJSON, err := json.Marshal(t)
	if err != nil {
		return nil, err
	}
	query := `{"in":[` + string(tagJSON) + `,{"var":"tags"}]}`
	return c.SearchVault(ctx, "jsonlogic", query)
}

// ListCommands GET /commands/
func (c *Client) ListCommands(ctx context.Context) (json.RawMessage, error) {
	opt := RequestOptions{Method: http.MethodGet, Path: "/commands/"}
	_, b, err := c.Do(ctx, opt)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

// ExecuteCommand POST /commands/{commandId}/
func (c *Client) ExecuteCommand(ctx context.Context, commandID string) error {
	commandID = strings.TrimSpace(commandID)
	if commandID == "" {
		return fmt.Errorf("commandId is required")
	}
	path := "/commands/" + url.PathEscape(commandID) + "/"
	h := http.Header{}
	h.Set("Content-Type", mimeJSON)
	// Avoid defaulting POST bodies to text/markdown in Do(); command endpoint ignores body.
	opt := RequestOptions{Method: http.MethodPost, Path: path, Headers: h, BodyString: "{}", Expect204: true}
	_, _, err := c.Do(ctx, opt)
	return err
}

func periodicBasePath(period PeriodicPeriod) string {
	return "/periodic/" + string(period) + "/"
}

// GetPeriodicNote GET /periodic/{period}/
func (c *Client) GetPeriodicNote(ctx context.Context, period PeriodicPeriod, asJSON bool) ([]byte, error) {
	h := http.Header{}
	if asJSON {
		h.Set("Accept", mimeNoteJSON)
	} else {
		h.Set("Accept", mimeMarkdown)
	}
	opt := RequestOptions{Method: http.MethodGet, Path: periodicBasePath(period), Headers: h}
	_, b, err := c.Do(ctx, opt)
	return b, err
}

// UpdatePeriodicNote PUT /periodic/{period}/
func (c *Client) UpdatePeriodicNote(ctx context.Context, period PeriodicPeriod, content string) error {
	h := http.Header{}
	h.Set("Content-Type", mimeMarkdown)
	opt := RequestOptions{Method: http.MethodPut, Path: periodicBasePath(period), BodyString: content, Headers: h, Expect204: true}
	_, _, err := c.Do(ctx, opt)
	return err
}

// AppendPeriodicNote POST /periodic/{period}/
func (c *Client) AppendPeriodicNote(ctx context.Context, period PeriodicPeriod, content string) error {
	h := http.Header{}
	h.Set("Content-Type", mimeMarkdown)
	opt := RequestOptions{Method: http.MethodPost, Path: periodicBasePath(period), BodyString: content, Headers: h, Expect204: true}
	_, _, err := c.Do(ctx, opt)
	return err
}

// PatchPeriodicNote PATCH /periodic/{period}/
func (c *Client) PatchPeriodicNote(ctx context.Context, period PeriodicPeriod, p PatchParams) (string, error) {
	h := patchHeaders(p)
	opt := RequestOptions{Method: http.MethodPatch, Path: periodicBasePath(period), BodyString: p.Content, Headers: h}
	_, b, err := c.Do(ctx, opt)
	return string(b), err
}

// DeletePeriodicNote DELETE /periodic/{period}/
func (c *Client) DeletePeriodicNote(ctx context.Context, period PeriodicPeriod) error {
	opt := RequestOptions{Method: http.MethodDelete, Path: periodicBasePath(period), Expect204: true}
	_, _, err := c.Do(ctx, opt)
	return err
}

func patchHeaders(p PatchParams) http.Header {
	h := http.Header{}
	h.Set("Operation", p.Operation)
	h.Set("Target-Type", p.TargetType)
	h.Set("Target", p.Target)
	h.Set("Create-Target-If-Missing", "true")
	if p.TargetDelimiter != nil {
		h.Set("Target-Delimiter", *p.TargetDelimiter)
	}
	if p.TrimTargetWhitespace != nil {
		h.Set("Trim-Target-Whitespace", strconv.FormatBool(*p.TrimTargetWhitespace))
	}
	if p.ContentType != nil {
		h.Set("Content-Type", *p.ContentType)
	}
	return h
}
