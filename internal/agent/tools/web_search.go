package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// WebSearchParams is the JSON schema for the web_search tool.
type WebSearchParams struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results,omitempty"`
}

// WebSearch is a tool that searches the web using DuckDuckGo's API.
type WebSearch struct {
	httpClient *http.Client
}

// NewWebSearch creates a new WebSearch tool.
func NewWebSearch(timeout time.Duration) *WebSearch {
	return &WebSearch{
		httpClient: &http.Client{Timeout: timeout},
	}
}

// Name returns the tool name.
func (w *WebSearch) Name() string {
	return "web_search"
}

// Description returns the tool description.
func (w *WebSearch) Description() string {
	return "Search the web for information. Returns a list of results with titles, URLs, and snippets."
}

// Parameters returns the JSON Schema for the tool's parameters.
func (w *WebSearch) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"description": "The search query"
			},
			"max_results": {
				"type": "integer",
				"description": "Maximum number of results to return (default: 5, max: 10)",
				"default": 5
			}
		},
		"required": ["query"]
	}`)
}

// Execute runs the web search.
func (w *WebSearch) Execute(ctx context.Context, arguments json.RawMessage) (string, error) {
	var params WebSearchParams
	if err := json.Unmarshal(arguments, &params); err != nil {
		return "", fmt.Errorf("web_search: invalid arguments: %w", err)
	}

	if params.Query == "" {
		return "", fmt.Errorf("web_search: query is required")
	}

	if params.MaxResults <= 0 {
		params.MaxResults = 5
	}
	if params.MaxResults > 10 {
		params.MaxResults = 10
	}

	// Use DuckDuckGo Instant Answer API (no API key required)
	apiURL := "https://api.duckduckgo.com/?" + url.Values{
		"q":     {params.Query},
		"format": {"json"},
		"no_html": {"1"},
		"skip_disambig": {"1"},
	}.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("web_search: request creation failed: %w", err)
	}
	req.Header.Set("User-Agent", "local-ai-companion/0.1")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("web_search: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return "", fmt.Errorf("web_search: failed to read response: %w", err)
	}

	var ddgResp struct {
		AbstractText string `json:"AbstractText"`
		AbstractURL  string `json:"AbstractURL"`
		Heading      string `json:"Heading"`
		RelatedTopics []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"RelatedTopics"`
	}

	if err := json.Unmarshal(body, &ddgResp); err != nil {
		return "", fmt.Errorf("web_search: failed to parse response: %w", err)
	}

	results := []map[string]string{}

	if ddgResp.AbstractText != "" {
		results = append(results, map[string]string{
			"title":   ddgResp.Heading,
			"url":     ddgResp.AbstractURL,
			"snippet": truncate(ddgResp.AbstractText, 500),
		})
	}

	for _, topic := range ddgResp.RelatedTopics {
		if len(results) >= params.MaxResults {
			break
		}
		if topic.Text == "" {
			continue
		}
		results = append(results, map[string]string{
			"title":   extractTitle(topic.Text),
			"url":     topic.FirstURL,
			"snippet": truncate(stripHTML(topic.Text), 500),
		})
	}

	if len(results) == 0 {
		return fmt.Sprintf(`{"results": [], "query": %q, "message": "no results found"}`, params.Query), nil
	}

	out, _ := json.Marshal(map[string]interface{}{
		"results": results,
		"query":   params.Query,
	})
	return string(out), nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func stripHTML(s string) string {
	// Simple HTML tag removal
	result := s
	for {
		start := strings.Index(result, "<")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], ">")
		if end == -1 {
			break
		}
		result = result[:start] + result[start+end+1:]
	}
	return strings.TrimSpace(result)
}

func extractTitle(s string) string {
	// Use first 50 chars of the snippet as title
	if s == "" {
		return "Untitled"
	}
	return truncate(strings.SplitN(s, ".", 2)[0], 80)
}
