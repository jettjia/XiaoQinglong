package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"time"
)

// ========== 完整类型定义（与 runner/types.go 保持一致）==========

type RunRequest struct {
	Prompt         string                 `json:"prompt"`
	Models         map[string]ModelConfig `json:"models"`
	Messages       []Message              `json:"messages"`
	Context        map[string]any         `json:"context"`
	Knowledge      []KnowledgeItem         `json:"knowledge"`
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
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Instruction string `json:"instruction"`
	Scope       string `json:"scope"`
	Trigger     string `json:"trigger"`
	EntryScript string `json:"entry_script"`
	FilePath    string `json:"file_path"`
	RiskLevel   string `json:"risk_level,omitempty"`
}

type MCPConfig struct {
	Name      string            `json:"name"`
	Transport string            `json:"transport"` // "stdio" 或 "http"
	Command   string            `json:"command"`    // stdio 模式: 启动命令
	Args      []string          `json:"args"`      // stdio 模式: 命令参数
	Env       map[string]string `json:"env"`       // stdio 模式: 环境变量
	Endpoint  string            `json:"endpoint"`   // http 模式: MCP 服务地址
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

type RetryConfig struct {
	MaxAttempts       int      `json:"max_attempts"`
	InitialDelayMs    int      `json:"initial_delay_ms"`
	MaxDelayMs        int      `json:"max_delay_ms"`
	BackoffMultiplier float64  `json:"backoff_multiplier"`
	RetryableErrors   []string `json:"retryable_errors"`
}

type RoutingConfig struct {
	DefaultModel    string `json:"default_model"`
	RewritePrompt   string `json:"rewrite_prompt"`
	SummarizePrompt string `json:"summarize_prompt"`
}

type ApprovalPolicy struct {
	Enabled        bool     `json:"enabled"`
	RiskThreshold  string   `json:"risk_threshold"`
	AutoApprove    []string `json:"auto_approve"`
}

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

type SandboxLimits struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

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

type PendingApproval struct {
	InterruptID    string `json:"interrupt_id"`
	ToolName       string `json:"tool_name"`
	ToolType       string `json:"tool_type"`
	ArgumentsJSON   string `json:"arguments_json"`
	RiskLevel      string `json:"risk_level"`
	Description    string `json:"description"`
}

type ToolCall struct {
	Tool   string `json:"tool"`
	Input  any    `json:"input"`
	Output any    `json:"output"`
}

type A2AResult struct {
	AgentName string `json:"agent_name"`
	Status    string `json:"status"`
	Result    any    `json:"result,omitempty"`
	Error     string `json:"error,omitempty"`
}

type ResponseMetadata struct {
	Model              string             `json:"model"`
	LatencyMs          int64              `json:"latency_ms"`
	TokensUsed         int                `json:"tokens_used,omitempty"`
	PromptTokens       int                `json:"prompt_tokens,omitempty"`
	CompletionTokens   int                `json:"completion_tokens,omitempty"`
	ToolCallsCount     int                `json:"tool_calls_count,omitempty"`
	A2ACallsCount      int                `json:"a2a_calls_count,omitempty"`
	SkillCallsCount    int                `json:"skill_calls_count,omitempty"`
	Iterations         int                `json:"iterations,omitempty"`
	ToolCallsDetail    []ToolCallMetadata `json:"tool_calls_detail,omitempty"`
	Error              string             `json:"error,omitempty"`
}

type ToolCallMetadata struct {
	Tool      string `json:"tool"`
	Input     any    `json:"input"`
	Output    any    `json:"output"`
	LatencyMs int64  `json:"latency_ms"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

// ========== 测试配置（简化版，用于加载 JSON 文件）==========

type TestConfig struct {
	Endpoint     string            `json:"endpoint"`
	Models       map[string]ModelConfig `json:"models"`
	SystemPrompt string            `json:"system_prompt"`
	UserMessage  string            `json:"user_message"`
	Tools        []ToolConfig     `json:"tools"`
	A2A          []A2AAgentConfig  `json:"a2a"`
	MCPs         []MCPConfig       `json:"mcps"`
	Skills       []Skill          `json:"skills"`
	Sandbox      *SandboxConfig   `json:"sandbox"`
	Options      *RunOptions      `json:"options"`
	Knowledge    []KnowledgeItem  `json:"knowledge"`
	Context      map[string]any   `json:"context"`
}

func main() {
	configPath := "test-all.json"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	configData, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("读取配置文件失败: %v", err)
	}

	var config TestConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		log.Fatalf("解析配置文件失败: %v", err)
	}

	// 展开环境变量
	models := expandEnvModels(config.Models)

	// 构建请求
	req := RunRequest{
		Prompt:    config.SystemPrompt,
		Models:    models,
		Messages:  []Message{{Role: "user", Content: config.UserMessage}},
		Skills:    config.Skills,
		Tools:     config.Tools,
		A2A:       config.A2A,
		MCPs:      config.MCPs,
		Sandbox:   config.Sandbox,
		Options:    config.Options,
		Knowledge:  config.Knowledge,
		Context:    config.Context,
	}

	// 发送请求
	reqBytes, _ := json.Marshal(req)

	log.Println("========== 开始执行 Runner ==========")
	log.Printf("Endpoint: %s", config.Endpoint)
	log.Printf("Models: %d", len(config.Models))
	for k, v := range config.Models {
		log.Printf("  - %s: %s", k, v.Name)
	}
	log.Printf("System Prompt: %s", truncateString(config.SystemPrompt, 100))
	log.Printf("User Message: %s", truncateString(config.UserMessage, 100))
	log.Printf("Tools: %d", len(config.Tools))
	for _, t := range config.Tools {
		log.Printf("  - %s (risk_level=%s)", t.Name, t.RiskLevel)
	}
	log.Printf("A2A Agents: %d", len(config.A2A))
	for _, a := range config.A2A {
		log.Printf("  - %s (risk_level=%s)", a.Name, a.RiskLevel)
	}
	log.Printf("MCPs: %d", len(config.MCPs))
	for _, m := range config.MCPs {
		log.Printf("  - %s (transport=%s, risk_level=%s)", m.Name, m.Transport, m.RiskLevel)
	}
	log.Printf("Skills: %d", len(config.Skills))
	for _, s := range config.Skills {
		log.Printf("  - %s (risk_level=%s)", s.ID, s.RiskLevel)
	}
	if config.Options != nil && config.Options.ApprovalPolicy != nil {
		log.Printf("ApprovalPolicy: enabled=%v, threshold=%s", config.Options.ApprovalPolicy.Enabled, config.Options.ApprovalPolicy.RiskThreshold)
	}
	if config.Sandbox != nil && config.Sandbox.Limits != nil {
		log.Printf("Sandbox Limits: cpu=%s, memory=%s", config.Sandbox.Limits.CPU, config.Sandbox.Limits.Memory)
	}

	start := time.Now()

	httpReq, err := http.NewRequestWithContext(context.Background(), "POST", config.Endpoint, bytes.NewReader(reqBytes))
	if err != nil {
		log.Fatalf("创建请求失败: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Fatalf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("读取响应失败: %v", err)
	}

	elapsed := time.Since(start)

	if resp.StatusCode != http.StatusOK {
		log.Printf("响应状态码: %d", resp.StatusCode)
		log.Printf("响应内容: %s", string(body))
		log.Fatalf("请求失败")
	}

	var result RunResponse
	if err := json.Unmarshal(body, &result); err != nil {
		log.Fatalf("解析响应失败: %v", err)
	}

	log.Println()
	log.Println("========== 执行结果 ==========")
	log.Printf("耗时: %v", elapsed)
	log.Printf("Finish Reason: %s", result.FinishReason)
	if result.CheckPointID != "" {
		log.Printf("CheckPointID: %s", result.CheckPointID)
	}

	if result.Content != "" {
		log.Println()
		log.Println("----------- Content -----------")
		fmt.Println(result.Content)
	}

	if len(result.ToolCalls) > 0 {
		log.Println()
		log.Println("----------- Tool Calls -----------")
		for i, tc := range result.ToolCalls {
			log.Printf("%d. Tool: %s", i+1, tc.Tool)
			if input, ok := tc.Input.(string); ok {
				log.Printf("   Input: %s", truncateString(input, 200))
			} else {
				log.Printf("   Input: %+v", tc.Input)
			}
			if tc.Output != nil {
				if output, ok := tc.Output.(string); ok {
					log.Printf("   Output: %s", truncateString(output, 200))
				} else {
					log.Printf("   Output: %+v", tc.Output)
				}
			}
		}
	}

	if len(result.A2AResults) > 0 {
		log.Println()
		log.Println("----------- A2A Results -----------")
		for i, ar := range result.A2AResults {
			log.Printf("%d. Agent: %s, Status: %s", i+1, ar.AgentName, ar.Status)
			if ar.Error != "" {
				log.Printf("   Error: %s", ar.Error)
			}
		}
	}

	if len(result.PendingApprovals) > 0 {
		log.Println()
		log.Println("----------- Pending Approvals -----------")
		for i, pa := range result.PendingApprovals {
			log.Printf("%d. Tool: %s (%s), Risk: %s, InterruptID: %s", i+1, pa.ToolName, pa.ToolType, pa.RiskLevel, pa.InterruptID)
			log.Printf("   Arguments: %s", truncateString(pa.ArgumentsJSON, 200))
		}
	}

	if result.Metadata.Model != "" {
		log.Println()
		log.Println("----------- Metadata -----------")
		log.Printf("Model: %s", result.Metadata.Model)
		log.Printf("Latency: %dms", result.Metadata.LatencyMs)
		if result.Metadata.PromptTokens > 0 {
			log.Printf("Prompt Tokens: %d", result.Metadata.PromptTokens)
		}
		if result.Metadata.CompletionTokens > 0 {
			log.Printf("Completion Tokens: %d", result.Metadata.CompletionTokens)
		}
		if result.Metadata.ToolCallsCount > 0 {
			log.Printf("Tool Calls Count: %d", result.Metadata.ToolCallsCount)
		}
		if result.Metadata.A2ACallsCount > 0 {
			log.Printf("A2A Calls Count: %d", result.Metadata.A2ACallsCount)
		}
		if result.Metadata.Iterations > 0 {
			log.Printf("Iterations: %d", result.Metadata.Iterations)
		}
		if result.Metadata.Error != "" {
			log.Printf("Error: %s", result.Metadata.Error)
		}
	}

	if len(result.A2UIMessages) > 0 {
		log.Println()
		log.Println("----------- A2UI Messages -----------")
		a2uiJSON, _ := json.MarshalIndent(result.A2UIMessages, "", "  ")
		log.Printf("%s", string(a2uiJSON))
	}

	log.Println()
	log.Println("========== 执行完成 ==========")
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func expandEnvStr(s string) string {
	re := regexp.MustCompile(`\$\{([^}]+)\}`)
	return re.ReplaceAllStringFunc(s, func(match string) string {
		envVar := match[2 : len(match)-1]
		return os.Getenv(envVar)
	})
}

func expandEnvModels(models map[string]ModelConfig) map[string]ModelConfig {
	result := make(map[string]ModelConfig)
	for k, v := range models {
		result[k] = ModelConfig{
			Provider:    v.Provider,
			Name:        expandEnvStr(v.Name),
			APIKey:      expandEnvStr(v.APIKey),
			APIBase:     expandEnvStr(v.APIBase),
			Temperature: v.Temperature,
			MaxTokens:   v.MaxTokens,
			TopP:        v.TopP,
		}
	}
	return result
}
