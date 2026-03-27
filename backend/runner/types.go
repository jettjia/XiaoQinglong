package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

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
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
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
}

type MCPConfig struct {
	Name    string            `json:"name"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

type A2AAgentConfig struct {
	Name     string            `json:"name"`
	Endpoint string            `json:"endpoint"`
	Headers  map[string]string `json:"headers"`
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
	ResponseFormat *ResponseFormatConfig `json:"response_format"`
}

type RetryConfig struct {
	MaxAttempts       int      `json:"max_attempts"`
	InitialDelayMs    int      `json:"initial_delay_ms"`
	MaxDelayMs        int      `json:"max_delay_ms"`
	BackoffMultiplier float64  `json:"backoff_multiplier"`
	RetryableErrors   []string `json:"retryable_errors"`
}

type ResponseFormatConfig struct {
	Type      string                 `json:"type"`
	Version   string                 `json:"version"`
	Strict    bool                   `json:"strict"`
	Fallback  string                 `json:"fallback"`
	Templates map[string]any         `json:"templates"`
}

type SandboxConfig struct {
	Enabled   bool              `json:"enabled"`
	Mode      string            `json:"mode"`
	Image     string            `json:"image"`
	Workdir   string            `json:"workdir"`
	Network   string            `json:"network"`
	TimeoutMs int               `json:"timeout_ms"`
	Env       map[string]string `json:"env"`
}

// ========== Response Types ==========

type RunResponse struct {
	Content      string            `json:"content,omitempty"`
	ToolCalls    []ToolCall        `json:"tool_calls,omitempty"`
	A2AResults   []A2AResult       `json:"a2a_results,omitempty"`
	TokensUsed   int               `json:"tokens_used,omitempty"`
	FinishReason string            `json:"finish_reason"`
	Metadata     ResponseMetadata  `json:"metadata"`
	A2UIMessages []json.RawMessage `json:"a2ui_messages,omitempty"`
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

	resp := &RunResponse{
		Content:      result.Content,
		ToolCalls:    result.ToolCalls,
		A2AResults:   result.A2AResults,
		TokensUsed:   result.TokensUsed,
		FinishReason: result.FinishReason,
		Metadata: ResponseMetadata{
			Model:     r.getDefaultModelName(),
			LatencyMs: latencyMs,
		},
		A2UIMessages: result.A2UIMessages,
	}

	return resp, nil
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