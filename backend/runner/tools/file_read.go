package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ========== FileReadTool ==========

// FileReadInput for file read tool
type FileReadInput struct {
	FilePath       string `json:"file_path"` // Path to file to read
	Offset        int    `json:"offset,omitempty"`        // Line offset to start reading from
	Limit         int    `json:"limit,omitempty"`        // Maximum number of lines to read
	ShowLineNumbers bool   `json:"show_line_numbers,omitempty"` // Include line numbers in output
}

// FileReadOutput for file read result
type FileReadOutput struct {
	Content    string `json:"content"`     // File content
	Lines      int    `json:"lines"`      // Number of lines read
	Truncated  bool   `json:"truncated"`  // Whether output was truncated
	FullPath   string `json:"full_path"`  // Full resolved path
	FileSize   int64  `json:"file_size"`  // Original file size in bytes
}

// FileReadTool reads files from the filesystem
type FileReadTool struct {
	basePath string
}

func NewFileReadTool(basePath ...string) *FileReadTool {
	if len(basePath) > 0 {
		return &FileReadTool{basePath: basePath[0]}
	}
	return &FileReadTool{basePath: "."}
}

func init() {
	GlobalRegistry.Register(ToolMeta{
		Name:           "Read",
		Desc:           "Read the contents of a file from the local filesystem. Use this instead of cat, head, tail, or sed commands.",
		IsReadOnly:     true,
		MaxResultChars: 500000, // 限制结果大小
		DefaultRisk:    "low",
		Creator: func(basePath string) interface{} {
			return NewFileReadTool(basePath)
		},
	})
}

func (t *FileReadTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "Read",
		Desc: "Read the contents of a file from the local filesystem. Use this instead of cat, head, tail, or sed commands.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"file_path": {
				Type:        schema.String,
				Desc:        "Path to the file to read",
				Required:    true,
			},
			"offset": {
				Type:        schema.Integer,
				Desc:        "Line offset to start reading from (0-indexed)",
				Required:    false,
			},
			"limit": {
				Type:        schema.Integer,
				Desc:        "Maximum number of lines to read",
				Required:    false,
			},
			"show_line_numbers": {
				Type:        schema.Boolean,
				Desc:        "Include line numbers in output",
				Required:    false,
			},
		}),
	}, nil
}

func (t *FileReadTool) ValidateInput(ctx context.Context, input string) *ValidationResult {
	var readInput FileReadInput
	if err := json.Unmarshal([]byte(input), &readInput); err != nil {
		return &ValidationResult{Valid: false, Message: fmt.Sprintf("invalid JSON: %v", err), ErrorCode: 1}
	}
	if readInput.FilePath == "" {
		return &ValidationResult{Valid: false, Message: "file_path is required", ErrorCode: 2}
	}
	return &ValidationResult{Valid: true}
}

func (t *FileReadTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var readInput FileReadInput
	if err := json.Unmarshal([]byte(input), &readInput); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	basePath := t.basePath
	filePath := readInput.FilePath
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(basePath, filePath)
	}

	// Check if file exists
	info, err := os.Stat(filePath)
	if err != nil {
		return "", fmt.Errorf("file not found: %s", filePath)
	}

	if info.IsDir() {
		return "", fmt.Errorf("path is a directory, not a file: %s", filePath)
	}

	// Open file
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("cannot open file: %w", err)
	}
	defer f.Close()

	fileSize := info.Size()

	// Handle line-based reading
	var content string
	if readInput.Limit > 0 || readInput.Offset > 0 {
		content, err = t.readLines(f, readInput.Offset, readInput.Limit, readInput.ShowLineNumbers)
		if err != nil {
			return "", fmt.Errorf("error reading file: %w", err)
		}
	} else {
		// Read entire file (with size limit)
		maxSize := int64(1 << 20) // 1MB
		buf := make([]byte, maxSize)
		n, err := f.ReadAt(buf, 0)
		if err != nil && err != io.EOF {
			return "", fmt.Errorf("error reading file: %w", err)
		}
		content = string(buf[:n])
	}

	output := FileReadOutput{
		Content:   content,
		Lines:     strings.Count(content, "\n") + 1,
		Truncated: fileSize > int64(len(content)),
		FullPath:  filePath,
		FileSize:  fileSize,
	}

	result, _ := json.Marshal(output)
	return string(result), nil
}

func (t *FileReadTool) readLines(f *os.File, offset, limit int, showLineNumbers bool) (string, error) {
	scanner := bufio.NewScanner(f)
	var lines []string
	lineNum := 0

	for scanner.Scan() {
		if lineNum < offset {
			lineNum++
			continue
		}
		if limit > 0 && lineNum >= offset+limit {
			break
		}

		line := scanner.Text()
		if showLineNumbers {
			line = fmt.Sprintf("%d: %s", lineNum+1, line)
		}
		lines = append(lines, line)
		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return strings.Join(lines, "\n"), nil
}
