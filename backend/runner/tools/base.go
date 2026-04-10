package tools

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ValidationResult 验证结果
type ValidationResult struct {
	Valid     bool
	Message   string
	ErrorCode int
}

// BaseTool 基础工具接口
type BaseTool interface {
	Info(ctx context.Context) (*schema.ToolInfo, error)
	ValidateInput(ctx context.Context, input string) *ValidationResult
	InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error)
}

// OptionalValidateTool 可选验证接口（简化工具可不实现）
type OptionalValidateTool interface {
	ValidateInput(ctx context.Context, input string) *ValidationResult
}

// Adapter 把 BaseTool 适配为 eino tool
type Adapter struct {
	tool BaseTool
}

func (a *Adapter) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return a.tool.Info(ctx)
}

func (a *Adapter) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	if v, ok := a.tool.(OptionalValidateTool); ok {
		if result := v.ValidateInput(ctx, input); !result.Valid {
			return "", &ValidationError{Message: result.Message, Code: result.ErrorCode}
		}
	}
	return a.tool.InvokableRun(ctx, input, opts...)
}

// ValidationError 验证错误
type ValidationError struct {
	Message string
	Code    int
}

func (e *ValidationError) Error() string {
	return e.Message
}
