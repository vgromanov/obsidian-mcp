package omlx

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const checkTimeout = 5 * time.Second

// Check verifies the oMLX OpenAI-compatible API is reachable (GET /models).
func Check(ctx context.Context, baseURL, apiKey string) error {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return fmt.Errorf("OMLX_BASE_URL is empty")
	}
	url := base + "/models"

	reqCtx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("omlx check request: %w", err)
	}
	if key := strings.TrimSpace(apiKey); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("cannot reach oMLX at %s — is it running on port 8000? (%w)", base, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("oMLX at %s returned HTTP %d", base, resp.StatusCode)
	}
	return nil
}
