package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// retrievalLogMu serializes appends within this process. Cross-process safety
// relies on the single-writer-per-host-file rule: each machine writes only its
// own <host>.jsonl shard, so concurrent writers to one file never happen and a
// file-sync backend (OneDrive via the remotely-save plugin) never has to merge.
var retrievalLogMu sync.Mutex

type retrievalReturned struct {
	Path        string   `json:"path"`
	Rank        int      `json:"rank"`
	Score       *float64 `json:"score,omitempty"`
	RerankScore *float64 `json:"rerankScore,omitempty"`
}

type retrievalEvent struct {
	TS       string              `json:"ts"`
	Host     string              `json:"host"`
	Regime   string              `json:"regime,omitempty"`
	Query    string              `json:"query"`
	Params   map[string]any      `json:"params,omitempty"`
	Returned []retrievalReturned `json:"returned"`
}

// shortHost returns a filesystem-safe short hostname used to shard the per-host
// log file. Mirrors the Python side's _short_host so both writers agree on the
// shard name for a given machine.
func shortHost() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "unknown-host"
	}
	if i := strings.IndexByte(h, '.'); i >= 0 {
		h = h[:i]
	}
	var b strings.Builder
	for _, c := range h {
		if c == '-' || c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			b.WriteRune(c)
		} else {
			b.WriteRune('-')
		}
	}
	if b.Len() == 0 {
		return "unknown-host"
	}
	return b.String()
}

// logRetrieval best-effort appends one JSONL retrieval event to
// <dir>/<host>.jsonl. It never returns an error and never panics: logging must
// never break search. A nil/empty dir disables logging.
func logRetrieval(dir, regime, query string, body map[string]any, raw json.RawMessage) {
	if strings.TrimSpace(dir) == "" {
		return
	}

	// Extract just path/score/rerankScore from the plugin's results.
	var parsed struct {
		Results []struct {
			Path        string   `json:"path"`
			Score       *float64 `json:"score"`
			RerankScore *float64 `json:"rerankScore"`
		} `json:"results"`
	}
	_ = json.Unmarshal(raw, &parsed)
	returned := make([]retrievalReturned, 0, len(parsed.Results))
	for i, r := range parsed.Results {
		returned = append(returned, retrievalReturned{
			Path: r.Path, Rank: i, Score: r.Score, RerankScore: r.RerankScore,
		})
	}

	// params = the request body minus the query (query is logged top-level).
	params := make(map[string]any, len(body))
	for k, v := range body {
		if k == "query" {
			continue
		}
		params[k] = v
	}
	if len(params) == 0 {
		params = nil
	}

	ev := retrievalEvent{
		TS:       time.Now().UTC().Format(time.RFC3339),
		Host:     shortHost(),
		Regime:   regime,
		Query:    query,
		Params:   params,
		Returned: returned,
	}
	line, err := json.Marshal(ev)
	if err != nil {
		return
	}

	retrievalLogMu.Lock()
	defer retrievalLogMu.Unlock()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, ev.Host+".jsonl"),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = f.Write(append(line, '\n'))
}
