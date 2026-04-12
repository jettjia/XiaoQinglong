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

// ========== WebSearchTool ==========

// WebSearchInput for web search tool
type WebSearchInput struct {
	Query string `json:"query"` // Search query
	Count int    `json:"count,omitempty"` // Number of results (default 10)
}

// WebSearchOutput for web search result
type WebSearchOutput struct {
	Results []SearchResult `json:"results"`
	Query   string        `json:"query"`
}

// SearchResult represents a single search result
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// WebSearchTool searches the web
type WebSearchTool struct {
	client *http.Client
}

func NewWebSearchTool() *WebSearchTool {
	return &WebSearchTool{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func init() {
	GlobalRegistry.Register(ToolMeta{
		Name:           "WebSearch",
		Desc:           "Search the web for current events and recent information. Use this for queries that require up-to-date information not in the model's training data.",
		IsReadOnly:     true,
		MaxResultChars: 50000,
		DefaultRisk:    "low",
		Creator: func(basePath string) interface{} {
			return NewWebSearchTool()
		},
	})
}

func (t *WebSearchTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "WebSearch",
		Desc: "Search the web for current events and recent information. Use this for queries that require up-to-date information not in the model's training data.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"query": {
				Type:        schema.String,
				Desc:        "The search query",
				Required:    true,
			},
			"count": {
				Type:        schema.Integer,
				Desc:        "Number of results to return (default 10)",
				Required:    false,
			},
		}),
	}, nil
}

func (t *WebSearchTool) ValidateInput(ctx context.Context, input string) *ValidationResult {
	var searchInput WebSearchInput
	if err := json.Unmarshal([]byte(input), &searchInput); err != nil {
		return &ValidationResult{Valid: false, Message: fmt.Sprintf("invalid JSON: %v", err), ErrorCode: 1}
	}
	if searchInput.Query == "" {
		return &ValidationResult{Valid: false, Message: "query is required", ErrorCode: 2}
	}
	return &ValidationResult{Valid: true}
}

func (t *WebSearchTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var searchInput WebSearchInput
	if err := json.Unmarshal([]byte(input), &searchInput); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	count := searchInput.Count
	if count <= 0 || count > 20 {
		count = 10
	}

	// Use DuckDuckGo HTML search (no API key required)
	query := url.QueryEscape(searchInput.Query)
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", query)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; RunnerBot/1.0)")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body failed: %w", err)
	}

	results := parseDuckDuckGoResults(string(body), count)

	output := WebSearchOutput{
		Results: results,
		Query:   searchInput.Query,
	}

	result, _ := json.Marshal(output)
	return string(result), nil
}

func parseDuckDuckGoResults(html string, count int) []SearchResult {
	var results []SearchResult

	// Simple HTML parsing for DuckDuckGo results
	// Each result is in a div with class "result"
	lines := strings.Split(html, "\n")
	var currentResult *SearchResult

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Look for result headers
		if strings.Contains(line, "<a class=\"result__a\" href=\"") {
			// Extract URL
			hrefStart := strings.Index(line, "href=\"") + 6
			hrefEnd := strings.Index(line[hrefStart:], "\"")
			if hrefEnd > 0 {
				resultURL := line[hrefStart : hrefStart+hrefEnd]
				currentResult = &SearchResult{URL: resultURL}
			}

			// Extract title (between > and </a>)
			titleStart := strings.Index(line, "\">") + 2
			titleEnd := strings.Index(line, "</a>")
			if titleEnd > titleStart && currentResult != nil {
				currentResult.Title = stripHTML(line[titleStart:titleEnd])
			}
		}

		// Look for snippet
		if strings.Contains(line, "<a class=\"result__snippet\"") && currentResult != nil {
			// Extract snippet
			snippetStart := strings.Index(line, "\">") + 2
			snippetEnd := strings.Index(line, "</a>")
			if snippetEnd > snippetStart {
				currentResult.Snippet = stripHTML(line[snippetStart:snippetEnd])
			}

			results = append(results, *currentResult)
			currentResult = nil

			if len(results) >= count {
				break
			}
		}
	}

	return results
}
