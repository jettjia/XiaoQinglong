package plugins

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/jettjia/XiaoQinglong/runner/pkg/logger"
)

// ========== Validation Types ==========

// ToolValidationResult 验证结果
type ToolValidationResult struct {
	Valid     bool
	Message   string // 验证失败时的错误信息
	ErrorCode int    // 错误码，0 表示无错误
}

// NewSuccessValidation 创建一个成功的验证结果
func NewSuccessValidation() *ToolValidationResult {
	return &ToolValidationResult{Valid: true, ErrorCode: 0}
}

// NewFailedValidation 创建一个失败的验证结果
func NewFailedValidation(message string, errorCode int) *ToolValidationResult {
	return &ToolValidationResult{Valid: false, Message: message, ErrorCode: errorCode}
}

// ========== Permission Types ==========

// PermissionDecision 权限决定
type PermissionDecision struct {
	Allowed     bool
	Message     string                 // 拒绝原因
	Suggestions []PermissionSuggestion // 建议
}

// PermissionSuggestion 权限建议
type PermissionSuggestion struct {
	Type        string // "addRules", "continue"
	Rules       []PermissionRule
	Behavior    string // "allow", "deny"
	Destination string // "localSettings", "global"
}

// PermissionRule 权限规则
type PermissionRule struct {
	ToolName    string
	RuleContent string
}

// ========== Tool Context Modifier ==========

// ToolContextModifier 工具上下文修改器
// Tool 执行后可以返回此函数来修改后续执行的上下文
type ToolContextModifier func(ctx *ToolExecutionContext) *ToolExecutionContext

// ToolExecutionContext 工具执行上下文
type ToolExecutionContext struct {
	AllowedTools  []string          // 临时添加的允许工具列表
	ModelOverride string            // 模型覆盖
	EffortLevel   string            // effort 级别
	Variables     map[string]string // 额外变量
}

// NewToolExecutionContext 创建一个新的工具执行上下文
func NewToolExecutionContext() *ToolExecutionContext {
	return &ToolExecutionContext{
		AllowedTools: make([]string, 0),
		Variables:    make(map[string]string),
	}
}

// Merge 合并另一个上下文
func (c *ToolExecutionContext) Merge(other *ToolExecutionContext) *ToolExecutionContext {
	if other == nil {
		return c
	}
	result := &ToolExecutionContext{
		AllowedTools:  make([]string, len(c.AllowedTools)+len(other.AllowedTools)),
		ModelOverride: other.ModelOverride,
		EffortLevel:   other.EffortLevel,
		Variables:     make(map[string]string),
	}
	// 合并 AllowedTools
	copy(result.AllowedTools, c.AllowedTools)
	result.AllowedTools = append(result.AllowedTools, other.AllowedTools...)
	// 覆盖
	if other.ModelOverride != "" {
		result.ModelOverride = other.ModelOverride
	}
	if other.EffortLevel != "" {
		result.EffortLevel = other.EffortLevel
	}
	// 合并 Variables
	for k, v := range c.Variables {
		result.Variables[k] = v
	}
	for k, v := range other.Variables {
		result.Variables[k] = v
	}
	return result
}

// ========== Tool Result with Context Modifier ==========

// ToolResult 工具执行结果，包含可选的上下文修改器
type ToolResult struct {
	Output          string
	ContextModifier ToolContextModifier
	NewMessages     []interface{} // 可选的新消息
}

// ========== Tool Validator Interface ==========

// ToolValidator 输入验证器接口
// 工具可以实现此接口来添加自定义验证逻辑
type ToolValidator interface {
	// ValidateInput 验证输入参数
	// 返回 nil 表示验证通过
	// 返回 error 表示验证失败，error 的 message 会被提取为用户友好的错误信息
	ValidateInput(ctx context.Context, argumentsInJSON string) *ToolValidationResult
}

// DefaultToolValidator 默认验证器
// 对所有工具默认放行
type DefaultToolValidator struct{}

func (v *DefaultToolValidator) ValidateInput(ctx context.Context, argumentsInJSON string) *ToolValidationResult {
	return NewSuccessValidation()
}

// ========== Tool Permission Checker Interface ==========

// ToolPermissionChecker 权限检查器接口
// 工具可以实现此接口来添加自定义权限检查
type ToolPermissionChecker interface {
	// CheckPermissions 检查执行权限
	// 返回 PermissionDecision{Allowed: true} 表示允许执行
	// 返回 PermissionDecision{Allowed: false, Message: "..."} 表示拒绝执行
	CheckPermissions(ctx context.Context, argumentsInJSON string) *PermissionDecision
}

// DefaultToolPermissionChecker 默认权限检查器
// 对所有工具默认允许
type DefaultToolPermissionChecker struct{}

func (p *DefaultToolPermissionChecker) CheckPermissions(ctx context.Context, argumentsInJSON string) *PermissionDecision {
	return &PermissionDecision{Allowed: true}
}

// ========== Tool with Validation and Permissions ==========

// ValidatableTool 包装工具，添加验证和权限检查
type ValidatableTool struct {
	tool.InvokableTool
	validator   ToolValidator
	permissions ToolPermissionChecker
}

// NewValidatableTool 创建一个带验证和权限的工具包装器
// baseTool 必须实现 tool.InvokableTool 接口
func NewValidatableTool(baseTool tool.InvokableTool, validator ToolValidator, permissions ToolPermissionChecker) *ValidatableTool {
	if validator == nil {
		validator = &DefaultToolValidator{}
	}
	if permissions == nil {
		permissions = &DefaultToolPermissionChecker{}
	}
	return &ValidatableTool{
		InvokableTool: baseTool,
		validator:     validator,
		permissions:   permissions,
	}
}

// ValidateInput 验证输入
func (t *ValidatableTool) ValidateInput(ctx context.Context, argumentsInJSON string) *ToolValidationResult {
	return t.validator.ValidateInput(ctx, argumentsInJSON)
}

// CheckPermissions 检查权限
func (t *ValidatableTool) CheckPermissions(ctx context.Context, argumentsInJSON string) *PermissionDecision {
	return t.permissions.CheckPermissions(ctx, argumentsInJSON)
}

// InvokableRun 执行验证和权限检查，然后执行工具
func (t *ValidatableTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	logger.GetRunnerLogger().Infof("[ValidatableTool] Validating input for tool invocation")

	// 1. 验证输入
	result := t.validator.ValidateInput(ctx, argumentsInJSON)
	if !result.Valid {
		logger.GetRunnerLogger().Infof("[ValidatableTool] >>> VALIDATION FAILED: %s (code: %d)", result.Message, result.ErrorCode)
		return "", fmt.Errorf("input validation failed: %s (code: %d)", result.Message, result.ErrorCode)
	}
	logger.GetRunnerLogger().Infof("[ValidatableTool] >>> Input validated successfully")

	// 2. 检查权限
	decision := t.permissions.CheckPermissions(ctx, argumentsInJSON)
	if !decision.Allowed {
		logger.GetRunnerLogger().Infof("[ValidatableTool] >>> PERMISSION DENIED: %s", decision.Message)
		return "", fmt.Errorf("permission denied: %s", decision.Message)
	}
	logger.GetRunnerLogger().Infof("[ValidatableTool] >>> Permission granted")

	// 3. 执行工具
	logger.GetRunnerLogger().Infof("[ValidatableTool] Executing tool...")
	return t.InvokableTool.InvokableRun(ctx, argumentsInJSON, opts...)
}

// ========== Deferred Tool (Lazy Loading) ==========

// ToolLoader 工具加载器函数类型
type ToolLoader func() (tool.BaseTool, error)

// DeferredTool 延迟加载的工具
// 适用于某些工具不需要在启动时加载，而是在首次使用时才加载
type DeferredTool struct {
	name        string
	loader      ToolLoader
	loadedTool  tool.BaseTool
	shouldDefer bool
}

// NewDeferredTool 创建一个延迟加载的工具
func NewDeferredTool(name string, loader ToolLoader) *DeferredTool {
	return &DeferredTool{
		name:        name,
		loader:      loader,
		loadedTool:  nil,
		shouldDefer: true,
	}
}

// IsLoaded 检查工具是否已加载
func (t *DeferredTool) IsLoaded() bool {
	return t.loadedTool != nil
}

// Load 加载工具
func (t *DeferredTool) Load() (tool.BaseTool, error) {
	if t.loadedTool != nil {
		return t.loadedTool, nil
	}
	logger.GetRunnerLogger().Infof("[DeferredTool] >>> LAZY LOADING tool: %s (first time use)", t.name)
	loaded, err := t.loader()
	if err != nil {
		return nil, fmt.Errorf("failed to load tool %s: %w", t.name, err)
	}
	t.loadedTool = loaded
	logger.GetRunnerLogger().Infof("[DeferredTool] >>> Tool %s loaded successfully", t.name)
	return t.loadedTool, nil
}

// MustLoad 加载工具，如果失败则 panic
func (t *DeferredTool) MustLoad() tool.BaseTool {
	tool, err := t.Load()
	if err != nil {
		panic(err)
	}
	return tool
}

// Info 获取工具信息（如果已加载）
func (t *DeferredTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	if t.loadedTool == nil {
		return nil, fmt.Errorf("tool %s not loaded yet", t.name)
	}
	return t.loadedTool.Info(ctx)
}

// ShouldDefer 返回是否应该延迟加载
func (t *DeferredTool) ShouldDefer() bool {
	return t.shouldDefer
}

// GetLoadedTool 返回已加载的工具（如果没有加载则返回 nil）
func (t *DeferredTool) GetLoadedTool() tool.BaseTool {
	return t.loadedTool
}

// ========== HTTP Tool with Validation ==========

// HTTPToolValidator HTTP 工具的验证器
type HTTPToolValidator struct {
	endpoint string
	method   string
}

func NewHTTPToolValidator(endpoint, method string) *HTTPToolValidator {
	return &HTTPToolValidator{endpoint: endpoint, method: method}
}

func (v *HTTPToolValidator) ValidateInput(ctx context.Context, argumentsInJSON string) *ToolValidationResult {
	if argumentsInJSON == "" {
		// 空参数可能是合法的，取决于工具定义
		return NewSuccessValidation()
	}

	// 尝试解析 JSON
	var inputMap map[string]any
	if err := json.Unmarshal([]byte(argumentsInJSON), &inputMap); err != nil {
		return NewFailedValidation(fmt.Sprintf("invalid JSON format: %v", err), 1)
	}

	// 验证必要字段（根据 HTTP 方法）
	if v.method == "POST" || v.method == "PUT" || v.method == "PATCH" {
		// POST 类方法通常需要 body
		if _, ok := inputMap["body"]; !ok {
			// body 可能不是必需的，这里简化处理
		}
	}

	return NewSuccessValidation()
}

// ========== Helper function to wrap tools with validation ==========

// WrapToolWithValidation 包装工具添加验证和权限检查
func WrapToolWithValidation(baseTool tool.BaseTool, validator ToolValidator, permissions ToolPermissionChecker) tool.BaseTool {
	invokable, ok := baseTool.(tool.InvokableTool)
	if !ok {
		return baseTool
	}
	return NewValidatableTool(invokable, validator, permissions)
}
