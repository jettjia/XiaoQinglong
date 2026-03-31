package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FileExtractor 文件内容提取器
type FileExtractor struct {
	baseDir string // 宿主机的 uploads 目录，如 /home/jett/aishu/XiaoQinglong/backend/agent-frame/data/uploads
}

// NewFileExtractor 创建文件提取器
func NewFileExtractor(baseDir string) *FileExtractor {
	return &FileExtractor{baseDir: baseDir}
}

// ExtractFilesContent 提取多个文件的内容
func (e *FileExtractor) ExtractFilesContent(files []FileConfig) (string, error) {
	if len(files) == 0 {
		return "", nil
	}

	var lines []string
	lines = append(lines, "<uploaded_files_content>", "")

	for _, f := range files {
		content, err := e.extractFileContent(f)
		if err != nil {
			lines = append(lines, fmt.Sprintf("文件: %s", f.Name))
			lines = append(lines, fmt.Sprintf("错误: %v", err))
			lines = append(lines, "")
			continue
		}

		lines = append(lines, fmt.Sprintf("文件: %s", f.Name))
		lines = append(lines, "---")
		lines = append(lines, content)
		lines = append(lines, "---")
		lines = append(lines, "")
	}

	lines = append(lines, "</uploaded_files_content>")
	return strings.Join(lines, "\n"), nil
}

// extractFileContent 提取单个文件的内容
func (e *FileExtractor) extractFileContent(f FileConfig) (string, error) {
	// 将虚拟路径转换为实际路径
	// /mnt/uploads/session_id/file.md -> {baseDir}/session_id/file.md
	realPath := e.virtualToReal(f.VirtualPath)

	// 检查文件是否存在
	if _, err := os.Stat(realPath); err != nil {
		return "", fmt.Errorf("file not found: %s", realPath)
	}

	// 根据扩展名判断处理方式
	ext := strings.ToLower(filepath.Ext(realPath))
	if isTextFile(ext) {
		return e.readTextFile(realPath)
	}

	// 二进制文件使用 markitdown
	return e.extractWithMarkitdown(realPath)
}

// virtualToReal 将虚拟路径转换为实际路径
// /mnt/uploads/session_id/file.md -> {baseDir}/session_id/file.md
func (e *FileExtractor) virtualToReal(virtualPath string) string {
	log.Printf("[FileExtractor] virtualToReal: virtualPath=%s, baseDir=%s", virtualPath, e.baseDir)
	// 去掉 /mnt/uploads/ 前缀
	if strings.HasPrefix(virtualPath, "/mnt/uploads/") {
		relativePath := strings.TrimPrefix(virtualPath, "/mnt/uploads/")
		realPath := filepath.Join(e.baseDir, relativePath)
		log.Printf("[FileExtractor] virtualToReal: converted to realPath=%s", realPath)
		return realPath
	}
	// 如果不是虚拟路径格式，直接返回
	log.Printf("[FileExtractor] virtualToReal: not a virtual path, returning as-is")
	return virtualPath
}

// isTextFile 判断是否为文本文件
func isTextFile(ext string) bool {
	textExts := map[string]bool{
		".txt": true, ".md": true, ".markdown": true,
		".csv": true, ".json": true, ".xml": true,
		".html": true, ".htm": true,
		".py": true, ".js": true, ".ts": true,
		".go": true, ".java": true, ".c": true, ".cpp": true, ".h": true,
		".sh": true, ".bash": true,
		".yaml": true, ".yml": true, ".toml": true,
		".ini": true, ".conf": true, ".log": true,
		".sql": true, ".proto": true,
	}
	return textExts[ext]
}

// readTextFile 读取文本文件
func (e *FileExtractor) readTextFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file failed: %w", err)
	}
	return string(content), nil
}

// extractWithMarkitdown 使用 markitdown 提取内容
func (e *FileExtractor) extractWithMarkitdown(path string) (string, error) {
	// 检查 markitdown 是否可用
	cmd := exec.Command("markitdown", "--version")
	if err := cmd.Run(); err != nil {
		// markitdown 不可用，尝试 pip 安装
		if installErr := e.installMarkitdown(); installErr != nil {
			return "", fmt.Errorf("markitdown not available and install failed: %w", installErr)
		}
	}

	// 执行 markitdown
	cmd = exec.Command("markitdown", path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("markitdown failed: %w, stderr: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// installMarkitdown 安装 markitdown
func (e *FileExtractor) installMarkitdown() error {
	cmd := exec.Command("pip", "install", "-q", "markitdown[all]")
	return cmd.Run()
}

// BuildFilesBlock 构建 uploaded_files 块（供 agent-frame 使用）
func BuildFilesBlock(files []FileConfig) string {
	if len(files) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "<uploaded_files>", "")

	for _, f := range files {
		sizeKB := float64(f.Size) / 1024
		sizeStr := fmt.Sprintf("%.1f KB", sizeKB)
		if sizeKB >= 1024 {
			sizeStr = fmt.Sprintf("%.1f MB", sizeKB/1024)
		}
		lines = append(lines, fmt.Sprintf("- %s (%s)", f.Name, sizeStr))
		lines = append(lines, fmt.Sprintf("  Path: %s", f.VirtualPath))
		lines = append(lines, "")
	}

	lines = append(lines, "</uploaded_files>")
	return strings.Join(lines, "\n")
}
