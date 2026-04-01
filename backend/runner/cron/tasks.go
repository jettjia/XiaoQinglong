package cron

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/jettjia/XiaoQinglong/runner/pkg/logger"
)

// ========== Constants ==========

const (
	MaxJobs           = 50                      // 最大任务数限制
	DefaultMaxAgeDays = 7                       // 循环任务 7 天后自动过期
	DefaultMaxAgeMs   = 7 * 24 * 60 * 60 * 1000 // 7 days in ms
)

// ========== Cron Task Types ==========

// CronTask represents a scheduled cron task
type CronTask struct {
	ID          string `json:"id"`
	Cron        string `json:"cron"`                    // 5-field cron expression
	Prompt      string `json:"prompt"`                  // Prompt to execute when task fires
	CreatedAt   int64  `json:"created_at"`              // Epoch ms when created
	LastFiredAt *int64 `json:"last_fired_at,omitempty"` // Epoch ms of most recent fire
	Recurring   bool   `json:"recurring"`               // Whether task reschedules after firing
	Permanent   bool   `json:"permanent,omitempty"`     // Exempt from auto-expiry (system use)
	Durable     bool   `json:"-"`                       // Runtime-only: false = session-scoped
	AgentID     string `json:"agent_id,omitempty"`      // Teammate agent ID (runtime-only)
}

// CronFile represents the on-disk format for persistent tasks
type CronFile struct {
	Tasks []CronTask `json:"tasks"`
}

// ========== Task Storage ==========

var (
	// Session tasks (in-memory only)
	sessionTasks   = make(map[string]*CronTask)
	sessionTasksMu sync.RWMutex

	// File tasks (persistent)
	fileTasksMu sync.RWMutex
)

// ========== Task Storage Operations ==========

// GenerateTaskID generates a short task ID (8 hex chars)
func GenerateTaskID() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}

// AddSessionTask adds a task to the session store
func AddSessionTask(task *CronTask) error {
	sessionTasksMu.Lock()
	defer sessionTasksMu.Unlock()

	if len(sessionTasks) >= MaxJobs {
		return fmt.Errorf("too many scheduled jobs (max %d)", MaxJobs)
	}

	sessionTasks[task.ID] = task
	return nil
}

// RemoveSessionTasks removes tasks by IDs from the session store
func RemoveSessionTasks(ids []string) int {
	sessionTasksMu.Lock()
	defer sessionTasksMu.Unlock()

	removed := 0
	for _, id := range ids {
		if _, ok := sessionTasks[id]; ok {
			delete(sessionTasks, id)
			removed++
		}
	}
	return removed
}

// GetSessionTasks returns all session tasks
func GetSessionTasks() []*CronTask {
	sessionTasksMu.RLock()
	defer sessionTasksMu.RUnlock()

	tasks := make([]*CronTask, 0, len(sessionTasks))
	for _, t := range sessionTasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// ClearSessionTasks clears all session tasks
func ClearSessionTasks() {
	sessionTasksMu.Lock()
	defer sessionTasksMu.Unlock()
	sessionTasks = make(map[string]*CronTask)
}

// ========== File-based Task Storage ==========

// GetCronFilePath returns the path to the cron tasks file
func GetCronFilePath(dir string) string {
	return filepath.Join(dir, ".claude", "scheduled_tasks.json")
}

// ReadCronTasks reads tasks from the persistent file
func ReadCronTasks(dir string) ([]*CronTask, error) {
	filePath := GetCronFilePath(dir)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []*CronTask{}, nil
		}
		return nil, fmt.Errorf("read cron tasks failed: %w", err)
	}

	var file CronFile
	if err := json.Unmarshal(data, &file); err != nil {
		logger.Warnf("[CronTasks] Failed to parse cron file: %v", err)
		return []*CronTask{}, nil
	}

	// Validate and filter tasks
	validTasks := make([]*CronTask, 0, len(file.Tasks))
	for _, t := range file.Tasks {
		if t.ID == "" || t.Cron == "" || t.Prompt == "" {
			logger.Warnf("[CronTasks] Skipping malformed task: %+v", t)
			continue
		}
		if ParseCronExpression(t.Cron) == nil {
			logger.Warnf("[CronTasks] Skipping task %s with invalid cron: %s", t.ID, t.Cron)
			continue
		}
		validTasks = append(validTasks, &t)
	}

	return validTasks, nil
}

// WriteCronTasks writes tasks to the persistent file
func WriteCronTasks(tasks []*CronTask, dir string) error {
	filePath := GetCronFilePath(dir)

	// Ensure .claude directory exists
	claudeDir := filepath.Dir(filePath)
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return fmt.Errorf("create .claude dir failed: %w", err)
	}

	// Convert []*CronTask to []CronTask
	taskSlice := make([]CronTask, len(tasks))
	for i, t := range tasks {
		taskSlice[i] = *t
	}
	file := CronFile{Tasks: taskSlice}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cron tasks failed: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("write cron tasks failed: %w", err)
	}

	return nil
}

// ========== Combined Task Operations ==========

// ListAllTasks returns all tasks (both file-based and session)
func ListAllTasks(dir string) []*CronTask {
	var allTasks []*CronTask

	// Get durable (file-based) tasks
	durableTasks, err := ReadCronTasks(dir)
	if err != nil {
		logger.Warnf("[CronTasks] Failed to read durable tasks: %v", err)
	} else {
		for _, t := range durableTasks {
			t.Durable = true // Mark as durable
			allTasks = append(allTasks, t)
		}
	}

	// Get session tasks
	sessionTasksList := GetSessionTasks()
	for _, t := range sessionTasksList {
		t.Durable = false // Mark as session-only
		allTasks = append(allTasks, t)
	}

	return allTasks
}

// AddTask adds a new task
func AddTask(cron, prompt string, recurring, durable bool, agentID string) (*CronTask, error) {
	id, err := GenerateTaskID()
	if err != nil {
		return nil, fmt.Errorf("generate task ID failed: %w", err)
	}

	task := &CronTask{
		ID:        id,
		Cron:      cron,
		Prompt:    prompt,
		CreatedAt: time.Now().UnixMilli(),
		Recurring: recurring,
	}

	if agentID != "" {
		task.AgentID = agentID
	}

	if !durable {
		// Session-only task
		if err := AddSessionTask(task); err != nil {
			return nil, err
		}
		return task, nil
	}

	// Durable task - save to file
	tasks, err := ReadCronTasks("")
	if err != nil {
		return nil, fmt.Errorf("read existing tasks failed: %w", err)
	}

	if len(tasks) >= MaxJobs {
		return nil, fmt.Errorf("too many scheduled jobs (max %d)", MaxJobs)
	}

	tasks = append(tasks, task)
	if err := WriteCronTasks(tasks, ""); err != nil {
		return nil, fmt.Errorf("write task failed: %w", err)
	}

	return task, nil
}

// RemoveTasks removes tasks by IDs
func RemoveTasks(ids []string, dir string) error {
	if len(ids) == 0 {
		return nil
	}

	idSet := make(map[string]bool)
	for _, id := range ids {
		idSet[id] = true
	}

	// Remove from session store first
	RemoveSessionTasks(ids)

	// Remove from file store
	tasks, err := ReadCronTasks(dir)
	if err != nil {
		return fmt.Errorf("read tasks failed: %w", err)
	}

	remaining := make([]*CronTask, 0)
	for _, t := range tasks {
		if !idSet[t.ID] {
			remaining = append(remaining, t)
		}
	}

	if len(remaining) != len(tasks) {
		if err := WriteCronTasks(remaining, dir); err != nil {
			return fmt.Errorf("write tasks failed: %w", err)
		}
	}

	return nil
}

// MarkTasksFired updates the LastFiredAt timestamp for tasks
func MarkTasksFired(ids []string, firedAt int64, dir string) error {
	if len(ids) == 0 {
		return nil
	}

	idSet := make(map[string]bool)
	for _, id := range ids {
		idSet[id] = true
	}

	tasks, err := ReadCronTasks(dir)
	if err != nil {
		return fmt.Errorf("read tasks failed: %w", err)
	}

	changed := false
	for _, t := range tasks {
		if idSet[t.ID] {
			t.LastFiredAt = &firedAt
			changed = true
		}
	}

	if changed {
		if err := WriteCronTasks(tasks, dir); err != nil {
			return fmt.Errorf("write tasks failed: %w", err)
		}
	}

	return nil
}

// HasTasks checks if there are any persistent tasks
func HasTasks(dir string) bool {
	tasks, err := ReadCronTasks(dir)
	if err != nil {
		return false
	}
	return len(tasks) > 0
}

// ========== Task Expiry ==========

// FindExpiredTasks finds tasks that have exceeded max age
func FindExpiredTasks(tasks []*CronTask) []*CronTask {
	now := time.Now().UnixMilli()
	maxAgeMs := int64(DefaultMaxAgeMs)

	var expired []*CronTask
	for _, t := range tasks {
		if t.Permanent {
			continue // Permanent tasks never expire
		}
		if !t.Recurring {
			continue // One-shot tasks are deleted after firing, not expired
		}
		if now-t.CreatedAt > maxAgeMs {
			expired = append(expired, t)
		}
	}
	return expired
}

// CleanupExpiredTasks removes expired tasks
func CleanupExpiredTasks(dir string) error {
	tasks, err := ReadCronTasks(dir)
	if err != nil {
		return err
	}

	now := time.Now().UnixMilli()
	maxAgeMs := int64(DefaultMaxAgeMs)

	remaining := make([]*CronTask, 0)
	for _, t := range tasks {
		if t.Permanent {
			remaining = append(remaining, t)
			continue
		}
		if !t.Recurring {
			continue
		}
		if now-t.CreatedAt > maxAgeMs {
			logger.Infof("[CronTasks] Auto-expired task %s (age exceeded %d days)", t.ID, DefaultMaxAgeDays)
			continue
		}
		remaining = append(remaining, t)
	}

	if len(remaining) != len(tasks) {
		return WriteCronTasks(remaining, dir)
	}

	return nil
}
