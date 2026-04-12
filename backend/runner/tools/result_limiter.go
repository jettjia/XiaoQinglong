package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ResultLimiter 工具结果大小限制器
type ResultLimiter struct {
	mu          sync.Mutex
	tempDir     string
	maxCharSize int
}

// NewResultLimiter 创建结果限制器
// tempDir: 临时文件目录，maxCharSize: 最大字符数限制
func NewResultLimiter(tempDir string, maxCharSize int) *ResultLimiter {
	if tempDir == "" {
		tempDir = os.TempDir()
	}
	return &ResultLimiter{
		tempDir:     tempDir,
		maxCharSize: maxCharSize,
	}
}

// LimitResult 检查并限制结果大小
// 如果结果超过限制，写入临时文件并返回文件路径
func (r *ResultLimiter) LimitResult(result string, toolName string) (string, bool) {
	if r.maxCharSize <= 0 || len(result) <= r.maxCharSize {
		return result, false // 不需要截断
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// 写入临时文件
	filename := fmt.Sprintf("runner_%s_%d.tmp", toolName, os.Getpid())
	filePath := filepath.Join(r.tempDir, filename)

	if err := os.WriteFile(filePath, []byte(result), 0644); err != nil {
		// 写入失败，返回截断结果
		return result[:r.maxCharSize] + "\n\n[Result truncated - failed to save to temp file]", true
	}

	// 返回文件路径
	truncatedMsg := fmt.Sprintf("\n\n[Result too large (%d chars), saved to: %s]\n\n", len(result), filePath)
	return truncatedMsg, true
}

// ToolResultLimiter 接口，用于包装工具执行时限制结果大小
type ToolResultLimiter interface {
	LimitResult(result string, toolName string) (string, bool)
}

// WrapToolWithLimiter 包装工具，添加结果大小限制
func WrapToolWithLimiter(t BaseTool, limiter *ResultLimiter) *LimitedTool {
	return &LimitedTool{
		tool:   t,
		limiter: limiter,
	}
}

// LimitedTool 带结果限制的工具包装器
type LimitedTool struct {
	tool   BaseTool
	limiter *ResultLimiter
}

func (t *LimitedTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return t.tool.Info(ctx)
}

func (t *LimitedTool) ValidateInput(ctx context.Context, input string) *ValidationResult {
	if v, ok := t.tool.(OptionalValidateTool); ok {
		return v.ValidateInput(ctx, input)
	}
	return &ValidationResult{Valid: true}
}

func (t *LimitedTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	result, err := t.tool.InvokableRun(ctx, input, opts...)
	if err != nil {
		return result, err
	}

	// 获取工具名
	info, _ := t.tool.Info(ctx)
	toolName := "unknown"
	if info != nil {
		toolName = info.Name
	}

	// 限制结果大小
	limitedResult, wasLimited := t.limiter.LimitResult(result, toolName)
	if wasLimited {
		return limitedResult, nil
	}
	return result, nil
}

// GetMaxResultCharsFromRegistry 从注册中心获取工具的最大结果限制
func GetMaxResultCharsFromRegistry(toolName string) int {
	return GlobalRegistry.GetMaxResultChars(toolName)
}

// DefaultResultLimiter 默认结果限制器
// 默认限制: 200KB (200 * 1024 chars)
var DefaultResultLimiter = NewResultLimiter("", 200*1024)
