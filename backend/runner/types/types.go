package types

import (
	"encoding/json"

	"github.com/jettjia/XiaoQinglong/runner/subagent"
)

// ========== Shared Types ==========

// Skill 技能配置
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

// MCPConfig MCP 服务配置
type MCPConfig struct {
	Name      string            `json:"name"`
	Transport string            `json:"transport"` // "stdio" 或 "http"
	Command   string            `json:"command"`   // stdio 模式: 启动命令
	Args      []string          `json:"args"`      // stdio 模式: 命令参数
	Env       map[string]string `json:"env"`       // stdio 模式: 环境变量
	Endpoint  string            `json:"endpoint"`   // http 模式: MCP 服务地址
	Headers   map[string]string `json:"headers"`   // http 模式: 请求头
	RiskLevel string            `json:"risk_level"`
}

// CLIConfig CLI 工具配置（用于扩展第三方 CLI 如飞书 CLI）
type CLIConfig struct {
	Name      string `json:"name"`       // CLI 名称，如 "lark"
	Command   string `json:"command"`    // CLI 命令，如 "lark-cli"
	ConfigDir string `json:"config_dir"` // Token 配置目录
	SkillsDir string `json:"skills_dir"` // Skills 文件目录
	RiskLevel string `json:"risk_level"` // 风险级别
	AuthType  string `json:"auth_type"`  // 授权类型：none, oauth2_device
}

// A2AAgentConfig A2A Agent 配置
type A2AAgentConfig struct {
	Name      string            `json:"name"`
	Endpoint  string            `json:"endpoint"`
	Headers   map[string]string `json:"headers"`
	RiskLevel string            `json:"risk_level"`
}

// ToolConfig 工具配置
type ToolConfig struct {
	Type        string            `json:"type"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Endpoint    string            `json:"endpoint"`
	Method      string            `json:"method"`
	Headers     map[string]string `json:"headers"`
	RiskLevel   string            `json:"risk_level"`
}

// InternalAgentConfig 内部 Agent 配置
type InternalAgentConfig struct {
	ID     string      `json:"id"`
	Name   string      `json:"name"`
	Prompt string      `json:"prompt"`
	Model  ModelConfig `json:"model"`
}

// ModelConfig 模型配置
type ModelConfig struct {
	Provider    string  `json:"provider"`
	Name        string  `json:"name"`
	APIKey      string  `json:"api_key"`
	APIBase     string  `json:"api_base"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
	TopP        float64 `json:"top_p"`
}

// RunOptions 运行选项
type RunOptions struct {
	Temperature    float64               `json:"temperature"`
	MaxTokens      int                   `json:"max_tokens"`
	Stream         bool                  `json:"stream"`
	TopP           float64               `json:"top_p"`
	Stop           []string              `json:"stop"`
	TimeoutMs      int                   `json:"timeout_ms"`
	MaxIterations  int                   `json:"max_iterations"`
	MaxToolCalls   int                   `json:"max_tool_calls"`
	MaxA2ACalls    int                   `json:"max_a2a_calls"`
	MaxTotalTokens int                   `json:"max_total_tokens"`
	Retry          *RetryConfig          `json:"retry"`
	ResponseSchema *ResponseSchemaConfig `json:"response_schema"`
	Routing        *RoutingConfig        `json:"routing"`
	ApprovalPolicy *ApprovalPolicy       `json:"approval_policy"`
	CheckPointID   string                `json:"checkpoint_id"`
}

// ApprovalPolicy 审批策略
type ApprovalPolicy struct {
	Enabled       bool     `json:"enabled"`
	RiskThreshold string   `json:"risk_threshold"` // low, medium, high
	AutoApprove   []string `json:"auto_approve"`   // 白名单，tool names
}

// RetryConfig 重试配置
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
	ModelRoleDefault   ModelRole = "default"   // 默认模型，用于主对话
	ModelRoleRewrite   ModelRole = "rewrite"   // 改写模型，用于query改写
	ModelRoleSkill     ModelRole = "skill"     // 技能模型，用于skill执行
	ModelRoleSummarize ModelRole = "summarize" // 总结模型，用于内容总结
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
type ResponseSchemaConfig struct {
	Type     string         `json:"type"`
	Version  string         `json:"version"`
	Strict   bool           `json:"strict"`
	Schema   map[string]any `json:"schema"`
	Fallback string         `json:"fallback"`
}

// SandboxConfig 沙箱配置
type SandboxConfig struct {
	Enabled   bool              `json:"enabled"`
	Mode      string            `json:"mode"`
	Image     string            `json:"image"`
	Workdir   string            `json:"workdir"`
	Network   string            `json:"network"`
	TimeoutMs int               `json:"timeout_ms"`
	Env       map[string]string `json:"env"`
	Limits    *SandboxLimits    `json:"limits"`
	Volumes   []VolumeMount     `json:"volumes"` // 额外挂载的卷
}

// VolumeMount 卷挂载配置
type VolumeMount struct {
	HostPath      string `json:"host_path"`      // 宿主机路径
	ContainerPath string `json:"container_path"` // 容器内路径
	ReadOnly      bool   `json:"read_only"`      // 是否只读
}

// SandboxLimits 沙箱资源限制
type SandboxLimits struct {
	CPU    string `json:"cpu"`    // e.g., "0.5" (0.5 cores), "2" (2 cores)
	Memory string `json:"memory"` // e.g., "512m", "1g", "256M"
}

// FileConfig 文件配置
type FileConfig struct {
	Name        string `json:"name"`
	VirtualPath string `json:"virtual_path"` // 虚拟路径，如 /mnt/uploads/session_id/file.md
	Size        int64  `json:"size"`
	Type        string `json:"type"` // mime type
}

// Message 消息
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// RunRequest 运行请求
type RunRequest struct {
	Prompt         string                    `json:"prompt"`
	Models         map[string]ModelConfig    `json:"models"`
	Messages       []Message                 `json:"messages"`
	Context        map[string]any            `json:"context"`
	KnowledgeBases []KnowledgeBaseConfig     `json:"knowledge_bases"` // 知识库配置（用于运行时检索）
	Skills         []Skill                   `json:"skills"`
	MCPs           []MCPConfig               `json:"mcps"`
	CLIs           []CLIConfig               `json:"clis"` // CLI 工具配置（如飞书 CLI）
	A2A            []A2AAgentConfig          `json:"a2a"`
	Tools          []ToolConfig              `json:"tools"`
	InternalAgents []InternalAgentConfig     `json:"internal_agents"`
	SubAgents      []subagent.SubAgentConfig `json:"sub_agents"` // Sub-Agent 配置列表
	Options        *RunOptions               `json:"options"`
	Sandbox        *SandboxConfig            `json:"sandbox"`
	Files          []FileConfig             `json:"files"` // 上传的文件列表
}

// KnowledgeBaseConfig 知识库配置
type KnowledgeBaseConfig struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	RetrievalURL string `json:"retrieval_url"`
	Token        string `json:"token"`
	TopK         int    `json:"top_k"`
}

// ToolCall 工具调用
type ToolCall struct {
	Tool   string `json:"tool"`
	Input  any    `json:"input"`
	Output any    `json:"output"`
}

// A2AResult A2A 结果
type A2AResult struct {
	AgentName string `json:"agent_name"`
	Status    string `json:"status"`
	Result    any    `json:"result,omitempty"`
	Error     string `json:"error,omitempty"`
}

// PendingApproval 待审批信息
type PendingApproval struct {
	InterruptID   string `json:"interrupt_id"`
	ToolName      string `json:"tool_name"`
	ToolType      string `json:"tool_type"`
	ArgumentsJSON string `json:"arguments_json"`
	RiskLevel     string `json:"risk_level"`
	Description   string `json:"description"`
}

// ResponseMetadata 响应元数据
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

// ToolCallMetadata 工具调用元数据
type ToolCallMetadata struct {
	Tool      string `json:"tool"`
	Input     any    `json:"input"`
	Output    any    `json:"output"`
	LatencyMs int64  `json:"latency_ms"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

// ResumeRequest resume 请求
type ResumeRequest struct {
	CheckPointID string           `json:"checkpoint_id"`
	Approvals    []ResumeApproval `json:"approvals"`
}

// ResumeApproval 单个审批结果
type ResumeApproval struct {
	InterruptID      string  `json:"interrupt_id"`
	Approved         bool    `json:"approved"`
	DisapproveReason *string `json:"disapprove_reason,omitempty"`
}

// ResumeResponse resume 响应
type ResumeResponse struct {
	Success      bool             `json:"success"`
	Error        string           `json:"error,omitempty"`
	FinishReason string           `json:"finish_reason"`
	Content      string           `json:"content,omitempty"`
	ToolCalls    []ToolCall       `json:"tool_calls,omitempty"`
	Metadata     ResponseMetadata `json:"metadata"`
}

// ApprovalInfo 审批信息，传递给前端
type ApprovalInfo struct {
	ToolName        string `json:"tool_name"`
	ToolType       string `json:"tool_type"` // http, mcp, skill, a2a
	ArgumentsInJSON string `json:"arguments_in_json"`
	RiskLevel      string `json:"risk_level"`
	Description    string `json:"description"`
}

// ApprovalResult 审批结果
type ApprovalResult struct {
	Approved         bool
	DisapproveReason *string
}

// RunResponse 运行响应
type RunResponse struct {
	Content          string            `json:"content,omitempty"`
	ToolCalls        []ToolCall        `json:"tool_calls,omitempty"`
	A2AResults       []A2AResult       `json:"a2a_results,omitempty"`
	TokensUsed       int               `json:"tokens_used,omitempty"`
	FinishReason     string            `json:"finish_reason"`
	Metadata         ResponseMetadata  `json:"metadata"`
	A2UIMessages     []json.RawMessage `json:"a2ui_messages,omitempty"`
	PendingApprovals []PendingApproval `json:"pending_approvals,omitempty"`
	CheckPointID     string            `json:"checkpoint_id,omitempty"`
}