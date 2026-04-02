package runner

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
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
	uploadDir := os.Getenv("APP_DATA")
	if uploadDir == "" {
		uploadDir = "/tmp/xiaoqinglong/data"
	}
	uploadDir = filepath.Join(uploadDir, "uploads", sessionID)

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
