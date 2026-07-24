package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
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

		if err := validatePublicURL(params.URL); err != nil {
			return "", fmt.Errorf("web_fetch: %w", err)
		}

		client := &http.Client{
			Timeout: 20 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				if err := validatePublicURL(req.URL.String()); err != nil {
					return fmt.Errorf("redirect blocked: %w", err)
				}
				return nil
			},
		}
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

func validatePublicURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("scheme not allowed: %s", u.Scheme)
	}

	host := u.Hostname()
	if ip := net.ParseIP(host); ip != nil {
		if isPrivateIP(ip) {
			return fmt.Errorf("access to private/internal IP is not allowed")
		}
		return nil
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("DNS lookup failed for %s: %w", host, err)
	}
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return fmt.Errorf("host %s resolves to private IP %s", host, ip)
		}
	}
	return nil
}

func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	return ip.IsPrivate()
}
