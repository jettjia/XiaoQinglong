package subagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/jettjia/XiaoQinglong/runner/pkg/logger"
)

// DelegateInput 委托工具输入
type DelegateInput struct {
	AgentID string `json:"agent_id"`
	Task    string `json:"task"`
	Context string `json:"context,omitempty"` // 额外上下文
}

// DelegateTool 委托工具 - 让主 Agent 可以调用 Sub-Agent
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
		for id, cfg := range subAgents {
			agentDesc.WriteString(fmt.Sprintf("- %s (%s): %s\n", id, cfg.Name, cfg.Description))
		}
	} else {
		agentDesc.WriteString("No sub-agents configured")
	}

	params := schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"agent_id": {
			Type:     schema.String,
			Desc:     fmt.Sprintf("The ID of the sub-agent to delegate to.\n%s", agentDesc.String()),
			Required: true,
		},
		"task": {
			Type:     schema.String,
			Desc:     "The task description to send to the sub-agent. Include all relevant context and details the sub-agent needs to complete the task.",
			Required: true,
		},
		"context": {
			Type:     schema.String,
			Desc:     "Additional context information such as user preferences, session info, or previous conversation history (optional)",
			Required: false,
		},
	})

	return &schema.ToolInfo{
		Name:        "delegate_to_agent",
		Desc:        "Use this tool when a task requires specialized knowledge or processing that goes beyond your capabilities. Delegate to a sub-agent who has specific expertise.\n\nWhen to use:\n- User asks about HR policies (leave, reimbursement, benefits) → delegate to hr_policy_assistant\n- User asks about technical documentation or code → delegate to technical_assistant\n- User asks about orders, payments → delegate to order_assistant\n- Task requires deep domain expertise you don't have\n\n" + agentDesc.String(),
		ParamsOneOf: params,
	}, nil
}

// InvokableRun 执行委托
func (t *DelegateTool) InvokableRun(ctx context.Context, argumentsInJSON string, opt ...tool.Option) (string, error) {
	var input DelegateInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("parse delegate input failed: %w", err)
	}

	logger.GetRunnerLogger().Infof("[DelegateTool] Delegating task to agent: %s", input.AgentID)

	// 构建任务描述，添加额外上下文
	task := input.Task
	if input.Context != "" {
		task = fmt.Sprintf("%s\n\nAdditional context:\n%s", task, input.Context)
	}

	// 运行 Sub-Agent
	result, err := t.manager.Run(ctx, input.AgentID, task)
	if err != nil {
		return "", fmt.Errorf("delegate to agent %s failed: %w", input.AgentID, err)
	}

	// 序列化结果
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal result failed: %w", err)
	}

	logger.GetRunnerLogger().Infof("[DelegateTool] Agent %s completed, output length: %d", input.AgentID, len(result.Output))

	return string(resultJSON), nil
}
