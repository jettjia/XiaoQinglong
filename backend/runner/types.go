package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/compose"
)

// ========== Global Checkpoint Store Manager ==========

var (
	checkpointStores = make(map[string]compose.CheckPointStore)
	checkpointMu     sync.RWMutex
	// runners 存储活跃的 runner 实例，用于 resume
	runners   = make(map[string]*adkRunner)
	runnersMu sync.RWMutex
)

// adkRunner 包装 adk.Runner 和相关信息
type adkRunner struct {
	runner   *adk.Runner
	Messages []adk.Message
}

// GetCheckPointStore 获取指定 ID 的 checkpoint store
func GetCheckPointStore(id string) compose.CheckPointStore {
	checkpointMu.RLock()
	defer checkpointMu.RUnlock()
	return checkpointStores[id]
}

// SetCheckPointStore 存储指定 ID 的 checkpoint store
func SetCheckPointStore(id string, store compose.CheckPointStore) {
	checkpointMu.Lock()
	defer checkpointMu.Unlock()
	checkpointStores[id] = store
}

// GetRunner 获取指定 checkpoint ID 的 runner
func GetRunner(id string) *adkRunner {
	runnersMu.RLock()
	defer runnersMu.RUnlock()
	return runners[id]
}

// SetRunner 存储指定 checkpoint ID 的 runner
func SetRunner(id string, r *adkRunner) {
	runnersMu.Lock()
	defer runnersMu.Unlock()
	runners[id] = r
}

// ========== Request Types ==========

type RunRequest struct {
	Prompt         string                 `json:"prompt"`
	Models         map[string]ModelConfig `json:"models"`
	Messages       []Message              `json:"messages"`
	Context        map[string]any          `json:"context"`
	Knowledge      []KnowledgeItem        `json:"knowledge"`
	Skills         []Skill                `json:"skills"`
	MCPs           []MCPConfig            `json:"mcps"`
	A2A            []A2AAgentConfig       `json:"a2a"`
	Tools          []ToolConfig           `json:"tools"`
	InternalAgents []InternalAgentConfig  `json:"internal_agents"`
	Options        *RunOptions            `json:"options"`
	Sandbox        *SandboxConfig         `json:"sandbox"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ModelConfig struct {
	Provider    string  `json:"provider"`
	Name        string  `json:"name"`
	APIKey      string  `json:"api_key"`
	APIBase     string  `json:"api_base"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
	TopP        float64 `json:"top_p"`
}

type KnowledgeItem struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Content  string         `json:"content"`
	Score    float64        `json:"score"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type Skill struct {
	ID string `json:"id"`
	// 以下字段由 SKILL.md 定义，runner 自动加载
	Name        string   `json:",omitempty"`
	Description string   `json:",omitempty"`
	Instruction string   `json:",omitempty"`
	Scope       string   `json:",omitempty"`
	Trigger     string   `json:",omitempty"`
	EntryScript string   `json:",omitempty"`
	FilePath    string   `json:",omitempty"`
	Inputs      []string `json:",omitempty"`
	Outputs     []string `json:",omitempty"`
	RiskLevel   string   `json:"risk_level,omitempty"`
}

type MCPConfig struct {
	Name      string            `json:"name"`
	Transport string            `json:"transport"` // "stdio" 或 "http"
	Command   string            `json:"command"`   // stdio 模式: 启动命令
	Args      []string          `json:"args"`      // stdio 模式: 命令参数
	Env       map[string]string `json:"env"`       // stdio 模式: 环境变量
	Endpoint  string            `json:"endpoint"`  // http 模式: MCP 服务地址
	Headers   map[string]string `json:"headers"`   // http 模式: 请求头
	RiskLevel string            `json:"risk_level"`
}

type A2AAgentConfig struct {
	Name      string            `json:"name"`
	Endpoint  string            `json:"endpoint"`
	Headers   map[string]string `json:"headers"`
	RiskLevel string            `json:"risk_level"`
}

type ToolConfig struct {
	Type        string            `json:"type"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Endpoint    string            `json:"endpoint"`
	Method      string            `json:"method"`
	Headers     map[string]string `json:"headers"`
	RiskLevel   string            `json:"risk_level"`
}

type InternalAgentConfig struct {
	ID     string      `json:"id"`
	Name   string      `json:"name"`
	Prompt string      `json:"prompt"`
	Model  ModelConfig `json:"model"`
}

type RunOptions struct {
	Temperature     float64               `json:"temperature"`
	MaxTokens       int                   `json:"max_tokens"`
	Stream          bool                  `json:"stream"`
	TopP            float64               `json:"top_p"`
	Stop            []string              `json:"stop"`
	TimeoutMs       int                   `json:"timeout_ms"`
	MaxIterations   int                   `json:"max_iterations"`
	MaxToolCalls    int                   `json:"max_tool_calls"`
	MaxA2ACalls     int                   `json:"max_a2a_calls"`
	MaxTotalTokens  int                   `json:"max_total_tokens"`
	Retry           *RetryConfig          `json:"retry"`
	ResponseSchema  *ResponseSchemaConfig `json:"response_schema"`
	Routing         *RoutingConfig        `json:"routing"`
	ApprovalPolicy  *ApprovalPolicy      `json:"approval_policy"`
	CheckPointID   string               `json:"checkpoint_id"`
}

// ApprovalPolicy 审批策略
type ApprovalPolicy struct {
	Enabled        bool     `json:"enabled"`
	RiskThreshold string   `json:"risk_threshold"` // low, medium, high
	AutoApprove   []string `json:"auto_approve"`  // 白名单，tool names
}

type RetryConfig struct {
	MaxAttempts       int      `json:"max_attempts"`
	InitialDelayMs    int      `json:"initial_delay_ms"`
	MaxDelayMs        int      `json:"max_delay_ms"`
	BackoffMultiplier float64  `json:"backoff_multiplier"`
	RetryableErrors   []string `json:"retryable_errors"`
}

// ModelRole 模型角色，用于多模型路由
type ModelRole string

const (
	ModelRoleDefault    ModelRole = "default"    // 默认模型，用于主对话
	ModelRoleRewrite    ModelRole = "rewrite"    // 改写模型，用于query改写
	ModelRoleSkill      ModelRole = "skill"      // 技能模型，用于skill执行
	ModelRoleSummarize  ModelRole = "summarize"  // 总结模型，用于内容总结
)

// RoutingConfig 多模型路由配置
type RoutingConfig struct {
	// DefaultModel 默认使用的模型角色
	DefaultModel ModelRole `json:"default_model"`
	// RewritePrompt 改写使用的提示词模板
	RewritePrompt string `json:"rewrite_prompt"`
	// SummarizePrompt 总结使用的提示词模板
	SummarizePrompt string `json:"summarize_prompt"`
}

// ResponseSchemaConfig 响应格式配置
// 支持的响应类型 (type):
//   - text: 纯文本
//   - markdown: Markdown 格式
//   - a2ui: A2UI 结构化格式（通过 schema 定义组件）
//   - json: JSON 格式（通过 schema 定义结构）
//   - image: 图片 (url 或 base64)
//   - audio: 音频 (url 或 base64)
//   - video: 视频 (url 或 base64)
//   - multipart: 多格式混合
type ResponseSchemaConfig struct {
	Type     string         `json:"type"`
	Version  string         `json:"version"`
	Strict   bool           `json:"strict"`
	Schema   map[string]any `json:"schema"`
	Fallback string         `json:"fallback"`
}

type SandboxConfig struct {
	Enabled   bool              `json:"enabled"`
	Mode      string            `json:"mode"`
	Image     string            `json:"image"`
	Workdir   string            `json:"workdir"`
	Network   string            `json:"network"`
	TimeoutMs int               `json:"timeout_ms"`
	Env       map[string]string `json:"env"`
	Limits    *SandboxLimits    `json:"limits"`
}

// SandboxLimits 沙箱资源限制
type SandboxLimits struct {
	CPU    string `json:"cpu"`    // e.g., "0.5" (0.5 cores), "2" (2 cores)
	Memory string `json:"memory"` // e.g., "512m", "1g", "256M"
}

// ========== Response Types ==========

type RunResponse struct {
	Content          string             `json:"content,omitempty"`
	ToolCalls        []ToolCall         `json:"tool_calls,omitempty"`
	A2AResults      []A2AResult        `json:"a2a_results,omitempty"`
	TokensUsed       int                `json:"tokens_used,omitempty"`
	FinishReason     string             `json:"finish_reason"`
	Metadata         ResponseMetadata   `json:"metadata"`
	A2UIMessages     []json.RawMessage  `json:"a2ui_messages,omitempty"`
	PendingApprovals []PendingApproval  `json:"pending_approvals,omitempty"`
	CheckPointID     string             `json:"checkpoint_id,omitempty"`
}

// PendingApproval 待审批信息
type PendingApproval struct {
	InterruptID     string `json:"interrupt_id"`
	ToolName      string `json:"tool_name"`
	ToolType      string `json:"tool_type"`
	ArgumentsJSON string `json:"arguments_json"`
	RiskLevel     string `json:"risk_level"`
	Description   string `json:"description"`
}

type ToolCall struct {
	Tool   string `json:"tool"`
	Input  any    `json:"input"`
	Output any    `json:"output"`
}

// ResumeRequest resume 请求
type ResumeRequest struct {
	CheckPointID string                `json:"checkpoint_id"`
	Approvals    []ResumeApproval       `json:"approvals"`
}

// ResumeApproval 单个审批结果
type ResumeApproval struct {
	InterruptID      string  `json:"interrupt_id"`
	Approved         bool    `json:"approved"`
	DisapproveReason *string `json:"disapprove_reason,omitempty"`
}

// ResumeResponse resume 响应
type ResumeResponse struct {
	Success      bool     `json:"success"`
	Error        string   `json:"error,omitempty"`
	FinishReason string   `json:"finish_reason"`
	Content      string   `json:"content,omitempty"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	Metadata     ResponseMetadata `json:"metadata"`
}

type A2AResult struct {
	AgentName string `json:"agent_name"`
	Status    string `json:"status"`
	Result    any    `json:"result,omitempty"`
	Error     string `json:"error,omitempty"`
}

type ResponseMetadata struct {
	Model            string             `json:"model"`
	LatencyMs        int64              `json:"latency_ms"`
	TokensUsed       int                `json:"tokens_used,omitempty"`
	PromptTokens     int                `json:"prompt_tokens,omitempty"`
	CompletionTokens int                `json:"completion_tokens,omitempty"`
	ToolCallsCount   int                `json:"tool_calls_count,omitempty"`
	A2ACallsCount    int                `json:"a2a_calls_count,omitempty"`
	SkillCallsCount  int                `json:"skill_calls_count,omitempty"`
	Iterations       int                `json:"iterations,omitempty"`
	ToolCallsDetail  []ToolCallMetadata `json:"tool_calls_detail,omitempty"`
	Error            string             `json:"error,omitempty"`
}

// ========== Runner ==========

type Runner struct {
	request    *RunRequest
	dispatcher *Dispatcher
}

func NewRunner(req *RunRequest) *Runner {
	return &Runner{
		request:    req,
		dispatcher: NewDispatcher(req),
	}
}

func (r *Runner) Run(ctx context.Context) (*RunResponse, error) {
	startTime := time.Now()

	// 执行调度器运行
	result, err := r.dispatcher.Run(ctx)
	if err != nil {
		return nil, fmt.Errorf("dispatcher run failed: %w", err)
	}

	latencyMs := time.Since(startTime).Milliseconds()

	// 构建响应元数据
	metadata := ResponseMetadata{
		Model:     r.getDefaultModelName(),
		LatencyMs: latencyMs,
	}

	// 如果有更详细的 metadata，则合并
	if result.Metadata != nil {
		metadata.TokensUsed = result.TokensUsed
		metadata.PromptTokens = result.Metadata.PromptTokens
		metadata.CompletionTokens = result.Metadata.CompletionTokens
		metadata.ToolCallsCount = result.Metadata.ToolCallsCount
		metadata.A2ACallsCount = result.Metadata.A2ACallsCount
		metadata.SkillCallsCount = result.Metadata.SkillCallsCount
		metadata.Iterations = result.Metadata.Iterations
		metadata.ToolCallsDetail = result.Metadata.ToolCallsDetail
		if result.Metadata.Error != "" {
			metadata.Error = result.Metadata.Error
		}
	}

	resp := &RunResponse{
		Content:          result.Content,
		ToolCalls:        result.ToolCalls,
		A2AResults:       result.A2AResults,
		TokensUsed:       result.TokensUsed,
		FinishReason:     result.FinishReason,
		Metadata:         metadata,
		A2UIMessages:     result.A2UIMessages,
		PendingApprovals: result.PendingApprovals,
		CheckPointID:     result.CheckPointID,
	}

	return resp, nil
}

// RunStream 流式运行 Agent
func (r *Runner) RunStream(ctx context.Context) (<-chan StreamEvent, error) {
	return r.dispatcher.RunStream(ctx)
}

func (r *Runner) getDefaultModelName() string {
	if r.request.Models == nil {
		return ""
	}
	cfg, ok := r.request.Models["default"]
	if !ok {
		return ""
	}
	return cfg.Name
}

// Resume 恢复中断的 agent 执行
func (r *Runner) Resume(ctx context.Context, req *ResumeRequest) (*ResumeResponse, error) {
	// 获取存储的 runner
	adkRunner := GetRunner(req.CheckPointID)
	if adkRunner == nil {
		return &ResumeResponse{
			Success: false,
			Error:   "checkpoint not found or expired",
		}, nil
	}

	// 构建 approval results
	approvals := make(map[string]any)
	for _, approval := range req.Approvals {
		result := &ApprovalResult{
			Approved:         approval.Approved,
			DisapproveReason: approval.DisapproveReason,
		}
		approvals[approval.InterruptID] = result
	}

	// 恢复执行
	events, err := adkRunner.runner.ResumeWithParams(ctx, req.CheckPointID, &adk.ResumeParams{
		Targets: approvals,
	})
	if err != nil {
		return &ResumeResponse{
			Success: false,
			Error:   fmt.Sprintf("resume failed: %v", err),
		}, nil
	}

	// 处理事件
	var finalContent string
	var toolCalls []ToolCall
	var finishReason string
	var toolCallsDetail []ToolCallMetadata
	toolCallCount := 0

	for {
		event, ok := events.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			return &ResumeResponse{
				Success: false,
				Error:   fmt.Sprintf("resume error: %v", event.Err),
			}, nil
		}

		// 处理消息输出和工具调用
		if event.Output != nil {
			if msg, err := event.Output.MessageOutput.GetMessage(); err == nil {
				finalContent = msg.Content

				// 处理工具调用
				for _, tc := range msg.ToolCalls {
					toolCallCount++
					tcMeta := ToolCallMetadata{
						Tool:  tc.Function.Name,
						Input: tc.Function.Arguments,
					}
					toolCallsDetail = append(toolCallsDetail, tcMeta)

					toolCalls = append(toolCalls, ToolCall{
						Tool:   tc.Function.Name,
						Input:  tc.Function.Arguments,
						Output: nil,
					})
				}

				// 检查是否完成
				if len(msg.ToolCalls) == 0 && finishReason == "" {
					finishReason = "completed"
				}
			}
		}
	}

	// 构建元数据
	metadata := ResponseMetadata{
		Model:          r.getDefaultModelName(),
		ToolCallsCount: toolCallCount,
		Iterations:     toolCallCount,
	}

	return &ResumeResponse{
		Success:      true,
		FinishReason: finishReason,
		Content:      finalContent,
		ToolCalls:    toolCalls,
		Metadata:     metadata,
	}, nil
}