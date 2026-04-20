package subagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/jettjia/XiaoQinglong/runner/pkg/logger"
)

// 最大并发子代理数
const MaxConcurrentChildren = 3

// 最大委派深度（父->子->孙）
const MaxDelegateDepth = 2

// BlockedTools 禁止子代理使用的工具
var BlockedTools = map[string]bool{
	"delegate_to_agent": true, // 禁止递归委派
	"clarify":          true, // 禁止用户交互
	"memory":           true, // 禁止写共享记忆
	"send_message":     true, // 禁止跨平台副作用
}

// DelegateTask 单个委派任务
type DelegateTask struct {
	Task      string `json:"task"`                // 任务描述
	Context   string `json:"context,omitempty"`   // 额外上下文
	Tools     []string `json:"tools,omitempty"`  // 允许的工具（可选，不填则用 sub_agent 配置的默认工具）
	Workspace string   `json:"workspace,omitempty"` // 工作目录
}

// DelegateInput 委托工具输入（hermes-agent 风格，支持 batch）
type DelegateInput struct {
	AgentID string        `json:"agent_id"` // sub-agent ID（兼容旧版）
	Task    string        `json:"task"`     // 任务描述（兼容旧版）
	Context string        `json:"context,omitempty"`
	// 新版 batch 支持
	Tasks []DelegateTask `json:"tasks,omitempty"` // 批量任务（与 Task 二选一）
	Depth int            `json:"depth,omitempty"` // 当前深度（自动传递）
}

// IsBatch 是否是批量模式
func (d *DelegateInput) IsBatch() bool {
	return len(d.Tasks) > 0 || d.Task == ""
}

// DelegateResult 单个任务结果
type DelegateResult struct {
	Success bool   `json:"success"`
	Output  string `json:"output,omitempty"`  // 完整输出
	Summary string `json:"summary,omitempty"` // 摘要
	Error   string `json:"error,omitempty"`
}

// DelegateOutput 批量结果
type DelegateOutput struct {
	Results []DelegateResult `json:"results"`
}

// DelegateTool 委托工具 - 让主 Agent 可以调用 Sub-Agent（hermes-agent 风格）
type DelegateTool struct {
	manager *SubAgentManager
}

// NewDelegateTool 创建委托工具
func NewDelegateTool(manager *SubAgentManager) *DelegateTool {
	return &DelegateTool{
		manager: manager,
	}
}

// Info 返回工具信息
func (t *DelegateTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	// 获取所有可用的 Sub-Agent
	subAgents := t.manager.ListSubAgents()

	// 构建详细的 agent 描述
	var agentDesc strings.Builder
	if len(subAgents) > 0 {
		agentDesc.WriteString("Available sub-agents and their capabilities:\n")
		for _, cfg := range subAgents {
			agentDesc.WriteString(fmt.Sprintf("- %s (%s): %s\n", cfg.ID, cfg.Name, cfg.Description))
		}
	} else {
		agentDesc.WriteString("No sub-agents configured")
	}

	params := schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"agent_id": {
			Type:     schema.String,
			Desc:     fmt.Sprintf("The ID of the sub-agent to delegate to.\n%s", agentDesc.String()),
			Required: false,
		},
		"task": {
			Type:     schema.String,
			Desc:     "Single task description (legacy mode, use 'tasks' for batch)",
			Required: false,
		},
		"context": {
			Type:     schema.String,
			Desc:     "Additional context for the task",
			Required: false,
		},
		"tasks": {
			Type:    schema.String,
			Desc:    "JSON array of tasks for batch delegation: [{\"task\":\"description\",\"context\":\"...\",\"tools\":[\"tool1\"]},...]. Max 3 tasks in parallel.",
			Required: false,
		},
		"depth": {
			Type:     schema.Integer,
			Desc:     "Current delegation depth (auto-managed, don't set manually)",
			Required: false,
		},
	})

	return &schema.ToolInfo{
		Name:        "delegate_to_agent",
		Desc:        "Delegate tasks to sub-agents for parallel execution. Use this when:\n- Multiple independent tasks can be executed in parallel\n- A task requires specialized capabilities\n\nBatch mode (preferred): Set 'tasks' array to delegate multiple tasks concurrently.\nSingle mode (legacy): Set 'agent_id' and 'task' for one task.\n\n" + agentDesc.String(),
		ParamsOneOf: params,
	}, nil
}

// InvokableRun 执行委托（支持 batch 批量模式）
func (t *DelegateTool) InvokableRun(ctx context.Context, argumentsInJSON string, opt ...tool.Option) (string, error) {
	var input DelegateInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("parse delegate input failed: %w", err)
	}

	// 检查深度限制
	if input.Depth >= MaxDelegateDepth {
		return "", fmt.Errorf("max delegate depth %d exceeded", MaxDelegateDepth)
	}

	// 批量模式
	if len(input.Tasks) > 0 {
		return t.runBatch(ctx, input)
	}

	// 单任务模式（兼容旧版）
	return t.runSingle(ctx, input)
}

// runSingle 执行单任务
func (t *DelegateTool) runSingle(ctx context.Context, input DelegateInput) (string, error) {
	if input.AgentID == "" || input.Task == "" {
		return "", fmt.Errorf("agent_id and task are required in single mode")
	}

	logger.GetRunnerLogger().Infof("[DelegateTool] Delegating single task to agent: %s", input.AgentID)

	// 构建任务描述
	task := input.Task
	if input.Context != "" {
		task = fmt.Sprintf("%s\n\nAdditional context:\n%s", task, input.Context)
	}

	// 检查深度
	if input.Depth >= MaxDelegateDepth {
		return "", fmt.Errorf("max delegate depth %d exceeded", MaxDelegateDepth)
	}

	// 运行 Sub-Agent
	result, err := t.manager.Run(ctx, input.AgentID, task)
	if err != nil {
		return "", fmt.Errorf("delegate to agent %s failed: %w", input.AgentID, err)
	}

	// 返回带摘要的结果
	output := DelegateOutput{
		Results: []DelegateResult{
			{
				Success: result.Status == "completed",
				Output:  result.Output,
				Summary: summarizeResult(result.Output),
				Error:   result.Error,
			},
		},
	}

	resultJSON, err := json.Marshal(output)
	if err != nil {
		return "", fmt.Errorf("marshal result failed: %w", err)
	}

	logger.GetRunnerLogger().Infof("[DelegateTool] Agent %s completed, output length: %d", input.AgentID, len(result.Output))
	return string(resultJSON), nil
}

// runBatch 执行批量任务（并行）
func (t *DelegateTool) runBatch(ctx context.Context, input DelegateInput) (string, error) {
	// 限制并发数
	taskCount := len(input.Tasks)
	if taskCount > MaxConcurrentChildren {
		logger.GetRunnerLogger().Warnf("[DelegateTool] Task count %d exceeds limit %d, truncating", taskCount, MaxConcurrentChildren)
		input.Tasks = input.Tasks[:MaxConcurrentChildren]
		taskCount = MaxConcurrentChildren
	}

	// 检查深度
	if input.Depth >= MaxDelegateDepth {
		return "", fmt.Errorf("max delegate depth %d exceeded", MaxDelegateDepth)
	}

	agentID := input.AgentID
	if agentID == "" {
		// 如果没指定 agent_id，使用第一个配置的
		agents := t.manager.ListSubAgents()
		for _, cfg := range agents {
			agentID = cfg.ID
			break
		}
	}

	logger.GetRunnerLogger().Infof("[DelegateTool] Delegating batch of %d tasks to agent: %s", taskCount, agentID)

	// 并行执行任务
	results := make([]DelegateResult, taskCount)
	var wg sync.WaitGroup

	for i, task := range input.Tasks {
		wg.Add(1)
		go func(idx int, tsk DelegateTask) {
			defer wg.Done()

			// 构建任务描述
			taskDesc := tsk.Task
			if tsk.Context != "" {
				taskDesc = fmt.Sprintf("%s\n\nAdditional context:\n%s", tsk.Task, tsk.Context)
			}

			// 过滤 blocked tools（注：当前 Run 接口不支持自定义 tools，留待后续扩展）
			_ = filterBlockedTools(tsk.Tools)

			// 运行 Sub-Agent
			result, err := t.manager.Run(ctx, agentID, taskDesc)

			if err != nil {
				results[idx] = DelegateResult{
					Success: false,
					Error:   err.Error(),
				}
				return
			}

			results[idx] = DelegateResult{
				Success: result.Status == "completed",
				Output:  result.Output,
				Summary: summarizeResult(result.Output),
				Error:   result.Error,
			}
		}(i, task)
	}

	wg.Wait()

	output := DelegateOutput{Results: results}

	resultJSON, err := json.Marshal(output)
	if err != nil {
		return "", fmt.Errorf("marshal batch result failed: %w", err)
	}

	logger.GetRunnerLogger().Infof("[DelegateTool] Batch completed, %d tasks", taskCount)
	return string(resultJSON), nil
}

// filterBlockedTools 过滤禁止的工具
func filterBlockedTools(tools []string) []string {
	if len(tools) == 0 {
		return nil
	}
	var filtered []string
	for _, t := range tools {
		if !BlockedTools[t] {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// summarizeResult 生成结果摘要（hermes-agent 风格）
func summarizeResult(output string) string {
	if output == "" {
		return "(无输出)"
	}
	// 截断到 500 字符
	const maxLen = 500
	if len(output) <= maxLen {
		return output
	}
	// 尝试在句子边界截断
	truncated := output[:maxLen]
	if idx := strings.LastIndex(truncated, "。"); idx > maxLen/2 {
		return truncated[:idx+1] + "\n[摘要: 内容已截断]"
	}
	if idx := strings.LastIndex(truncated, "\n"); idx > maxLen/2 {
		return truncated[:idx] + "\n[摘要: 内容已截断]"
	}
	return truncated + "...\n[摘要: 内容已截断]"
}
