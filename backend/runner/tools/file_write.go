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

// ========== FileWriteTool ==========

// FileWriteInput for file write tool
type FileWriteInput struct {
	FilePath string `json:"file_path"` // Path to file to write
	Content  string `json:"content"`  // Content to write
	Append   bool   `json:"append,omitempty"` // Append to file instead of overwriting
}

// FileWriteOutput for file write result
type FileWriteOutput struct {
	Success    bool   `json:"success"`     // Whether write was successful
	BytesWritten int64 `json:"bytes_written"` // Number of bytes written
	FullPath   string `json:"full_path"`   // Full path to written file
}

// FileWriteTool writes files to the filesystem
type FileWriteTool struct {
	basePath string
}

func NewFileWriteTool(basePath ...string) *FileWriteTool {
	if len(basePath) > 0 {
		return &FileWriteTool{basePath: basePath[0]}
	}
	return &FileWriteTool{basePath: "."}
}

func (t *FileWriteTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "Write",
		Desc: "Write content to a file. Creates a new file or overwrites existing file. Use this instead of echo redirection or cat with heredoc.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"file_path": {
				Type:        schema.String,
				Desc:        "Path to the file to write",
				Required:    true,
			},
			"content": {
				Type:        schema.String,
				Desc:        "Content to write to the file",
				Required:    true,
			},
			"append": {
				Type:        schema.Boolean,
				Desc:        "Append to existing file instead of overwriting",
				Required:    false,
			},
		}),
	}, nil
}

func (t *FileWriteTool) ValidateInput(ctx context.Context, input string) *ValidationResult {
	var writeInput FileWriteInput
	if err := json.Unmarshal([]byte(input), &writeInput); err != nil {
		return &ValidationResult{Valid: false, Message: fmt.Sprintf("invalid JSON: %v", err), ErrorCode: 1}
	}
	if writeInput.FilePath == "" {
		return &ValidationResult{Valid: false, Message: "file_path is required", ErrorCode: 2}
	}
	if writeInput.Content == "" {
		return &ValidationResult{Valid: false, Message: "content is required", ErrorCode: 3}
	}
	return &ValidationResult{Valid: true}
}

func (t *FileWriteTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var writeInput FileWriteInput
	if err := json.Unmarshal([]byte(input), &writeInput); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	filePath := writeInput.FilePath
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(t.basePath, filePath)
	}

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("cannot create directory: %w", err)
	}

	var bytesWritten int64
	var err error

	if writeInput.Append {
		// Append to file
		f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return "", fmt.Errorf("cannot open file for append: %w", err)
		}
		defer f.Close()

		n, err := f.WriteString(writeInput.Content)
		bytesWritten = int64(n)
		if err != nil {
			return "", fmt.Errorf("cannot append to file: %w", err)
		}
	} else {
		// Write/overwrite file
		err = os.WriteFile(filePath, []byte(writeInput.Content), 0644)
		if err != nil {
			return "", fmt.Errorf("cannot write file: %w", err)
		}
		bytesWritten = int64(len(writeInput.Content))
	}

	output := FileWriteOutput{
		Success:     true,
		BytesWritten: bytesWritten,
		FullPath:    filePath,
	}

	result, _ := json.Marshal(output)
	return string(result), nil
}
