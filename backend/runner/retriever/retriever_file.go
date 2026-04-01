package retriever

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/components/document/parser"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/jettjia/XiaoQinglong/runner/pkg/logger"
)

// ========== File Parser Retriever ==========

// FileParserRetriever 文件解析检索器
type FileParserRetriever struct {
	baseDir string
	parser  parser.Parser
}

// NewFileParserRetriever 创建文件解析检索器
func NewFileParserRetriever(baseDir string) *FileParserRetriever {
	return &FileParserRetriever{
		baseDir: baseDir,
		parser:  parser.TextParser{},
	}
}

// Retrieve 解析指定路径的文件内容
// query 格式: /path/to/file.md 或 ["file1.md", "file2.md"] 或 *
func (fpr *FileParserRetriever) Retrieve(ctx context.Context, query string) ([]*schema.Document, error) {
	var docs []*schema.Document

	if strings.HasPrefix(query, "[") {
		var files []string
		if err := json.Unmarshal([]byte(query), &files); err != nil {
			return nil, fmt.Errorf("parse file list failed: %w", err)
		}
		for _, file := range files {
			doc, err := fpr.parseFile(ctx, file)
			if err != nil {
				logger.Warnf("[FileParserRetriever] parse file %s failed: %v", file, err)
				continue
			}
			docs = append(docs, doc)
		}
	} else if query == "*" {
		entries, err := os.ReadDir(fpr.baseDir)
		if err != nil {
			return nil, fmt.Errorf("read dir failed: %w", err)
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				doc, err := fpr.parseFile(ctx, entry.Name())
				if err != nil {
					continue
				}
				docs = append(docs, doc)
			}
		}
	} else {
		doc, err := fpr.parseFile(ctx, query)
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}

	return docs, nil
}

// parseFile 解析单个文件
func (fpr *FileParserRetriever) parseFile(ctx context.Context, filePath string) (*schema.Document, error) {
	realPath := fpr.virtualToReal(filePath)

	if _, err := os.Stat(realPath); err != nil {
		return nil, fmt.Errorf("file not found: %s", realPath)
	}

	ext := strings.ToLower(filepath.Ext(realPath))
	p := fpr.getParser(ext)

	file, err := os.Open(realPath)
	if err != nil {
		return nil, fmt.Errorf("open file failed: %w", err)
	}
	defer file.Close()

	parsedDocs, err := p.Parse(ctx, file, parser.WithURI(realPath))
	if err != nil {
		content, markitdownErr := fpr.extractWithMarkitdown(realPath)
		if markitdownErr != nil {
			return nil, fmt.Errorf("parse failed: %w, markitdown also failed: %v", err, markitdownErr)
		}
		return &schema.Document{
			ID:       filePath,
			Content:  content,
			MetaData: map[string]any{"file": filePath},
		}, nil
	}

	if len(parsedDocs) == 0 {
		return &schema.Document{
			ID:       filePath,
			Content:  "",
			MetaData: map[string]any{"file": filePath},
		}, nil
	}

	doc := parsedDocs[0]
	doc.MetaData["file"] = filePath
	return doc, nil
}

func (fpr *FileParserRetriever) virtualToReal(virtualPath string) string {
	if strings.HasPrefix(virtualPath, "/mnt/uploads/") {
		relativePath := strings.TrimPrefix(virtualPath, "/mnt/uploads/")
		return filepath.Join(fpr.baseDir, relativePath)
	}
	return virtualPath
}

func (fpr *FileParserRetriever) getParser(ext string) parser.Parser {
	return fpr.parser
}

func (fpr *FileParserRetriever) extractWithMarkitdown(path string) (string, error) {
	cmd := exec.Command("markitdown", "--version")
	if err := cmd.Run(); err != nil {
		if installErr := exec.Command("pip", "install", "-q", "markitdown[all]").Run(); installErr != nil {
			return "", fmt.Errorf("markitdown not available: %w", installErr)
		}
	}

	cmd = exec.Command("markitdown", path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("markitdown failed: %w, stderr: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// ========== File Retrieval Tool ==========

// CreateFileRetrievalTool 创建文件检索工具（供 Agent 调用）
func CreateFileRetrievalTool(uploadsBase string) tool.BaseTool {
	fpr := NewFileParserRetriever(uploadsBase)
	return &fileRetrievalTool{fpr: fpr}
}

type fileRetrievalTool struct {
	fpr *FileParserRetriever
}

func (t *fileRetrievalTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "parse_file",
		Desc: "解析并获取上传文件的内容。支持 txt, md, json, pdf, docx 等格式。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"file_path": {Type: schema.String, Desc: "文件路径或路径数组，如 /mnt/uploads/session_id/file.md 或 [\"file1.md\", \"file2.md\"]", Required: true},
		}),
	}, nil
}

func (t *fileRetrievalTool) InvokableRun(ctx context.Context, argumentsInJSON string, opt ...tool.Option) (string, error) {
	var args struct {
		FilePath any `json:"file_path"` // string or []string
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("parse arguments failed: %w", err)
	}

	var filePaths []string
	switch v := args.FilePath.(type) {
	case string:
		filePaths = []string{v}
	case []any:
		for _, p := range v {
			if s, ok := p.(string); ok {
				filePaths = append(filePaths, s)
			}
		}
	default:
		return "", fmt.Errorf("invalid file_path type: %T", args.FilePath)
	}

	if len(filePaths) == 0 {
		return "", fmt.Errorf("file_path is required")
	}

	var buf bytes.Buffer
	for i, path := range filePaths {
		content, err := t.fpr.parseFile(ctx, path)
		if err != nil {
			buf.WriteString(fmt.Sprintf("文件 %s: 解析失败 - %v\n", path, err))
			continue
		}
		if i > 0 {
			buf.WriteString("\n---\n\n")
		}
		buf.WriteString(fmt.Sprintf("【文件: %s】\n%s", path, content.Content))
	}

	return buf.String(), nil
}
