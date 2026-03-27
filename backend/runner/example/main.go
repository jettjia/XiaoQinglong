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

// TestConfig 测试配置
type TestConfig struct {
	// Runner 服务地址
	Endpoint string `json:"endpoint"`

	// Model 配置
	Model ModelConfig `json:"model"`

	// 系统提示词
	SystemPrompt string `json:"system_prompt"`

	// 用户消息
	UserMessage string `json:"user_message"`

	// Skills 配置
	Skills []Skill `json:"skills"`

	// Tools 配置
	Tools []ToolConfig `json:"tools"`

	// A2A 配置
	A2A []A2AAgentConfig `json:"a2a"`

	// MCP 配置
	MCPs []MCPConfig `json:"mcps"`

	// 沙箱配置
	Sandbox *SandboxConfig `json:"sandbox"`

	// 运行选项
	Options *RunOptions `json:"options"`

	// 知识库
	Knowledge []KnowledgeItem `json:"knowledge"`
}

// ModelConfig 模型配置
type ModelConfig struct {
	Provider string `json:"provider"`
	Name     string `json:"name"`
	APIKey   string `json:"api_key"`
	APIBase  string `json:"api_base"`
}

// Skill Skill配置
type Skill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Instruction string `json:"instruction"`
	Scope       string `json:"scope"`
	Trigger     string `json:"trigger"`
	EntryScript string `json:"entry_script"`
	FilePath    string `json:"file_path"`
}

// ToolConfig HTTP Tool配置
type ToolConfig struct {
	Type        string            `json:"type"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Endpoint    string            `json:"endpoint"`
	Method      string            `json:"method"`
	Headers     map[string]string `json:"headers"`
	RiskLevel   string            `json:"risk_level"`
}

// A2AAgentConfig A2A Agent配置
type A2AAgentConfig struct {
	Name     string            `json:"name"`
	Endpoint string            `json:"endpoint"`
	Headers  map[string]string `json:"headers"`
}

// MCPConfig MCP配置
type MCPConfig struct {
	Name    string            `json:"name"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
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
}

// RunOptions 运行选项
type RunOptions struct {
	Temperature   float64  `json:"temperature"`
	MaxTokens     int      `json:"max_tokens"`
	Stream        bool     `json:"stream"`
	TopP          float64  `json:"top_p"`
	Stop          []string `json:"stop"`
	TimeoutMs     int      `json:"timeout_ms"`
	MaxIterations int      `json:"max_iterations"`
	MaxToolCalls  int      `json:"max_tool_calls"`
	MaxA2ACalls   int      `json:"max_a2a_calls"`
}

// KnowledgeItem 知识库条目
type KnowledgeItem struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

// ========== Request/Response ==========

type RunRequest struct {
	Prompt         string                 `json:"prompt"`
	Models         map[string]ModelConfig `json:"models"`
	Messages       []Message              `json:"messages"`
	Context        map[string]any         `json:"context"`
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

type InternalAgentConfig struct {
	ID     string      `json:"id"`
	Name   string      `json:"name"`
	Prompt string      `json:"prompt"`
	Model  ModelConfig `json:"model"`
}

type RunResponse struct {
	Content      string           `json:"content,omitempty"`
	ToolCalls    []ToolCall       `json:"tool_calls,omitempty"`
	A2AResults   []A2AResult      `json:"a2a_results,omitempty"`
	TokensUsed   int              `json:"tokens_used,omitempty"`
	FinishReason string           `json:"finish_reason"`
	Metadata     ResponseMetadata `json:"metadata"`
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
	Model     string `json:"model"`
	LatencyMs int64  `json:"latency_ms"`
}

func main() {
	// 读取测试配置
	configPath := "test.json"
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
	model := expandEnvModel(config.Model)

	// 构建请求
	req := RunRequest{
		Prompt:    config.SystemPrompt,
		Models:    map[string]ModelConfig{"default": model},
		Messages:  []Message{{Role: "user", Content: config.UserMessage}},
		Skills:    config.Skills,
		Tools:     config.Tools,
		A2A:       config.A2A,
		MCPs:      config.MCPs,
		Sandbox:   config.Sandbox,
		Options:   config.Options,
		Knowledge: config.Knowledge,
		Context: map[string]any{
			"skills_dir": "/home/jett/aishu/XiaoQinglong/skills",
		},
	}

	// 发送请求
	reqBytes, _ := json.Marshal(req)

	log.Println("========== 开始执行 Runner ==========")
	log.Printf("Endpoint: %s", config.Endpoint)
	log.Printf("Model: %s", config.Model.Name)
	log.Printf("System Prompt: %s", truncateString(config.SystemPrompt, 100))
	log.Printf("User Message: %s", truncateString(config.UserMessage, 100))
	log.Printf("Tools: %d", len(config.Tools))
	log.Printf("A2A Agents: %d", len(config.A2A))
	log.Printf("Skills: %d", len(config.Skills))

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

	if result.Metadata.Model != "" {
		log.Println()
		log.Println("----------- Metadata -----------")
		log.Printf("Model: %s", result.Metadata.Model)
		log.Printf("Latency: %dms", result.Metadata.LatencyMs)
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

// expandEnvStr 展开字符串中的 ${ENV_VAR} 格式的环境变量
func expandEnvStr(s string) string {
	re := regexp.MustCompile(`\$\{([^}]+)\}`)
	return re.ReplaceAllStringFunc(s, func(match string) string {
		envVar := match[2 : len(match)-1] // 去掉 ${ 和 }
		return os.Getenv(envVar)
	})
}

// expandEnvModel 展开 ModelConfig 中的环境变量
func expandEnvModel(m ModelConfig) ModelConfig {
	return ModelConfig{
		Provider: m.Provider,
		Name:     expandEnvStr(m.Name),
		APIKey:   expandEnvStr(m.APIKey),
		APIBase:  expandEnvStr(m.APIBase),
	}
}
