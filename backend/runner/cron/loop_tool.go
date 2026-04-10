package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ========== Tool Names ==========

const (
	CronCreateToolName = "cron_create"
	CronDeleteToolName = "cron_delete"
	CronListToolName   = "cron_list"
)

// ========== Tool Input/Output Types ==========

// CronCreateInput for cron_create tool
type CronCreateInput struct {
	Cron      string `json:"cron"`
	Prompt    string `json:"prompt"`
	Recurring *bool  `json:"recurring"`
	Durable   *bool  `json:"durable"`
}

// CronCreateOutput for cron_create tool
type CronCreateOutput struct {
	ID           string `json:"id"`
	HumanSchedule string `json:"human_schedule"`
	Recurring    bool   `json:"recurring"`
	Durable      bool   `json:"durable,omitempty"`
}

// CronDeleteInput for cron_delete tool
type CronDeleteInput struct {
	ID string `json:"id"`
}

// CronDeleteOutput for cron_delete tool
type CronDeleteOutput struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// CronListOutput for cron_list tool
type CronListOutput struct {
	Tasks []CronTaskInfo `json:"tasks"`
}

// CronTaskInfo represents a task in list output
type CronTaskInfo struct {
	ID           string  `json:"id"`
	Cron         string  `json:"cron"`
	HumanSchedule string  `json:"human_schedule"`
	Prompt       string  `json:"prompt"`
	Recurring    bool    `json:"recurring"`
	Durable      bool    `json:"durable"`
	CreatedAt    int64   `json:"created_at"`
	NextFireAt   *int64  `json:"next_fire_at,omitempty"`
	LastFiredAt  *int64  `json:"last_fired_at,omitempty"`
}

// ========== Validation Types ==========

// ValidationResult represents input validation result
type ValidationResult struct {
	Valid     bool
	Message   string
	ErrorCode int
}

// ========== CronCreate Tool ==========

// CronCreateTool is the tool for creating scheduled tasks
type CronCreateTool struct {
	projectDir string
}

// NewCronCreateTool creates a new CronCreateTool
func NewCronCreateTool(projectDir string) *CronCreateTool {
	return &CronCreateTool{projectDir: projectDir}
}

func (t *CronCreateTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: CronCreateToolName,
		Desc: "Schedule a prompt to run at a future time — either recurring on a cron schedule, or once at a specific time.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"cron": {
				Type:        schema.String,
				Desc:        "Standard 5-field cron expression in local time: \"M H DoM Mon DoW\" (e.g. \"*/5 * * * *\" = every 5 minutes, \"30 14 28 2 *\" = Feb 28 at 2:30pm local once).",
				Required:    true,
			},
			"prompt": {
				Type:        schema.String,
				Desc:        "The prompt to enqueue at each fire time.",
				Required:    true,
			},
			"recurring": {
				Type:        schema.Boolean,
				Desc:        "true (default) = fire on every cron match until deleted. false = fire once at the next match, then auto-delete.",
				Required:    false,
			},
			"durable": {
				Type:        schema.Boolean,
				Desc:        "true = persist to .claude/scheduled_tasks.json and survive restarts. false (default) = in-memory only, dies when this session ends.",
				Required:    false,
			},
		}),
	}, nil
}

// ValidateInput validates the input before execution
func (t *CronCreateTool) ValidateInput(ctx context.Context, argumentsInJSON string) *ValidationResult {
	var input CronCreateInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return &ValidationResult{
			Valid:     false,
			Message:   fmt.Sprintf("invalid JSON: %v", err),
			ErrorCode: 1,
		}
	}

	// Validate cron expression
	if input.Cron == "" {
		return &ValidationResult{
			Valid:     false,
			Message:   "cron expression is required",
			ErrorCode: 1,
		}
	}

	if ParseCronExpression(input.Cron) == nil {
		return &ValidationResult{
			Valid:     false,
			Message:   fmt.Sprintf("invalid cron expression '%s'. Expected 5 fields: M H DoM Mon DoW.", input.Cron),
			ErrorCode: 2,
		}
	}

	// Validate next fire time
	nextMs, err := NextCronRunMs(input.Cron, time.Now().UnixMilli())
	if err != nil || nextMs == 0 {
		return &ValidationResult{
			Valid:     false,
			Message:   fmt.Sprintf("cron expression '%s' does not match any calendar date in the next year.", input.Cron),
			ErrorCode: 3,
		}
	}

	// Check task count limit
	tasks := ListAllTasks(t.projectDir)
	if len(tasks) >= MaxJobs {
		return &ValidationResult{
			Valid:     false,
			Message:   fmt.Sprintf("too many scheduled jobs (max %d). Cancel one first.", MaxJobs),
			ErrorCode: 4,
		}
	}

	return &ValidationResult{Valid: true}
}

// InvokableRun executes the tool
func (t *CronCreateTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	validation := t.ValidateInput(ctx, argumentsInJSON)
	if !validation.Valid {
		return "", fmt.Errorf("validation failed: %s", validation.Message)
	}

	var input CronCreateInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", err
	}

	// Default recurring to true
	recurring := true
	if input.Recurring != nil {
		recurring = *input.Recurring
	}

	// Default durable to false
	durable := false
	if input.Durable != nil {
		durable = *input.Durable
	}

	// Add the task
	task, err := AddTask(input.Cron, input.Prompt, recurring, durable, "")
	if err != nil {
		return "", fmt.Errorf("add task failed: %w", err)
	}

	// Start the scheduler if not running
	scheduler := GetScheduler()
	if !scheduler.IsRunning() {
		scheduler.Start()
	}

	output := CronCreateOutput{
		ID:           task.ID,
		HumanSchedule: CronToHuman(input.Cron),
		Recurring:    recurring,
		Durable:      durable,
	}

	resultJSON, _ := json.Marshal(output)
	return string(resultJSON), nil
}

// ========== CronDelete Tool ==========

// CronDeleteTool is the tool for deleting scheduled tasks
type CronDeleteTool struct {
	projectDir string
}

// NewCronDeleteTool creates a new CronDeleteTool
func NewCronDeleteTool(projectDir string) *CronDeleteTool {
	return &CronDeleteTool{projectDir: projectDir}
}

func (t *CronDeleteTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: CronDeleteToolName,
		Desc: "Cancel a scheduled cron job by ID.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id": {
				Type:        schema.String,
				Desc:        "The job ID to cancel.",
				Required:    true,
			},
		}),
	}, nil
}

// InvokableRun executes the tool
func (t *CronDeleteTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var input CronDeleteInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", err
	}

	if input.ID == "" {
		return "", fmt.Errorf("id is required")
	}

	// Remove the task
	if err := RemoveTasks([]string{input.ID}, t.projectDir); err != nil {
		return "", fmt.Errorf("remove task failed: %w", err)
	}

	// Also remove from session
	RemoveSessionTasks([]string{input.ID})

	output := CronDeleteOutput{
		Success: true,
		Message: fmt.Sprintf("Task %s cancelled", input.ID),
	}

	resultJSON, _ := json.Marshal(output)
	return string(resultJSON), nil
}

// ========== CronList Tool ==========

// CronListTool is the tool for listing scheduled tasks
type CronListTool struct {
	projectDir string
}

// NewCronListTool creates a new CronListTool
func NewCronListTool(projectDir string) *CronListTool {
	return &CronListTool{projectDir: projectDir}
}

func (t *CronListTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: CronListToolName,
		Desc: "List all scheduled cron jobs.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	}, nil
}

// InvokableRun executes the tool
func (t *CronListTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	tasks := ListAllTasks(t.projectDir)

	taskInfos := make([]CronTaskInfo, 0, len(tasks))
	now := time.Now().UnixMilli()

	for _, task := range tasks {
		info := CronTaskInfo{
			ID:           task.ID,
			Cron:         task.Cron,
			HumanSchedule: CronToHuman(task.Cron),
			Prompt:       task.Prompt,
			Recurring:    task.Recurring,
			Durable:      task.Durable,
			CreatedAt:    task.CreatedAt,
		}

		if task.LastFiredAt != nil {
			info.LastFiredAt = task.LastFiredAt
		}

		// Compute next fire time
		nextMs, err := NextCronRunMs(task.Cron, now)
		if err == nil && nextMs > 0 {
			info.NextFireAt = &nextMs
		}

		taskInfos = append(taskInfos, info)
	}

	output := CronListOutput{Tasks: taskInfos}
	resultJSON, _ := json.Marshal(output)
	return string(resultJSON), nil
}

// ========== Tool Registration Helper ==========

// CreateLoopTools creates all loop/cron tools
func CreateLoopTools(projectDir string) []interface {
	Info(ctx context.Context) (*schema.ToolInfo, error)
	InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error)
} {
	return []interface {
		Info(ctx context.Context) (*schema.ToolInfo, error)
		InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error)
	}{
		NewCronCreateTool(projectDir),
		NewCronDeleteTool(projectDir),
		NewCronListTool(projectDir),
	}
}

// IntervalToCron converts interval notation to cron expression
// Nm (N ≤ 59)  → */N * * * *
// Nm (N ≥ 60)  → 0 */H * * *  (转为小时)
// Nh (N ≤ 23)  → 0 */N * * *
// Nd           → 0 0 */N * *
func IntervalToCron(interval string) string {
	interval = strings.TrimSpace(interval)

	if strings.HasSuffix(interval, "m") {
		// Minutes: Nm
		n := strings.TrimSuffix(interval, "m")
		return fmt.Sprintf("*/%s * * * *", n)
	}

	if strings.HasSuffix(interval, "h") {
		// Hours: Nh
		n := strings.TrimSuffix(interval, "h")
		return fmt.Sprintf("0 */%s * * *", n)
	}

	if strings.HasSuffix(interval, "d") {
		// Days: Nd
		n := strings.TrimSuffix(interval, "d")
		return fmt.Sprintf("0 0 */%s * *", n)
	}

	// Try to parse as plain number (assume minutes)
	return fmt.Sprintf("*/%s * * * *", interval)
}

// BuildLoopSkillPrompt returns the prompt for the loop skill
func BuildLoopSkillPrompt() string {
	return `Schedule a prompt to be enqueued at a future time. Use for both recurring schedules and one-shot reminders.

Uses standard 5-field cron in the user's local timezone: minute hour day-of-month month day-of-week. "0 9 * * *" means 9am local — no timezone conversion needed.

## One-shot tasks (recurring: false)
For "remind me at X" or "at <time>, do Y" requests — fire once then auto-delete.

## Recurring jobs (recurring: true, the default)
For "every N minutes" / "every hour" / "weekdays at 9am" requests:
  "*/5 * * * *" (every 5 min), "0 * * * *" (hourly), "0 9 * * 1-5" (weekdays at 9am local)

## Avoid the :00 and :30 minute marks when the task allows it
Every user who asks for "9am" gets "0 9", and every user who asks for "hourly" gets "0 *" — which means requests from across the planet land on the API at the same instant. When the user's request is approximate, pick a minute that is NOT 0 or 30.

## Durability
By default (durable: false) the job lives only in this session — nothing is written to disk, and the job is gone when the session ends. Pass durable: true to persist to .claude/scheduled_tasks.json so the job survives restarts.

## Runtime behavior
Jobs only fire while the session is idle (not mid-query). The scheduler adds a small deterministic jitter on top of whatever you pick: recurring tasks fire up to 10% of their period late (max 15 min).

Recurring tasks auto-expire after 7 days — they fire one final time, then are deleted.`
}
