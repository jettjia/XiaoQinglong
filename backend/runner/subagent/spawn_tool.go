package subagent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/jettjia/XiaoQinglong/runner/pkg/logger"
)

// SpawnInput spawn 工具输入
type SpawnInput struct {
	AgentID string `json:"agent_id"`
	Task    string `json:"task"`
	Timeout int    `json:"timeout,omitempty"` // 超时秒数，默认使用 agent 配置
}

// SpawnTool spawn 工具 - 异步启动 Sub-Agent 并立即返回
type SpawnTool struct {
	manager       *SubAgentManager
	maxConcurrent int // 最大并发数，默认 3
}

// NewSpawnTool 创建 spawn 工具
func NewSpawnTool(manager *SubAgentManager) *SpawnTool {
	return &SpawnTool{
		manager:       manager,
		maxConcurrent: 3,
	}
}

// SetMaxConcurrent 设置最大并发数
func (t *SpawnTool) SetMaxConcurrent(max int) {
	t.maxConcurrent = max
}

// Info 返回工具信息
func (t *SpawnTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
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
			Desc:     "The ID of the sub-agent to spawn. " + agentDesc,
			Required: true,
		},
		"task": {
			Type:     schema.String,
			Desc:     "The task description to send to the sub-agent",
			Required: true,
		},
		"timeout": {
			Type:     schema.Integer,
			Desc:     "Timeout in seconds (optional, defaults to agent config)",
			Required: false,
		},
	})

	return &schema.ToolInfo{
		Name:        "spawn",
		Desc:        fmt.Sprintf("Spawn a sub-agent to execute a task asynchronously and return immediately with a task_id. The task runs in background. Use collect_task to get results later. %s", agentDesc),
		ParamsOneOf: params,
	}, nil
}

// InvokableRun 执行 spawn
func (t *SpawnTool) InvokableRun(ctx context.Context, argumentsInJSON string, opt ...tool.Option) (string, error) {
	var input SpawnInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("parse spawn input failed: %w", err)
	}

	logger.GetRunnerLogger().Printf("[SpawnTool] Spawning task for agent: %s", input.AgentID)

	// 启动异步任务
	taskInfo, err := t.manager.Spawn(ctx, input.AgentID, input.Task)
	if err != nil {
		return "", fmt.Errorf("spawn failed: %w", err)
	}

	// 构建响应
	response := map[string]any{
		"task_id":  taskInfo.TaskID,
		"agent_id": taskInfo.AgentID,
		"status":   taskInfo.Status,
		"message":  fmt.Sprintf("Task %s started successfully. Use collect_task to get results.", taskInfo.TaskID),
	}

	resultJSON, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("marshal response failed: %w", err)
	}

	logger.GetRunnerLogger().Printf("[SpawnTool] Task %s spawned successfully", taskInfo.TaskID)

	return string(resultJSON), nil
}

// CollectTaskInput collect task 工具输入
type CollectTaskInput struct {
	TaskID  string `json:"task_id"`
	Timeout int    `json:"timeout,omitempty"` // 等待超时秒数，默认 30
}

// CollectTaskTool collect task 工具 - 获取异步任务结果
type CollectTaskTool struct {
	manager *SubAgentManager
}

// NewCollectTaskTool 创建 collect task 工具
func NewCollectTaskTool(manager *SubAgentManager) *CollectTaskTool {
	return &CollectTaskTool{
		manager: manager,
	}
}

// Info 返回工具信息
func (t *CollectTaskTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	params := schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"task_id": {
			Type:     schema.String,
			Desc:     "The task ID returned by spawn",
			Required: true,
		},
		"timeout": {
			Type:     schema.Integer,
			Desc:     "Wait timeout in seconds (default: 30, max: 300)",
			Required: false,
		},
	})

	return &schema.ToolInfo{
		Name:        "collect_task",
		Desc:        "Collect the result of a previously spawned sub-agent task by its task_id. Waits for completion if task is still running.",
		ParamsOneOf: params,
	}, nil
}

// InvokableRun 执行 collect
func (t *CollectTaskTool) InvokableRun(ctx context.Context, argumentsInJSON string, opt ...tool.Option) (string, error) {
	var input CollectTaskInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("parse collect_task input failed: %w", err)
	}

	logger.GetRunnerLogger().Printf("[CollectTaskTool] Collecting task: %s", input.TaskID)

	// 如果没有指定 timeout，使用默认值
	timeout := 30
	if input.Timeout > 0 {
		timeout = input.Timeout
	}
	if timeout > 300 {
		timeout = 300 // 最大 5 分钟
	}

	// 等待任务完成
	taskInfo, err := t.manager.WaitTask(input.TaskID, time.Duration(timeout)*time.Second)
	if err != nil {
		// 超时错误特殊处理
		taskInfo, _ := t.manager.GetTaskStatus(input.TaskID)
		if taskInfo != nil {
			// 任务还在运行，返回当前状态
			response := map[string]any{
				"task_id": taskInfo.TaskID,
				"status":  taskInfo.Status,
				"message": fmt.Sprintf("Task still %s after %d seconds timeout. Use collect_task again to wait.", taskInfo.Status, timeout),
			}
			resultJSON, _ := json.Marshal(response)
			return string(resultJSON), nil
		}
		return "", fmt.Errorf("wait task failed: %w", err)
	}

	// 构建响应
	response := map[string]any{
		"task_id":  taskInfo.TaskID,
		"agent_id": taskInfo.AgentID,
		"status":   taskInfo.Status,
	}

	// 如果任务完成，添加结果
	if taskInfo.Result != nil {
		response["output"] = taskInfo.Result.Output
		response["tokens_used"] = taskInfo.Result.TokensUsed
		response["latency_ms"] = taskInfo.Result.LatencyMs
		if taskInfo.Result.Error != "" {
			response["error"] = taskInfo.Result.Error
		}
	}

	resultJSON, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("marshal response failed: %w", err)
	}

	logger.GetRunnerLogger().Printf("[CollectTaskTool] Task %s collected, status: %s", input.TaskID, taskInfo.Status)

	return string(resultJSON), nil
}

// ListTasksInput list tasks 工具输入
type ListTasksInput struct {
	Status string `json:"status,omitempty"` // 过滤状态：pending, running, completed, failed
}

// ListTasksTool list tasks 工具 - 列出所有任务
type ListTasksTool struct {
	manager *SubAgentManager
}

// NewListTasksTool 创建 list tasks 工具
func NewListTasksTool(manager *SubAgentManager) *ListTasksTool {
	return &ListTasksTool{
		manager: manager,
	}
}

// Info 返回工具信息
func (t *ListTasksTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	params := schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"status": {
			Type:     schema.String,
			Desc:     "Filter by status: pending, running, completed, failed, cancelled (optional)",
			Required: false,
		},
	})

	return &schema.ToolInfo{
		Name:        "list_tasks",
		Desc:        "List all sub-agent tasks with their status. Use this to check running tasks before collecting results.",
		ParamsOneOf: params,
	}, nil
}

// InvokableRun 执行 list tasks
func (t *ListTasksTool) InvokableRun(ctx context.Context, argumentsInJSON string, opt ...tool.Option) (string, error) {
	var input ListTasksInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("parse list_tasks input failed: %w", err)
	}

	tasks := t.manager.ListTasks()

	// 过滤任务
	var filteredTasks []map[string]any
	for _, task := range tasks {
		if input.Status != "" && task.Status != input.Status {
			continue
		}
		taskMap := map[string]any{
			"task_id":    task.TaskID,
			"agent_id":   task.AgentID,
			"status":     task.Status,
			"started_at": task.StartedAt.Format(time.RFC3339),
		}
		if task.Result != nil {
			taskMap["output_length"] = len(task.Result.Output)
		}
		filteredTasks = append(filteredTasks, taskMap)
	}

	response := map[string]any{
		"tasks": filteredTasks,
		"count": len(filteredTasks),
	}

	resultJSON, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("marshal response failed: %w", err)
	}

	return string(resultJSON), nil
}

// CancelTaskTool 取消任务工具
type CancelTaskTool struct {
	manager *SubAgentManager
}

// NewCancelTaskTool 创建 cancel task 工具
func NewCancelTaskTool(manager *SubAgentManager) *CancelTaskTool {
	return &CancelTaskTool{
		manager: manager,
	}
}

// Info 返回工具信息
func (t *CancelTaskTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	params := schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"task_id": {
			Type:     schema.String,
			Desc:     "The task ID to cancel",
			Required: true,
		},
	})

	return &schema.ToolInfo{
		Name:        "cancel_task",
		Desc:        "Cancel a running sub-agent task by its task_id.",
		ParamsOneOf: params,
	}, nil
}

// InvokableRun 执行 cancel
func (t *CancelTaskTool) InvokableRun(ctx context.Context, argumentsInJSON string, opt ...tool.Option) (string, error) {
	var input struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("parse cancel_task input failed: %w", err)
	}

	logger.GetRunnerLogger().Printf("[CancelTaskTool] Cancelling task: %s", input.TaskID)

	if err := t.manager.CancelTask(input.TaskID); err != nil {
		return "", fmt.Errorf("cancel task failed: %w", err)
	}

	response := map[string]any{
		"task_id": input.TaskID,
		"status":  "cancelled",
		"message": fmt.Sprintf("Task %s cancelled successfully", input.TaskID),
	}

	resultJSON, _ := json.Marshal(response)
	return string(resultJSON), nil
}
