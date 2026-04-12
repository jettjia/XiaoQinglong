package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ========== GrepTool ==========

// GrepInput for grep tool
type GrepInput struct {
	Pattern   string   `json:"pattern"`             // Regex pattern to search
	Path      string   `json:"path"`               // Path to search in
	Glob      string   `json:"glob,omitempty"`     // File glob filter (e.g., "*.go")
	Context   int      `json:"context,omitempty"`  // Lines of context before/after
	CaseSensitive bool `json:"case_sensitive,omitempty"` // Case sensitive search
	Regex     bool     `json:"regex,omitempty"`    // Treat pattern as regex (default true)
	Head      int      `json:"head,omitempty"`     // Limit number of results
	Invert    bool     `json:"invert,omitempty"`   // Invert match (like grep -v)
}

// GrepMatch represents a single grep match
type GrepMatch struct {
	File string `json:"file"`      // File path
	Line int    `json:"line"`      // Line number
	Text string `json:"text"`      // Line content
}

// GrepOutput for grep results
type GrepOutput struct {
	Matches []GrepMatch `json:"matches"`
	Count   int         `json:"count"`
	Pattern string      `json:"pattern"`
	Path    string      `json:"path"`
}

// GrepTool searches file contents using regex
type GrepTool struct {
	basePath string
}

func NewGrepTool(basePath ...string) *GrepTool {
	if len(basePath) > 0 {
		return &GrepTool{basePath: basePath[0]}
	}
	return &GrepTool{basePath: "."}
}

func init() {
	GlobalRegistry.Register(ToolMeta{
		Name:           "Grep",
		Desc:           "Content search tool using ripgrep. Searches files for regex patterns.",
		IsReadOnly:     true,
		MaxResultChars: 200000, // 限制结果大小
		DefaultRisk:    "low",
		Creator: func(basePath string) interface{} {
			return NewGrepTool(basePath)
		},
	})
}

func (t *GrepTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "Grep",
		Desc: "Content search tool using ripgrep. Searches files for regex patterns.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"pattern": {
				Type:        schema.String,
				Desc:        "Regular expression pattern to search for",
				Required:    true,
			},
			"path": {
				Type:        schema.String,
				Desc:        "Path to search in (file or directory)",
				Required:    true,
			},
			"glob": {
				Type:        schema.String,
				Desc:        "Only search files matching glob pattern (e.g., \"*.go\")",
				Required:    false,
			},
			"context": {
				Type:        schema.Integer,
				Desc:        "Number of context lines before/after match",
				Required:    false,
			},
			"case_sensitive": {
				Type:        schema.Boolean,
				Desc:        "Case sensitive search (default true)",
				Required:    false,
			},
			"head": {
				Type:        schema.Integer,
				Desc:        "Limit number of results",
				Required:    false,
			},
		}),
	}, nil
}

func (t *GrepTool) ValidateInput(ctx context.Context, input string) *ValidationResult {
	var grepInput GrepInput
	if err := json.Unmarshal([]byte(input), &grepInput); err != nil {
		return &ValidationResult{Valid: false, Message: fmt.Sprintf("invalid JSON: %v", err), ErrorCode: 1}
	}
	if grepInput.Pattern == "" {
		return &ValidationResult{Valid: false, Message: "pattern is required", ErrorCode: 2}
	}
	if grepInput.Path == "" {
		return &ValidationResult{Valid: false, Message: "path is required", ErrorCode: 3}
	}
	return &ValidationResult{Valid: true}
}

func (t *GrepTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var grepInput GrepInput
	if err := json.Unmarshal([]byte(input), &grepInput); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	basePath := t.basePath
	if grepInput.Path != "" {
		basePath = grepInput.Path
	}

	// Build ripgrep command
	args := []string{"-n", "--json"}

	if grepInput.Context > 0 {
		args = append(args, "-C", strconv.Itoa(grepInput.Context))
	}

	if grepInput.CaseSensitive {
		args = append(args, "-s") // case sensitive (default)
	} else {
		args = append(args, "-i")
	}

	if grepInput.Invert {
		args = append(args, "-v")
	}

	if grepInput.Head > 0 {
		args = append(args, "-m", strconv.Itoa(grepInput.Head))
	}

	if grepInput.Glob != "" {
		args = append(args, "-g", grepInput.Glob)
	}

	args = append(args, grepInput.Pattern)
	args = append(args, basePath)

	cmd := exec.CommandContext(ctx, "rg", args...)
	output, err := cmd.Output()
	if err != nil {
		// Check if it's just "no matches" error
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			// No matches is not an error, return empty results
			result := GrepOutput{
				Matches: []GrepMatch{},
				Count:   0,
				Pattern: grepInput.Pattern,
				Path:    basePath,
			}
			out, _ := json.Marshal(result)
			return string(out), nil
		}
		return "", fmt.Errorf("grep failed: %w", err)
	}

	// Parse ripgrep JSON output
	matches := parseGrepJSONOutput(string(output))

	result := GrepOutput{
		Matches: matches,
		Count:   len(matches),
		Pattern: grepInput.Pattern,
		Path:    basePath,
	}

	out, _ := json.Marshal(result)
	return string(out), nil
}

// parseGrepJSONOutput parses ripgrep --json output
func parseGrepJSONOutput(output string) []GrepMatch {
	var matches []GrepMatch

	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		var rgResult struct {
			Type    string `json:"type"`
			File    string `json:"file"`
			LineNumber int `json:"line_number"`
			Lines    struct {
				Text string `json:"text"`
			} `json:"lines"`
		}

		if err := json.Unmarshal([]byte(line), &rgResult); err != nil {
			continue
		}

		if rgResult.Type == "match" {
			matches = append(matches, GrepMatch{
				File: rgResult.File,
				Line: rgResult.LineNumber,
				Text: strings.TrimSuffix(rgResult.Lines.Text, "\n"),
			})
		}
	}

	return matches
}

// GrepSimple does a simple regex search without ripgrep
func GrepSimple(pattern, path string, regex bool) ([]GrepMatch, error) {
	if regex {
		_, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regex: %w", err)
		}
	}

	// Fallback: use grep command
	cmd := exec.Command("grep", "-rn", pattern, path)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("grep failed: %w", err)
	}

	var matches []GrepMatch
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		// Parse grep output format: path:line:text
		parts := strings.SplitN(line, ":", 3)
		if len(parts) >= 3 {
			lineNum, _ := strconv.Atoi(parts[1])
			matches = append(matches, GrepMatch{
				File: parts[0],
				Line: lineNum,
				Text: parts[2],
			})
		}
	}

	return matches, nil
}
