package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ========== GlobTool ==========

// GlobInput for glob tool
type GlobInput struct {
	Pattern string `json:"pattern"` // Glob pattern (e.g., "**/*.go")
	Path    string `json:"path"`    // Base path to search from (optional, defaults to current directory)
}

// GlobTool matches files by glob patterns
type GlobTool struct {
	basePath string
}

func NewGlobTool(basePath ...string) *GlobTool {
	if len(basePath) > 0 {
		return &GlobTool{basePath: basePath[0]}
	}
	return &GlobTool{basePath: "."}
}

func init() {
	GlobalRegistry.Register(ToolMeta{
		Name:           "Glob",
		Desc:           "Fast file pattern matching tool. Use this to find files by name patterns (e.g., **/*.go, *.json).",
		IsReadOnly:     true,
		MaxResultChars: 100000, // 限制结果大小
		DefaultRisk:    "low",
		Creator: func(basePath string) interface{} {
			return NewGlobTool(basePath)
		},
	})
}

func (t *GlobTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "Glob",
		Desc: "Fast file pattern matching tool. Use this to find files by name patterns (e.g., **/*.go, *.json).",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"pattern": {
				Type:        schema.String,
				Desc:        "Glob pattern to match files (e.g., \"**/*.go\", \"src/**/*.ts\", \"*.json\")",
				Required:    true,
			},
			"path": {
				Type:        schema.String,
				Desc:        "Base path to search from (defaults to current directory)",
				Required:    false,
			},
		}),
	}, nil
}

func (t *GlobTool) ValidateInput(ctx context.Context, input string) *ValidationResult {
	var globInput GlobInput
	if err := json.Unmarshal([]byte(input), &globInput); err != nil {
		return &ValidationResult{Valid: false, Message: fmt.Sprintf("invalid JSON: %v", err), ErrorCode: 1}
	}
	if globInput.Pattern == "" {
		return &ValidationResult{Valid: false, Message: "pattern is required", ErrorCode: 2}
	}
	return &ValidationResult{Valid: true}
}

func (t *GlobTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var globInput GlobInput
	if err := json.Unmarshal([]byte(input), &globInput); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	basePath := t.basePath
	if globInput.Path != "" {
		basePath = globInput.Path
	}

	// Use filepath.Glob for matching
	pattern := globInput.Pattern
	if !filepath.IsAbs(pattern) {
		pattern = filepath.Join(basePath, pattern)
	}

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("glob failed: %w", err)
	}

	// Convert to absolute paths
	absMatches := make([]string, 0, len(matches))
	for _, m := range matches {
		absPath, err := filepath.Abs(m)
		if err == nil {
			absMatches = append(absMatches, absPath)
		}
	}

	result := map[string]interface{}{
		"matches": absMatches,
		"count":   len(absMatches),
		"pattern": globInput.Pattern,
		"path":    basePath,
	}

	output, _ := json.Marshal(result)
	return string(output), nil
}

// ========== Glob Implementation Details ==========

// MatchedFiles represents the result of a glob operation
type MatchedFiles struct {
	Files []string `json:"files"`
	Count int      `json:"count"`
}

// WalkGlob walks the directory tree and matches files
func WalkGlob(root, pattern string) ([]string, error) {
	var matches []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		matched, err := filepath.Match(pattern, filepath.Base(path))
		if err != nil {
			return err
		}
		if matched && !info.IsDir() {
			matches = append(matches, path)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return matches, nil
}
