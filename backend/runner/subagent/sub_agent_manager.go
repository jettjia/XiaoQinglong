package subagent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/jettjia/XiaoQinglong/runner/pkg/logger"
)

// TaskInfo 任务信息
type TaskInfo struct {
	TaskID    string
	AgentID   string
	Status    string // "pending", "running", "completed", "failed", "cancelled"
	Result    *SubAgentResult
	StartedAt time.Time
}

// SubAgentManager Sub-Agent 管理器
type SubAgentManager struct {
	configs      map[string]*SubAgentConfig // id -> config
	agents       map[string]*SubAgent       // id -> running agent
	defaultModel model.ToolCallingChatModel
	tools        map[string]interface{} // name -> tool
	mu           sync.RWMutex
	// 异步任务追踪
	tasks       map[string]*TaskInfo // taskID -> task info
	taskCounter int64
	taskMu      sync.RWMutex
}

// NewSubAgentManager 创建 Sub-Agent 管理器
func NewSubAgentManager(defaultModel model.ToolCallingChatModel) *SubAgentManager {
	return &SubAgentManager{
		configs:      make(map[string]*SubAgentConfig),
		agents:       make(map[string]*SubAgent),
		defaultModel: defaultModel,
		tools:        make(map[string]interface{}),
		tasks:        make(map[string]*TaskInfo),
	}
}

// NextTaskID 生成下一个任务 ID
func (m *SubAgentManager) NextTaskID() string {
	m.taskMu.Lock()
	defer m.taskMu.Unlock()
	m.taskCounter++
	return fmt.Sprintf("task_%d", m.taskCounter)
}

// RegisterConfigs 注册 Sub-Agent 配置
func (m *SubAgentManager) RegisterConfigs(configs []SubAgentConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range configs {
		cfg := &configs[i]
		m.configs[cfg.ID] = cfg
		logger.GetRunnerLogger().Infof("[SubAgentManager] Registered sub-agent: %s (%s)", cfg.ID, cfg.Name)
	}

	logger.GetRunnerLogger().Infof("[SubAgentManager] Total registered: %d sub-agents", len(m.configs))
}

// RegisterTool 注册工具到管理器
func (m *SubAgentManager) RegisterTool(name string, tool interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tools[name] = tool
}

// GetConfig 获取 Sub-Agent 配置
func (m *SubAgentManager) GetConfig(id string) (*SubAgentConfig, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg, ok := m.configs[id]
	return cfg, ok
}

// ListSubAgents 列出所有 Sub-Agent 配置
func (m *SubAgentManager) ListSubAgents() []*SubAgentConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	configs := make([]*SubAgentConfig, 0, len(m.configs))
	for _, cfg := range m.configs {
		configs = append(configs, cfg)
	}
	return configs
}

// Create 创建 Sub-Agent 实例（不运行）
func (m *SubAgentManager) Create(ctx context.Context, id string) (*SubAgent, error) {
	m.mu.RLock()
	cfg, ok := m.configs[id]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("sub-agent config not found: %s", id)
	}

	// TODO: 根据配置选择工具
	// 目前先创建无工具的简单 Agent
	agent, err := newSubAgent(ctx, cfg, m.defaultModel, nil)
	if err != nil {
		return nil, fmt.Errorf("create sub-agent failed: %w", err)
	}

	m.mu.Lock()
	m.agents[id] = agent
	m.mu.Unlock()

	return agent, nil
}

// Run 运行 Sub-Agent 并等待结果
func (m *SubAgentManager) Run(ctx context.Context, id string, task string) (*SubAgentResult, error) {
	agent, err := m.Create(ctx, id)
	if err != nil {
		return nil, err
	}

	// 异步执行
	errCh := make(chan error, 1)
	go func() {
		errCh <- agent.Run(ctx, task)
		close(errCh)
	}()

	// 等待执行完成或上下文取消
	select {
	case <-ctx.Done():
		agent.Cancel()
		return nil, ctx.Err()
	case err := <-errCh:
		if err != nil {
			return agent.GetResult(), err
		}
		return agent.GetResult(), nil
	}
}

// RunAsync 异步运行 Sub-Agent（不等待）
func (m *SubAgentManager) RunAsync(ctx context.Context, id string, task string) (*SubAgent, error) {
	agent, err := m.Create(ctx, id)
	if err != nil {
		return nil, err
	}

	go agent.Run(context.Background(), task)

	return agent, nil
}

// Wait 等待 Sub-Agent 执行完成
func (m *SubAgentManager) Wait(agent *SubAgent) *SubAgentResult {
	// 简单的轮询等待
	for {
		result := agent.GetResult()
		if result != nil {
			return result
		}
		// 避免 CPU 忙等待
		// 在实际中应该使用 channel 或条件变量
	}

}

// GetResult 获取 Sub-Agent 结果
func (m *SubAgentManager) GetResult(id string) *SubAgentResult {
	m.mu.RLock()
	agent, ok := m.agents[id]
	m.mu.RUnlock()

	if !ok {
		return nil
	}
	return agent.GetResult()
}

// Cancel 取消 Sub-Agent 执行
func (m *SubAgentManager) Cancel(id string) error {
	m.mu.RLock()
	agent, ok := m.agents[id]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("sub-agent not found: %s", id)
	}

	agent.Cancel()
	return nil
}

// Cleanup 清理已完成的 Sub-Agent
func (m *SubAgentManager) Cleanup(id string) {
	m.mu.Lock()
	delete(m.agents, id)
	m.mu.Unlock()
}

// Spawn 启动一个异步任务，立即返回 task_id
func (m *SubAgentManager) Spawn(ctx context.Context, agentID string, task string) (*TaskInfo, error) {
	// 检查 agent 配置是否存在
	m.mu.RLock()
	cfg, ok := m.configs[agentID]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("sub-agent config not found: %s", agentID)
	}

	// 生成 task ID
	taskID := m.NextTaskID()

	// 创建 task info
	taskInfo := &TaskInfo{
		TaskID:    taskID,
		AgentID:   agentID,
		Status:    "pending",
		StartedAt: time.Now(),
	}

	// 存储 task info
	m.taskMu.Lock()
	m.tasks[taskID] = taskInfo
	m.taskMu.Unlock()

	// 创建 agent（使用新的 context，避免被主 context 取消）
	agent, err := newSubAgent(ctx, cfg, m.defaultModel, nil)
	if err != nil {
		taskInfo.Status = "failed"
		return nil, fmt.Errorf("create sub-agent failed: %w", err)
	}

	// 存储 agent
	m.mu.Lock()
	m.agents[taskID] = agent
	m.mu.Unlock()

	// 后台执行
	taskInfo.Status = "running"
	go func() {
		logger.GetRunnerLogger().Infof("[SubAgentManager] Task %s started for agent %s", taskID, agentID)
		err := agent.Run(context.Background(), task)
		if err != nil {
			logger.GetRunnerLogger().Infof("[SubAgentManager] Task %s failed: %v", taskID, err)
			taskInfo.Status = "failed"
		} else {
			result := agent.GetResult()
			if result != nil && result.Error != "" {
				taskInfo.Status = "failed"
			} else {
				taskInfo.Status = "completed"
			}
			taskInfo.Result = result
			logger.GetRunnerLogger().Infof("[SubAgentManager] Task %s completed, output length: %d", taskID, len(result.Output))
		}
	}()

	return taskInfo, nil
}

// GetTaskStatus 获取任务状态
func (m *SubAgentManager) GetTaskStatus(taskID string) (*TaskInfo, error) {
	m.taskMu.RLock()
	taskInfo, ok := m.tasks[taskID]
	m.taskMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	return taskInfo, nil
}

// WaitTask 等待任务完成
func (m *SubAgentManager) WaitTask(taskID string, timeout time.Duration) (*TaskInfo, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		taskInfo, err := m.GetTaskStatus(taskID)
		if err != nil {
			return nil, err
		}
		if taskInfo.Status == "completed" || taskInfo.Status == "failed" || taskInfo.Status == "cancelled" {
			return taskInfo, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, fmt.Errorf("task %s timeout", taskID)
}

// CancelTask 取消任务
func (m *SubAgentManager) CancelTask(taskID string) error {
	m.taskMu.RLock()
	taskInfo, ok := m.tasks[taskID]
	m.taskMu.RUnlock()

	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}

	if taskInfo.Status != "running" {
		return fmt.Errorf("task %s is not running (status: %s)", taskID, taskInfo.Status)
	}

	m.mu.RLock()
	agent, ok := m.agents[taskID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("agent for task %s not found", taskID)
	}

	agent.Cancel()
	taskInfo.Status = "cancelled"
	return nil
}

// ListTasks 列出所有任务
func (m *SubAgentManager) ListTasks() []*TaskInfo {
	m.taskMu.RLock()
	defer m.taskMu.RUnlock()

	tasks := make([]*TaskInfo, 0, len(m.tasks))
	for _, t := range m.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}
