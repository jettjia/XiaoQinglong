package job

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	entityJob "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/job"
	srvJob "github.com/jettjia/xiaoqinglong/agent-frame/domain/srv/job"
)

var (
	globalJobManager *JobManager
	once             sync.Once
	jobManagerReady  = make(chan struct{})
)

// JobManager 周期任务管理器
type JobManager struct {
	cron            *cron.Cron
	runnerURL       string
	jobExecutionSvc *srvJob.JobExecution
	maxKeepCount    int

	// agent sessions (agent_id -> session_id)
	agentSessions   map[string]string
	agentSessionsMu sync.RWMutex

	// cron entry IDs (agent_id -> cron.EntryID)
	cronEntries   map[string]cron.EntryID
	cronEntriesMu sync.RWMutex

	// paused agents (agent_id -> true if paused)
	pausedAgents   map[string]bool
	pausedAgentsMu sync.RWMutex

	shutdown chan struct{}
	wg       sync.WaitGroup
}

// InitJobManager 初始化 JobManager (单例)
func InitJobManager(runnerURL string, maxKeepCount int) *JobManager {
	once.Do(func() {
		globalJobManager = &JobManager{
			cron:            cron.New(cron.WithSeconds()),
			runnerURL:       runnerURL,
			jobExecutionSvc: srvJob.NewJobExecutionSvc(),
			maxKeepCount:    maxKeepCount,
			agentSessions:   make(map[string]string),
			cronEntries:     make(map[string]cron.EntryID),
			pausedAgents:    make(map[string]bool),
			shutdown:        make(chan struct{}),
		}

		globalJobManager.cron.Start()
		log.Printf("[JobManager] Started")
		close(jobManagerReady)
	})

	return globalJobManager
}

// GetJobManager 获取全局 JobManager 实例
func GetJobManager() *JobManager {
	return globalJobManager
}

// WaitForReady 等待 JobManager 初始化完成
func WaitForReady() {
	<-jobManagerReady
}

// AddCronJob 添加周期任务
func (m *JobManager) AddCronJob(agentId, agentName, cronRule, configJson string) error {
	if cronRule == "" {
		return fmt.Errorf("cron rule is empty")
	}

	// 统一转为 6 字段 cron 表达式（robfig/cron 默认支持秒字段）
	cronRule = normalizeCronRule(cronRule)

	// 验证 cron 表达式
	specParser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	if _, err := specParser.Parse(cronRule); err != nil {
		return fmt.Errorf("invalid cron rule: %w", err)
	}

	// 检查是否已存在
	m.cronEntriesMu.Lock()
	if _, exists := m.cronEntries[agentId]; exists {
		m.cronEntriesMu.Unlock()
		return fmt.Errorf("cron job already exists for agent: %s", agentId)
	}

	// 创建 cron job
	entryID, err := m.cron.AddFunc(cronRule, func() {
		m.executeAgentJob(agentId, agentName, configJson)
	})
	if err != nil {
		m.cronEntriesMu.Unlock()
		return fmt.Errorf("failed to add cron job: %w", err)
	}

	m.cronEntries[agentId] = entryID
	m.cronEntriesMu.Unlock()

	log.Printf("[JobManager] Added cron job for agent %s (entry_id: %d), cron: %s", agentId, entryID, cronRule)
	return nil
}

// RemoveCronJob 移除周期任务
func (m *JobManager) RemoveCronJob(agentId string) error {
	m.cronEntriesMu.Lock()
	defer m.cronEntriesMu.Unlock()

	entryID, exists := m.cronEntries[agentId]
	if !exists {
		return fmt.Errorf("cron job not found for agent: %s", agentId)
	}

	m.cron.Remove(entryID)
	delete(m.cronEntries, agentId)

	// 清除 session
	m.agentSessionsMu.Lock()
	delete(m.agentSessions, agentId)
	m.agentSessionsMu.Unlock()

	// 清除暂停状态
	m.pausedAgentsMu.Lock()
	delete(m.pausedAgents, agentId)
	m.pausedAgentsMu.Unlock()

	log.Printf("[JobManager] Removed cron job for agent %s", agentId)
	return nil
}

// UpdateCronJob 更新周期任务
func (m *JobManager) UpdateCronJob(agentId, agentName, cronRule, configJson string) error {
	// 先移除旧的
	if err := m.RemoveCronJob(agentId); err != nil {
		log.Printf("[JobManager] UpdateCronJob (remove): %v", err)
	}

	// 如果仍然有 cron rule，添加新的
	if cronRule != "" {
		return m.AddCronJob(agentId, agentName, cronRule, configJson)
	}

	return nil
}

// PauseCronJob 暂停周期任务
func (m *JobManager) PauseCronJob(agentId string) error {
	m.cronEntriesMu.RLock()
	_, exists := m.cronEntries[agentId]
	m.cronEntriesMu.RUnlock()
	if !exists {
		return fmt.Errorf("cron job not found for agent: %s", agentId)
	}

	m.pausedAgentsMu.Lock()
	m.pausedAgents[agentId] = true
	m.pausedAgentsMu.Unlock()

	log.Printf("[JobManager] Paused cron job for agent %s", agentId)
	return nil
}

// ResumeCronJob 恢复周期任务
func (m *JobManager) ResumeCronJob(agentId string) error {
	m.cronEntriesMu.RLock()
	_, exists := m.cronEntries[agentId]
	m.cronEntriesMu.RUnlock()
	if !exists {
		return fmt.Errorf("cron job not found for agent: %s", agentId)
	}

	m.pausedAgentsMu.Lock()
	delete(m.pausedAgents, agentId)
	m.pausedAgentsMu.Unlock()

	log.Printf("[JobManager] Resumed cron job for agent %s", agentId)
	return nil
}

// IsCronJobPaused 检查周期任务是否暂停
func (m *JobManager) IsCronJobPaused(agentId string) bool {
	m.pausedAgentsMu.RLock()
	defer m.pausedAgentsMu.RUnlock()
	return m.pausedAgents[agentId]
}

// executeAgentJob 执行周期任务
func (m *JobManager) executeAgentJob(agentId, agentName, configJson string) {
	m.wg.Add(1)
	defer m.wg.Done()

	// 检查是否暂停
	if m.IsCronJobPaused(agentId) {
		log.Printf("[JobManager] Agent %s is paused, skipping execution", agentId)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// 获取或创建 session
	sessionId := m.getAgentSession(agentId)
	if sessionId == "" {
		sessionId = m.createAgentSession(agentId, agentName)
		if sessionId == "" {
			log.Printf("[JobManager] Failed to create session for agent %s", agentId)
			return
		}
		m.setAgentSession(agentId, sessionId)
	}

	// 创建执行记录
	execution := &entityJob.JobExecution{}
	execution.AgentId = agentId
	execution.AgentName = agentName
	execution.SessionId = sessionId
	execution.Status = entityJob.JobStatusRunning
	execution.TriggerTime = time.Now().UnixMilli()
	execution.StartedAt = time.Now().UnixMilli()
	execution.InputSummary = "Periodic task triggered by cron"

	ulid, err := m.jobExecutionSvc.CreateJobExecution(ctx, execution)
	if err != nil {
		log.Printf("[JobManager] Failed to create execution record: %v", err)
		return
	}
	execution.Ulid = ulid

	startTime := time.Now()

	// 调用 runner 执行
	result, err := m.callRunner(ctx, agentId, sessionId, configJson)
	latencyMs := time.Since(startTime).Milliseconds()

	if err != nil {
		failedJob := &entityJob.JobExecution{}
		failedJob.Ulid = ulid
		failedJob.Status = entityJob.JobStatusFailed
		failedJob.FinishedAt = time.Now().UnixMilli()
		failedJob.ErrorMsg = err.Error()
		failedJob.LatencyMs = latencyMs
		m.jobExecutionSvc.UpdateJobExecution(ctx, failedJob)
		m.cleanupOldRecords(ctx, agentId)
		log.Printf("[JobManager] Agent %s execution failed: %v", agentId, err)
		return
	}

	// 更新执行记录
	successJob := &entityJob.JobExecution{}
	successJob.Ulid = ulid
	successJob.Status = entityJob.JobStatusSuccess
	successJob.FinishedAt = time.Now().UnixMilli()
	successJob.OutputSummary = truncateString(result.Content, 500)
	successJob.OutputFull = result.Content
	successJob.TokensUsed = result.TokensUsed
	successJob.LatencyMs = latencyMs
	m.jobExecutionSvc.UpdateJobExecution(ctx, successJob)
	m.cleanupOldRecords(ctx, agentId)

	log.Printf("[JobManager] Agent %s executed successfully, tokens: %d, latency: %dms",
		agentId, result.TokensUsed, latencyMs)
}

// callRunner 调用 runner 服务执行 agent
func (m *JobManager) callRunner(ctx context.Context, agentId, sessionId, configJson string) (*runnerResult, error) {
	// 构建 runner 请求
	runnerReq := map[string]any{
		"agent_id":   agentId,
		"user_id":    "system",
		"session_id": sessionId,
		"input":      fmt.Sprintf("Periodic task execution for agent: %s", agentId),
		"is_test":    false,
	}

	// 解析 agent config 获取其他配置
	if configJson != "" {
		var config map[string]any
		if err := json.Unmarshal([]byte(configJson), &config); err == nil {
			if models, ok := config["models"].(map[string]any); ok {
				runnerReq["models"] = models
			}
			if prompt, ok := config["prompt"].(string); ok {
				runnerReq["prompt"] = prompt
			}
			if tools, ok := config["tools"].([]any); ok {
				runnerReq["tools"] = tools
			}
			if skills, ok := config["skills"].([]any); ok {
				runnerReq["skills"] = skills
			}
			if knowledge, ok := config["knowledge"].([]any); ok {
				runnerReq["knowledge"] = knowledge
			}
			if mcps, ok := config["mcps"].([]any); ok {
				runnerReq["mcps"] = mcps
			}
			if a2a, ok := config["a2a"].([]any); ok {
				runnerReq["a2a"] = a2a
			}
			if sandbox, ok := config["sandbox"].(map[string]any); ok {
				runnerReq["sandbox"] = sandbox
			}
		}
	}

	body, err := json.Marshal(runnerReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal runner request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", m.runnerURL+"/run", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call runner: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("runner returned status: %d", resp.StatusCode)
	}

	var result struct {
		Content    string `json:"content"`
		TokensUsed int    `json:"tokens_used"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &runnerResult{
		Content:    result.Content,
		TokensUsed: result.TokensUsed,
	}, nil
}

type runnerResult struct {
	Content    string
	TokensUsed int
}

// getAgentSession 获取 agent 的 session
func (m *JobManager) getAgentSession(agentId string) string {
	m.agentSessionsMu.RLock()
	defer m.agentSessionsMu.RUnlock()
	return m.agentSessions[agentId]
}

// setAgentSession 设置 agent 的 session
func (m *JobManager) setAgentSession(agentId, sessionId string) {
	m.agentSessionsMu.Lock()
	defer m.agentSessionsMu.Unlock()
	m.agentSessions[agentId] = sessionId
}

// createAgentSession 为 agent 创建 session
func (m *JobManager) createAgentSession(agentId, _ string) string {
	// 临时方案：使用 agentId + 时间戳作为 sessionId
	// TODO: 后续调用 chat service 创建真正的 session
	sessionId := fmt.Sprintf("cron-%s-%d", agentId, time.Now().UnixMilli())
	log.Printf("[JobManager] Created temporary session %s for agent %s", sessionId, agentId)
	return sessionId
}

// cleanupOldRecords 清理旧记录
func (m *JobManager) cleanupOldRecords(ctx context.Context, agentId string) {
	count, err := m.jobExecutionSvc.CountByAgentId(ctx, agentId)
	if err != nil {
		log.Printf("[JobManager] cleanupOldRecords count error: %v", err)
		return
	}

	if count > m.maxKeepCount {
		if err := m.jobExecutionSvc.DeleteOldByAgentId(ctx, agentId, m.maxKeepCount); err != nil {
			log.Printf("[JobManager] cleanupOldRecords delete error: %v", err)
		}
	}
}

// truncateString 截断字符串
// normalizeCronRule 将 5 字段 cron 转为 6 字段（robfig/cron 使用秒字段）
func normalizeCronRule(rule string) string {
	fields := strings.Fields(rule)
	if len(fields) == 5 {
		return "0 " + rule // 补全秒字段
	}
	return rule
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// Stop 停止 JobManager
func (m *JobManager) Stop() {
	close(m.shutdown)
	m.cron.Stop()
	m.wg.Wait()
	log.Printf("[JobManager] Stopped")
}

// PeriodicAgent 周期任务Agent的结构（用于从DB同步）
type PeriodicAgent struct {
	Ulid       string
	Name       string
	CronRule   string
	ConfigJson string
	Enabled    bool
}

// InitJob 初始化 JobManager (兼容 main.go 调用)
func InitJob(shutdown chan struct{}) {
	// runner URL 和 maxKeepCount 需要从配置获取，这里使用默认值
	runnerURL := "http://localhost:18080"
	maxKeepCount := 100

	jm := InitJobManager(runnerURL, maxKeepCount)
	log.Printf("[Job] JobManager initialized, runnerURL: %s", runnerURL)

	// 监听 shutdown 信号
	go func() {
		<-shutdown
		log.Printf("[Job] Received shutdown signal, stopping JobManager...")
		jm.Stop()
	}()
}

// SyncCronJobsFromDB 从数据库同步周期任务（服务启动时调用）
func SyncCronJobsFromDB(agents []PeriodicAgent) {
	// 等待 JobManager 初始化完成
	WaitForReady()

	if len(agents) == 0 {
		return
	}

	jm := GetJobManager()
	if jm == nil {
		log.Printf("[Job] SyncCronJobsFromDB: JobManager is nil")
		return
	}

	for _, ag := range agents {
		if ag.CronRule == "" {
			continue
		}
		if err := jm.AddCronJob(ag.Ulid, ag.Name, ag.CronRule, ag.ConfigJson); err != nil {
			log.Printf("[Job] SyncCronJobsFromDB: failed to add cron job for agent %s: %v", ag.Ulid, err)
			continue
		}
		// 如果该 Agent 被停用了，暂停其 cron job
		if !ag.Enabled {
			_ = jm.PauseCronJob(ag.Ulid)
		}
		log.Printf("[Job] SyncCronJobsFromDB: registered cron job for agent %s", ag.Ulid)
	}
}
