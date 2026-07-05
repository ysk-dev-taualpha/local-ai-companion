package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/tool"
)

func NewWebSearch(webSearchURL, webSearchAPIKeyEnv string) tool.Executor {
	var apiKey string
	if webSearchAPIKeyEnv != "" {
		apiKey = os.Getenv(webSearchAPIKeyEnv)
	}
	return tool.ExecutorFunc(func(args json.RawMessage) (string, error) {
		var params struct {
			Query      string `json:"query"`
			MaxResults int    `json:"max_results,omitempty"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return "", fmt.Errorf("web_search: invalid args: %w", err)
		}
		if params.Query == "" {
			return "", fmt.Errorf("web_search: query is required")
		}
		if params.MaxResults <= 0 {
			params.MaxResults = 3
		}
		if webSearchURL != "" && apiKey != "" {
			return searchViaWebSearchAPI(webSearchURL, apiKey, params.Query, params.MaxResults)
		}
		return searchViaDuckDuckGo(params.Query, params.MaxResults)
	})
}

func searchViaWebSearchAPI(apiURL, apiKey, query string, maxResults int) (string, error) {
	reqBody := fmt.Sprintf(`{"query":%q}`, query)
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("web_search: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("web_search: API request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("web_search: API returned %d: %s", resp.StatusCode, string(body))
	}
	var result struct {
		Results []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("web_search: parse error: %w", err)
	}
	return formatSearchResults(result.Results, maxResults), nil
}

func formatSearchResults(results []struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
}, maxResults int) string {
	var out strings.Builder
	count := 0
	for _, r := range results {
		if count >= maxResults {
			break
		}
		fmt.Fprintf(&out, "%d. %s\n   URL: %s\n   %s\n\n", count+1, r.Title, r.URL, r.Description)
		count++
	}
	if out.Len() == 0 {
		return "no results found"
	}
	return strings.TrimSpace(out.String())
}

func searchViaDuckDuckGo(query string, maxResults int) (string, error) {
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(searchURL)
	if err != nil {
		return "", fmt.Errorf("web_search: duckduckgo failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	htmlStr := string(body)
	var results []struct{ title, snippet, link string }
	entries := strings.Split(htmlStr, "result__snippet")
	for i, entry := range entries {
		if i == 0 || i > maxResults+2 {
			continue
		}
		var r struct{ title, snippet, link string }
		r.snippet = extractBetween(entry, ">", "<")
		r.title = extractBetween(entry, "result__title", "</a>")
		if idx := strings.LastIndex(r.title, ">"); idx >= 0 {
			r.title = strings.TrimSpace(r.title[idx+1:])
		}
		if hrefStart := strings.Index(entry, "href=\""); hrefStart >= 0 {
			rest := entry[hrefStart+6:]
			if hrefEnd := strings.Index(rest, "\""); hrefEnd >= 0 {
				r.link = rest[:hrefEnd]
			}
		}
		if r.snippet != "" {
			results = append(results, r)
		}
	}
	if len(results) == 0 {
		return "no results found", nil
	}
	var out strings.Builder
	for i, r := range results {
		if i >= maxResults {
			break
		}
		fmt.Fprintf(&out, "%d. %s\n   URL: %s\n   %s\n\n", i+1, r.title, r.link, r.snippet)
	}
	return strings.TrimSpace(out.String()), nil
}

func extractBetween(s, start, end string) string {
	si := strings.Index(s, start)
	if si < 0 {
		return ""
	}
	s = s[si+len(start):]
	ei := strings.Index(s, end)
	if ei < 0 {
		return ""
	}
	return strings.TrimSpace(s[:ei])
}
