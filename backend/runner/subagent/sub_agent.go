package subagent

import (
	"context"
	"sync"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// InMemoryCheckPointStore 内存中的检查点存储
type InMemoryCheckPointStore struct {
	mem map[string][]byte
}

func NewInMemoryCheckPointStore() *InMemoryCheckPointStore {
	return &InMemoryCheckPointStore{
		mem: map[string][]byte{},
	}
}

func (i *InMemoryCheckPointStore) Set(ctx context.Context, key string, value []byte) error {
	i.mem[key] = value
	return nil
}

func (i *InMemoryCheckPointStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	v, ok := i.mem[key]
	return v, ok, nil
}

// SubAgentStatus Sub-Agent 状态
type SubAgentStatus string

const (
	SubAgentStatusIdle      SubAgentStatus = "idle"
	SubAgentStatusRunning   SubAgentStatus = "running"
	SubAgentStatusCompleted SubAgentStatus = "completed"
	SubAgentStatusFailed    SubAgentStatus = "failed"
	SubAgentStatusCancelled SubAgentStatus = "cancelled"
)

// ModelConfig 模型配置
type ModelConfig struct {
	Name    string `json:"name"`
	APIKey  string `json:"api_key,omitempty"`
	APIBase string `json:"api_base,omitempty"`
}

// SubAgentConfig Sub-Agent 配置
type SubAgentConfig struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Description   string       `json:"description"`
	Prompt        string       `json:"prompt"`
	Model         *ModelConfig `json:"model,omitempty"`  // 可选，默认使用主模型
	Tools         []string     `json:"tools,omitempty"`  // 工具名称列表
	Skills        []string     `json:"skills,omitempty"` // 技能列表
	MCPs          []string     `json:"mcps,omitempty"`   // MCP 配置
	MaxIterations int          `json:"max_iterations"`   // 最大迭代次数
	TimeoutMs     int          `json:"timeout_ms"`       // 超时时间
}

// SubAgentResult Sub-Agent 执行结果
type SubAgentResult struct {
	AgentID    string         `json:"agent_id"`
	AgentName  string         `json:"agent_name"`
	Status     SubAgentStatus `json:"status"`
	Output     string         `json:"output"`
	TokensUsed int            `json:"tokens_used"`
	LatencyMs  int64          `json:"latency_ms"`
	Error      string         `json:"error,omitempty"`
}

// SubAgent 独立的 Sub-Agent 实例
type SubAgent struct {
	ID       string
	Config   *SubAgentConfig
	Agent    adk.Agent
	Status   SubAgentStatus
	Result   *SubAgentResult
	CancelFn context.CancelFunc
	mu       sync.RWMutex
}

// newSubAgent 创建 Sub-Agent 实例
func newSubAgent(ctx context.Context, cfg *SubAgentConfig, defaultModel model.ToolCallingChatModel, tools []interface{}) (*SubAgent, error) {
	// 构建 instruction
	instruction := cfg.Prompt
	if cfg.Description != "" {
		instruction = cfg.Description + "\n\n" + instruction
	}

	// 确定使用的模型
	agentModel := defaultModel
	if cfg.Model != nil && cfg.Model.Name != "" {
		// TODO: 创建独立的模型实例
		// 目前暂时使用默认模型
	}

	// 创建 Agent
	agentCfg := &adk.ChatModelAgentConfig{
		Name:          cfg.Name,
		Description:   cfg.Description,
		Instruction:   instruction,
		Model:         agentModel,
		MaxIterations: cfg.MaxIterations,
	}

	// 如果有工具配置，添加工具
	// 注意：Sub-Agent 的工具需要单独传递
	// 目前先支持无工具的简单执行

	agent, err := adk.NewChatModelAgent(ctx, agentCfg)
	if err != nil {
		return nil, err
	}

	return &SubAgent{
		ID:     cfg.ID,
		Config: cfg,
		Agent:  agent,
		Status: SubAgentStatusIdle,
	}, nil
}

// Run 运行 Sub-Agent 执行任务
func (s *SubAgent) Run(ctx context.Context, task string) error {
	s.mu.Lock()
	s.Status = SubAgentStatusRunning
	s.mu.Unlock()

	startTime := time.Now()

	// 创建带超时的 context
	if s.Config.TimeoutMs > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(s.Config.TimeoutMs)*time.Millisecond)
		s.CancelFn = cancel
	}

	// 构建消息
	messages := []adk.Message{
		schema.UserMessage(task),
	}

	// 创建 Runner
	checkpointStore := NewInMemoryCheckPointStore()
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           s.Agent,
		CheckPointStore: checkpointStore,
	})

	// 执行
	events := runner.Run(ctx, messages)

	var output string
	var tokensUsed int

	for {
		event, ok := events.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			s.mu.Lock()
			s.Status = SubAgentStatusFailed
			s.Result = &SubAgentResult{
				AgentID:   s.ID,
				AgentName: s.Config.Name,
				Status:    SubAgentStatusFailed,
				Output:    "",
				Error:     event.Err.Error(),
				LatencyMs: time.Since(startTime).Milliseconds(),
			}
			s.mu.Unlock()
			return event.Err
		}

		// 处理输出
		if event.Output != nil {
			if msg, err := event.Output.MessageOutput.GetMessage(); err == nil {
				output = msg.Content
				if msg.ResponseMeta != nil && msg.ResponseMeta.Usage != nil {
					tokensUsed = msg.ResponseMeta.Usage.TotalTokens
				}
			}
		}
	}

	s.mu.Lock()
	s.Status = SubAgentStatusCompleted
	s.Result = &SubAgentResult{
		AgentID:    s.ID,
		AgentName:  s.Config.Name,
		Status:     SubAgentStatusCompleted,
		Output:     output,
		TokensUsed: tokensUsed,
		LatencyMs:  time.Since(startTime).Milliseconds(),
	}
	s.mu.Unlock()

	return nil
}

// Cancel 取消 Sub-Agent 执行
func (s *SubAgent) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.CancelFn != nil {
		s.CancelFn()
	}
	if s.Status == SubAgentStatusRunning {
		s.Status = SubAgentStatusCancelled
	}
}

// GetResult 获取执行结果
func (s *SubAgent) GetResult() *SubAgentResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Result
}
