package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/tool"
)

func NewWebFetch() tool.Executor {
	return tool.ExecutorFunc(func(args json.RawMessage) (string, error) {
		var params struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return "", fmt.Errorf("web_fetch: invalid args: %w", err)
		}
		if params.URL == "" {
			return "", fmt.Errorf("web_fetch: url is required")
		}
		if isPrivateURL(params.URL) {
			return "", fmt.Errorf("web_fetch: access to private/internal URLs is not allowed")
		}
		client := &http.Client{Timeout: 20 * time.Second}
		req, err := http.NewRequest("GET", params.URL, nil)
		if err != nil {
			return "", fmt.Errorf("web_fetch: %w", err)
		}
		req.Header.Set("User-Agent", "local-ai-companion/1.0")
		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("web_fetch: fetch failed: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return "", fmt.Errorf("web_fetch: HTTP %d", resp.StatusCode)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 100*1024))
		if err != nil {
			return "", fmt.Errorf("web_fetch: read error: %w", err)
		}
		contentType := resp.Header.Get("Content-Type")
		if strings.Contains(contentType, "text/html") || strings.Contains(contentType, "text/plain") {
			return string(body), nil
		}
		return "", fmt.Errorf("web_fetch: unsupported content type: %s", contentType)
	})
}

func isPrivateURL(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	privatePatterns := []string{
		"localhost", "127.0.0.", "10.", "172.16.", "172.17.", "172.18.",
		"172.19.", "172.20.", "172.21.", "172.22.", "172.23.", "172.24.",
		"172.25.", "172.26.", "172.27.", "172.28.", "172.29.", "172.30.",
		"172.31.", "192.168.", "0.0.0.0", "[::1]",
	}
	for _, p := range privatePatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}
