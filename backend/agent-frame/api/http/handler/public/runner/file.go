package runner

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"github.com/jettjia/xiaoqinglong/agent-frame/pkg/xqldir"
)

// FileInfo 文件信息
type FileInfo struct {
	Name        string `json:"name"`
	VirtualPath string `json:"virtual_path"`
	Size        int64  `json:"size"`
	Type        string `json:"type"`
}

// Upload 文件上传
func (h *Handler) Upload(c *gin.Context) {
	// 1. 获取 session_id
	sessionID := c.PostForm("session_id")
	if sessionID == "" {
		c.JSON(400, gin.H{"error": "session_id is required"})
		return
	}

	// 2. 获取上传目录
	uploadDir := xqldir.GetUploadsDir()
	uploadDir = filepath.Join(uploadDir, sessionID)

	// 3. 创建上传目录
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		c.JSON(500, gin.H{"error": "failed to create upload directory: " + err.Error()})
		return
	}

	// 4. 获取文件
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(400, gin.H{"error": "failed to parse multipart form: " + err.Error()})
		return
	}

	fileHeaders := form.File["files"]
	if len(fileHeaders) == 0 {
		c.JSON(400, gin.H{"error": "no files provided"})
		return
	}

	// 5. 保存文件
	var uploadedFiles []map[string]any
	for _, fh := range fileHeaders {
		dst := filepath.Join(uploadDir, fh.Filename)
		src, err := fh.Open()
		if err != nil {
			c.JSON(500, gin.H{"error": "failed to open uploaded file: " + err.Error()})
			return
		}
		out, err := os.Create(dst)
		if err != nil {
			src.Close()
			c.JSON(500, gin.H{"error": "failed to create destination file: " + err.Error()})
			return
		}
		if _, err := io.Copy(out, src); err != nil {
			src.Close()
			out.Close()
			c.JSON(500, gin.H{"error": "failed to save file: " + err.Error()})
			return
		}
		src.Close()
		out.Close()

		uploadedFiles = append(uploadedFiles, map[string]any{
			"name":         fh.Filename,
			"size":         fh.Size,
			"type":         fh.Header.Get("Content-Type"),
			"virtual_path": fmt.Sprintf("/mnt/uploads/%s/%s", sessionID, fh.Filename),
		})
	}

	c.JSON(200, gin.H{
		"files": uploadedFiles,
		"count": len(uploadedFiles),
	})
}

// ServeReports serves HTML report files
// GET /api/xiaoqinglong/agent-frame/v1/runner/reports/:sessionID/:filename
func (h *Handler) ServeReports(c *gin.Context) {
	sessionID := c.Param("sessionID")
	filename := c.Param("filename")

	if sessionID == "" || filename == "" {
		c.JSON(400, gin.H{"error": "session_id and filename are required"})
		return
	}

	// 安全检查：只允许 alphanumeric、-、_、. 字符
	if !isSafeFilename(filename) {
		c.JSON(400, gin.H{"error": "invalid filename"})
		return
	}

	// 构建报告文件路径: {uploadsDir}/{sessionID}/reports/{filename}
	reportsDir := filepath.Join(xqldir.GetUploadsDir(), sessionID, "reports")
	filePath := filepath.Join(reportsDir, filename)

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(404, gin.H{"error": "report not found"})
		return
	}

	// 设置缓存头
	c.Header("Cache-Control", "no-cache")
	c.Header("Content-Type", "text/html; charset=utf-8")

	// 直接发送文件内容
	c.File(filePath)
}

// isSafeFilename 检查文件名是否安全（防止路径遍历攻击）
func isSafeFilename(filename string) bool {
	for _, c := range filename {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == '/') {
			return false
		}
	}
	return true && len(filename) < 256 && !containsPathTraversal(filename)
}

// containsPathTraversal 检查是否包含路径遍历
func containsPathTraversal(filename string) bool {
	return filepath.Clean(filename) != filename
}
