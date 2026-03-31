package subagent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
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
	var agentList []string
	for _, cfg := range subAgents {
		agentList = append(agentList, cfg.ID)
	}

	agentDesc := ""
	if len(agentList) > 0 {
		agentDesc = fmt.Sprintf("Available agents: %v", agentList)
	} else {
		agentDesc = "No sub-agents configured"
	}

	params := schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"agent_id": {
			Type:     schema.String,
			Desc:     "The ID of the sub-agent to delegate to. " + agentDesc,
			Required: true,
		},
		"task": {
			Type:     schema.String,
			Desc:     "The task description to send to the sub-agent",
			Required: true,
		},
		"context": {
			Type:     schema.String,
			Desc:     "Additional context information (optional)",
			Required: false,
		},
	})

	return &schema.ToolInfo{
		Name:        "delegate_to_agent",
		Desc:        fmt.Sprintf("Delegate a task to a sub-agent for independent execution. %s", agentDesc),
		ParamsOneOf: params,
	}, nil
}

// InvokableRun 执行委托
func (t *DelegateTool) InvokableRun(ctx context.Context, argumentsInJSON string, opt ...tool.Option) (string, error) {
	var input DelegateInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("parse delegate input failed: %w", err)
	}

	log.Printf("[DelegateTool] Delegating task to agent: %s", input.AgentID)

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

	log.Printf("[DelegateTool] Agent %s completed, output length: %d", input.AgentID, len(result.Output))

	return string(resultJSON), nil
}
