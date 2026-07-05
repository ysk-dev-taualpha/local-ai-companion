package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// WebFetchParams is the JSON schema for the web_fetch tool.
type WebFetchParams struct {
	URL string `json:"url"`
}

// WebFetch is a tool that fetches content from a URL.
type WebFetch struct {
	httpClient *http.Client
}

// NewWebFetch creates a new WebFetch tool.
func NewWebFetch(timeout time.Duration) *WebFetch {
	return &WebFetch{
		httpClient: &http.Client{Timeout: timeout},
	}
}

// Name returns the tool name.
func (w *WebFetch) Name() string {
	return "web_fetch"
}

// Description returns the tool description.
func (w *WebFetch) Description() string {
	return "Fetch content from a URL. Returns the text content (up to 10000 characters). Use this to read web pages."
}

// Parameters returns the JSON Schema for the tool's parameters.
func (w *WebFetch) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {
				"type": "string",
				"description": "The URL to fetch content from"
			}
		},
		"required": ["url"]
	}`)
}

// Execute fetches content from a URL.
func (w *WebFetch) Execute(ctx context.Context, arguments json.RawMessage) (string, error) {
	var params WebFetchParams
	if err := json.Unmarshal(arguments, &params); err != nil {
		return "", fmt.Errorf("web_fetch: invalid arguments: %w", err)
	}

	if params.URL == "" {
		return "", fmt.Errorf("web_fetch: url is required")
	}

	// Validate URL scheme
	lowerURL := strings.ToLower(params.URL)
	if !strings.HasPrefix(lowerURL, "http://") && !strings.HasPrefix(lowerURL, "https://") {
		return "", fmt.Errorf("web_fetch: only http and https URLs are supported")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", params.URL, nil)
	if err != nil {
		return "", fmt.Errorf("web_fetch: request creation failed: %w", err)
	}
	req.Header.Set("User-Agent", "local-ai-companion/0.1")
	req.Header.Set("Accept", "text/plain, text/html;q=0.9")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("web_fetch: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("web_fetch: server returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return "", fmt.Errorf("web_fetch: failed to read response: %w", err)
	}

	content := extractText(string(body))
	if len(content) > 10000 {
		content = content[:10000] + "..."
	}

	out, _ := json.Marshal(map[string]interface{}{
		"url":            params.URL,
		"content":        content,
		"content_length": len(content),
		"status_code":    resp.StatusCode,
	})
	return string(out), nil
}

// extractText strips HTML tags and extracts readable text.
func extractText(html string) string {
	text := html

	// Remove scripts and styles
	for _, tag := range []string{"script", "style", "head"} {
		for {
			start := strings.Index(strings.ToLower(text), "<"+tag)
			if start == -1 {
				break
			}
			closeTag := "</" + tag + ">"
			end := strings.Index(strings.ToLower(text[start:]), closeTag)
			if end == -1 {
				break
			}
			text = text[:start] + text[start+end+len(closeTag):]
		}
	}

	// Remove all HTML tags
	for {
		start := strings.Index(text, "<")
		if start == -1 {
			break
		}
		end := strings.Index(text[start:], ">")
		if end == -1 {
			break
		}
		text = text[:start] + " " + text[start+end+1:]
	}

	// Collapse whitespace
	lines := strings.Split(text, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}

	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}
