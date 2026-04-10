package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ========== FileEditTool ==========

// FileEditInput for file edit tool
type FileEditInput struct {
	FilePath  string `json:"file_path"` // Path to file to edit
	OldString string `json:"old_string"` // String to replace
	NewString string `json:"new_string"` // Replacement string
}

// FileEditOutput for file edit result
type FileEditOutput struct {
	Success   bool   `json:"success"`    // Whether edit was successful
	NewContent string `json:"new_content"` // New file content (optional)
	Replaced  int    `json:"replaced"`   // Number of replacements made
}

// FileEditTool performs exact string replacements in files
type FileEditTool struct {
	basePath string
}

func NewFileEditTool(basePath ...string) *FileEditTool {
	if len(basePath) > 0 {
		return &FileEditTool{basePath: basePath[0]}
	}
	return &FileEditTool{basePath: "."}
}

func (t *FileEditTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "Edit",
		Desc: "Perform exact string replacements in files. Use this instead of sed or awk commands.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"file_path": {
				Type:        schema.String,
				Desc:        "Path to the file to edit",
				Required:    true,
			},
			"old_string": {
				Type:        schema.String,
				Desc:        "The exact string to replace (must match the source text exactly, including whitespace)",
				Required:    true,
			},
			"new_string": {
				Type:        schema.String,
				Desc:        "The replacement string",
				Required:    true,
			},
		}),
	}, nil
}

func (t *FileEditTool) ValidateInput(ctx context.Context, input string) *ValidationResult {
	var editInput FileEditInput
	if err := json.Unmarshal([]byte(input), &editInput); err != nil {
		return &ValidationResult{Valid: false, Message: fmt.Sprintf("invalid JSON: %v", err), ErrorCode: 1}
	}
	if editInput.FilePath == "" {
		return &ValidationResult{Valid: false, Message: "file_path is required", ErrorCode: 2}
	}
	if editInput.OldString == "" {
		return &ValidationResult{Valid: false, Message: "old_string is required", ErrorCode: 3}
	}
	if editInput.NewString == "" {
		return &ValidationResult{Valid: false, Message: "new_string is required", ErrorCode: 4}
	}
	return &ValidationResult{Valid: true}
}

func (t *FileEditTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var editInput FileEditInput
	if err := json.Unmarshal([]byte(input), &editInput); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	filePath := editInput.FilePath
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(t.basePath, filePath)
	}

	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("cannot read file: %w", err)
	}

	// Count occurrences of old_string before replacing
	oldStr := editInput.OldString
	newStr := editInput.NewString

	// Check if old_string exists
	if !strings.Contains(string(content), oldStr) {
		return "", fmt.Errorf("old_string not found in file: %s", oldStr)
	}

	// Perform replacement
	newContent := strings.ReplaceAll(string(content), oldStr, newStr)

	// Count replacements
	replaced := strings.Count(string(content), oldStr)

	// Write back
	if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("cannot write file: %w", err)
	}

	output := FileEditOutput{
		Success:  true,
		Replaced: replaced,
	}

	result, _ := json.Marshal(output)
	return string(result), nil
}
