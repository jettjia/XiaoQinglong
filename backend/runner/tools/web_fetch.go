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

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ========== WebFetchTool ==========

// WebFetchInput for web fetch tool
type WebFetchInput struct {
	URL     string `json:"url"`      // URL to fetch
	Prompt  string `json:"prompt"`   // What to extract from the page
	Cache   bool   `json:"cache,omitempty"` // Use cached result (default true)
}

// WebFetchOutput for web fetch result
type WebFetchOutput struct {
	Content   string `json:"content"`    // Fetched content
	Title     string `json:"title,omitempty"` // Page title
	StatusCode int   `json:"status_code"`   // HTTP status code
	URL       string `json:"url"`          // Final URL (after redirects)
}

// WebFetchTool fetches and analyzes web content
type WebFetchTool struct {
	client    *http.Client
	cache     map[string]*cachedFetch
	cacheTTL  time.Duration
}

type cachedFetch struct {
	content  string
	title    string
	fetchedAt time.Time
}

func NewWebFetchTool() *WebFetchTool {
	return &WebFetchTool{
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse // Don't follow redirects automatically
			},
		},
		cache:    make(map[string]*cachedFetch),
		cacheTTL: 15 * time.Minute,
	}
}

func (t *WebFetchTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "WebFetch",
		Desc: "Fetch and analyze web content from a URL. Use this to retrieve information from web pages.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"url": {
				Type:        schema.String,
				Desc:        "URL to fetch content from",
				Required:    true,
			},
			"prompt": {
				Type:        schema.String,
				Desc:        "Description of what to extract from the page",
				Required:    false,
			},
			"cache": {
				Type:        schema.Boolean,
				Desc:        "Use cached result if available (default true)",
				Required:    false,
			},
		}),
	}, nil
}

func (t *WebFetchTool) ValidateInput(ctx context.Context, input string) *ValidationResult {
	var fetchInput WebFetchInput
	if err := json.Unmarshal([]byte(input), &fetchInput); err != nil {
		return &ValidationResult{Valid: false, Message: fmt.Sprintf("invalid JSON: %v", err), ErrorCode: 1}
	}
	if fetchInput.URL == "" {
		return &ValidationResult{Valid: false, Message: "url is required", ErrorCode: 2}
	}
	// Validate URL format
	if _, err := url.Parse(fetchInput.URL); err != nil {
		return &ValidationResult{Valid: false, Message: fmt.Sprintf("invalid URL: %v", err), ErrorCode: 3}
	}
	return &ValidationResult{Valid: true}
}

func (t *WebFetchTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var fetchInput WebFetchInput
	if err := json.Unmarshal([]byte(input), &fetchInput); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	// Check cache first
	useCache := fetchInput.Cache != false // default true
	if useCache {
		if cached, ok := t.cache[fetchInput.URL]; ok {
			if time.Since(cached.fetchedAt) < t.cacheTTL {
				output := WebFetchOutput{
					Content:   cached.content,
					Title:     cached.title,
					URL:       fetchInput.URL,
					StatusCode: 200,
				}
				result, _ := json.Marshal(output)
				return string(result), nil
			}
		}
	}

	// Fetch URL
	req, err := http.NewRequestWithContext(ctx, "GET", fetchInput.URL, nil)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; RunnerBot/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle redirects
	finalURL := fetchInput.URL
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		// Get redirect URL from Location header
		if loc := resp.Header.Get("Location"); loc != "" {
			if strings.HasPrefix(loc, "/") {
				u, _ := url.Parse(fetchInput.URL)
				finalURL = u.Scheme + "://" + u.Host + loc
			} else {
				finalURL = loc
			}
		}
	}

	// Read body with size limit (1MB)
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read body failed: %w", err)
	}

	content := string(body)

	// Extract title from HTML if present
	title := extractHTMLTitle(content)

	// Simple content extraction (remove HTML tags for plain text)
	if strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
		content = stripHTML(content)
	}

	// Cache result
	if useCache {
		t.cache[fetchInput.URL] = &cachedFetch{
			content:  content,
			title:    title,
			fetchedAt: time.Now(),
		}
	}

	output := WebFetchOutput{
		Content:   content,
		Title:     title,
		StatusCode: resp.StatusCode,
		URL:       finalURL,
	}

	result, _ := json.Marshal(output)
	return string(result), nil
}

func extractHTMLTitle(html string) string {
	const start = "<title>"
	const end = "</title>"
	if i := strings.Index(strings.ToLower(html), start); i >= 0 {
		startIdx := i + len(start)
		if j := strings.Index(strings.ToLower(html[startIdx:]), end); j >= 0 {
			return strings.TrimSpace(html[startIdx : startIdx+j])
		}
	}
	return ""
}

func stripHTML(html string) string {
	// Simple HTML tag stripper
	var result strings.Builder
	inTag := false
	for _, r := range html {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
}
