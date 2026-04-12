package tools

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ========== Task Types ==========

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusCancelled  TaskStatus = "cancelled"
)

// Task represents a tracked task
type Task struct {
	ID          string     `json:"id"`
	Content     string     `json:"content"`
	Status      TaskStatus `json:"status"`
	Owner       string     `json:"owner,omitempty"`
	BlockedBy   []string   `json:"blocked_by,omitempty"`
	Blocking    []string   `json:"blocking,omitempty"`
	CreatedAt   int64      `json:"created_at"`
	UpdatedAt   int64      `json:"updated_at"`
	CompletedAt *int64     `json:"completed_at,omitempty"`
}

// TaskStore manages tasks in memory and optionally persists to disk
type TaskStore struct {
	mu         sync.RWMutex
	tasks      map[string]*Task
	taskDir    string
}

var (
	globalTaskStore     *TaskStore
	globalTaskStoreOnce sync.Once
)

// GetTaskStore returns the global task store
func GetTaskStore(taskDir ...string) *TaskStore {
	globalTaskStoreOnce.Do(func() {
		dir := "."
		if len(taskDir) > 0 {
			dir = taskDir[0]
		}
		globalTaskStore = &TaskStore{
			tasks:   make(map[string]*Task),
			taskDir: dir,
		}
		globalTaskStore.loadFromDisk()
	})
	return globalTaskStore
}

func (s *TaskStore) loadFromDisk() {
	taskFile := filepath.Join(s.taskDir, ".runner_tasks.json")
	data, err := os.ReadFile(taskFile)
	if err != nil {
		return
	}

	var tasks []*Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return
	}

	for _, t := range tasks {
		s.tasks[t.ID] = t
	}
}

func (s *TaskStore) saveToDisk() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		tasks = append(tasks, t)
	}

	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(filepath.Join(s.taskDir, ".runner_tasks.json"), data, 0644)
}

func (s *TaskStore) CreateTask(content, owner string, blockedBy []string) (*Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id, _ := generateTaskID()
	now := time.Now().UnixMilli()

	task := &Task{
		ID:        id,
		Content:   content,
		Status:    TaskStatusPending,
		Owner:     owner,
		BlockedBy: blockedBy,
		CreatedAt: now,
		UpdatedAt: now,
	}

	s.tasks[id] = task
	go s.saveToDisk()

	return task, nil
}

func (s *TaskStore) GetTask(id string) *Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tasks[id]
}

func (s *TaskStore) ListTasks() []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

func (s *TaskStore) UpdateTask(id string, updates map[string]interface{}) (*Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", id)
	}

	now := time.Now().UnixMilli()

	if content, ok := updates["content"].(string); ok {
		task.Content = content
	}
	if status, ok := updates["status"].(string); ok {
		task.Status = TaskStatus(status)
		if task.Status == TaskStatusCompleted {
			task.CompletedAt = &now
		}
	}
	if owner, ok := updates["owner"].(string); ok {
		task.Owner = owner
	}
	if blockedBy, ok := updates["blocked_by"].([]string); ok {
		task.BlockedBy = blockedBy
	}

	task.UpdatedAt = now
	go s.saveToDisk()

	return task, nil
}

func (s *TaskStore) DeleteTask(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.tasks[id]; !ok {
		return fmt.Errorf("task not found: %s", id)
	}

	delete(s.tasks, id)
	go s.saveToDisk()

	return nil
}

func generateTaskID() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ========== TaskCreateTool ==========

// TaskCreateInput for task create tool
type TaskCreateInput struct {
	Content   string   `json:"content"`             // Task description
	Owner     string   `json:"owner,omitempty"`     // Owner of the task
	BlockedBy []string `json:"blocked_by,omitempty"` // Task IDs this is blocked by
}

// TaskCreateTool creates a new task
type TaskCreateTool struct {
	store *TaskStore
}

func NewTaskCreateTool(taskDir ...string) *TaskCreateTool {
	return &TaskCreateTool{store: GetTaskStore(taskDir...)}
}

func (t *TaskCreateTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "TaskCreate",
		Desc: "Create a new task for tracking progress.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"content": {
				Type:        schema.String,
				Desc:        "Task description",
				Required:    true,
			},
			"owner": {
				Type:        schema.String,
				Desc:        "Owner of the task",
				Required:    false,
			},
			"blocked_by": {
				Type:        schema.Array,
				Desc:        "Task IDs this task is blocked by",
				Required:    false,
			},
		}),
	}, nil
}

func (t *TaskCreateTool) ValidateInput(ctx context.Context, input string) *ValidationResult {
	var taskInput TaskCreateInput
	if err := json.Unmarshal([]byte(input), &taskInput); err != nil {
		return &ValidationResult{Valid: false, Message: fmt.Sprintf("invalid JSON: %v", err), ErrorCode: 1}
	}
	if taskInput.Content == "" {
		return &ValidationResult{Valid: false, Message: "content is required", ErrorCode: 2}
	}
	return &ValidationResult{Valid: true}
}

func (t *TaskCreateTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var taskInput TaskCreateInput
	if err := json.Unmarshal([]byte(input), &taskInput); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	task, err := t.store.CreateTask(taskInput.Content, taskInput.Owner, taskInput.BlockedBy)
	if err != nil {
		return "", err
	}

	result, _ := json.Marshal(task)
	return string(result), nil
}

// ========== TaskGetTool ==========

// TaskGetInput for task get tool
type TaskGetInput struct {
	ID string `json:"id"` // Task ID
}

// TaskGetTool retrieves a task by ID
type TaskGetTool struct {
	store *TaskStore
}

func NewTaskGetTool(taskDir ...string) *TaskGetTool {
	return &TaskGetTool{store: GetTaskStore(taskDir...)}
}

func (t *TaskGetTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "TaskGet",
		Desc: "Retrieve a task by its ID with full details.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id": {
				Type:        schema.String,
				Desc:        "Task ID to retrieve",
				Required:    true,
			},
		}),
	}, nil
}

func (t *TaskGetTool) ValidateInput(ctx context.Context, input string) *ValidationResult {
	var taskInput TaskGetInput
	if err := json.Unmarshal([]byte(input), &taskInput); err != nil {
		return &ValidationResult{Valid: false, Message: fmt.Sprintf("invalid JSON: %v", err), ErrorCode: 1}
	}
	if taskInput.ID == "" {
		return &ValidationResult{Valid: false, Message: "id is required", ErrorCode: 2}
	}
	return &ValidationResult{Valid: true}
}

func (t *TaskGetTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var taskInput TaskGetInput
	if err := json.Unmarshal([]byte(input), &taskInput); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	task := t.store.GetTask(taskInput.ID)
	if task == nil {
		return "", fmt.Errorf("task not found: %s", taskInput.ID)
	}

	result, _ := json.Marshal(task)
	return string(result), nil
}

// ========== TaskListTool ==========

// TaskListInput for task list tool
type TaskListInput struct {
	Status string `json:"status,omitempty"` // Filter by status
	Owner  string `json:"owner,omitempty"`  // Filter by owner
}

// TaskListTool lists all tasks
type TaskListTool struct {
	store *TaskStore
}

func NewTaskListTool(taskDir ...string) *TaskListTool {
	return &TaskListTool{store: GetTaskStore(taskDir...)}
}

func (t *TaskListTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "TaskList",
		Desc: "List all tasks in the task list.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"status": {
				Type:        schema.String,
				Desc:        "Filter by status (pending, in_progress, completed, cancelled)",
				Required:    false,
			},
			"owner": {
				Type:        schema.String,
				Desc:        "Filter by owner",
				Required:    false,
			},
		}),
	}, nil
}

func (t *TaskListTool) ValidateInput(ctx context.Context, input string) *ValidationResult {
	return &ValidationResult{Valid: true}
}

func (t *TaskListTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var taskInput TaskListInput
	if input != "" {
		json.Unmarshal([]byte(input), &taskInput)
	}

	tasks := t.store.ListTasks()

	// Filter
	var filtered []*Task
	for _, task := range tasks {
		if taskInput.Status != "" && string(task.Status) != taskInput.Status {
			continue
		}
		if taskInput.Owner != "" && task.Owner != taskInput.Owner {
			continue
		}
		filtered = append(filtered, task)
	}

	type ListOutput struct {
		Tasks []*Task `json:"tasks"`
		Count int     `json:"count"`
	}

	output := ListOutput{Tasks: filtered, Count: len(filtered)}
	result, _ := json.Marshal(output)
	return string(result), nil
}

// ========== TaskUpdateTool ==========

// TaskUpdateInput for task update tool
type TaskUpdateInput struct {
	ID      string `json:"id"`                // Task ID
	Status  string `json:"status,omitempty"` // New status
	Owner   string `json:"owner,omitempty"`  // New owner
	Content string `json:"content,omitempty"` // New content
}

// TaskUpdateTool updates a task
type TaskUpdateTool struct {
	store *TaskStore
}

func NewTaskUpdateTool(taskDir ...string) *TaskUpdateTool {
	return &TaskUpdateTool{store: GetTaskStore(taskDir...)}
}

func (t *TaskUpdateTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "TaskUpdate",
		Desc: "Update a task's status, details, or ownership.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id": {
				Type:        schema.String,
				Desc:        "Task ID to update",
				Required:    true,
			},
			"status": {
				Type:        schema.String,
				Desc:        "New status (pending, in_progress, completed, cancelled)",
				Required:    false,
			},
			"owner": {
				Type:        schema.String,
				Desc:        "New owner",
				Required:    false,
			},
			"content": {
				Type:        schema.String,
				Desc:        "New task content",
				Required:    false,
			},
		}),
	}, nil
}

func (t *TaskUpdateTool) ValidateInput(ctx context.Context, input string) *ValidationResult {
	var taskInput TaskUpdateInput
	if err := json.Unmarshal([]byte(input), &taskInput); err != nil {
		return &ValidationResult{Valid: false, Message: fmt.Sprintf("invalid JSON: %v", err), ErrorCode: 1}
	}
	if taskInput.ID == "" {
		return &ValidationResult{Valid: false, Message: "id is required", ErrorCode: 2}
	}
	return &ValidationResult{Valid: true}
}

func (t *TaskUpdateTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var taskInput TaskUpdateInput
	if err := json.Unmarshal([]byte(input), &taskInput); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	updates := make(map[string]interface{})
	if taskInput.Status != "" {
		updates["status"] = taskInput.Status
	}
	if taskInput.Owner != "" {
		updates["owner"] = taskInput.Owner
	}
	if taskInput.Content != "" {
		updates["content"] = taskInput.Content
	}

	task, err := t.store.UpdateTask(taskInput.ID, updates)
	if err != nil {
		return "", err
	}

	result, _ := json.Marshal(task)
	return string(result), nil
}

// ========== TodoWriteTool ==========

// TodoInput for todo write tool
type TodoInput struct {
	Action string            `json:"action"`             // Action: "add", "done", "remove", "clear"
	Content string           `json:"content,omitempty"`  // Todo content (for add)
	Index  int               `json:"index,omitempty"`   // Index (for done/remove)
	Todos  []map[string]string `json:"todos,omitempty"`  // Full todo list (for clear)
}

// TodoItem represents a single todo item
type TodoItem struct {
	Content string `json:"content"`
	Done   bool   `json:"done"`
}

// TodoWriteTool manages a simple todo list
type TodoWriteTool struct {
	store   *TaskStore
	todos   []TodoItem
	todosMu sync.RWMutex
}

func NewTodoWriteTool(taskDir ...string) *TodoWriteTool {
	return &TodoWriteTool{
		store: GetTaskStore(taskDir...),
		todos: make([]TodoItem, 0),
	}
}

func init() {
	// Task tools - Read/Write mix
	GlobalRegistry.Register(ToolMeta{
		Name:           "TaskCreate",
		Desc:           "Create a new task for tracking progress.",
		IsReadOnly:     false,
		MaxResultChars: 500,
		DefaultRisk:    "low",
		Creator: func(basePath string) interface{} {
			return NewTaskCreateTool(basePath)
		},
	})
	GlobalRegistry.Register(ToolMeta{
		Name:           "TaskGet",
		Desc:           "Retrieve a task by its ID with full details.",
		IsReadOnly:     true,
		MaxResultChars: 1000,
		DefaultRisk:    "low",
		Creator: func(basePath string) interface{} {
			return NewTaskGetTool(basePath)
		},
	})
	GlobalRegistry.Register(ToolMeta{
		Name:           "TaskList",
		Desc:           "List all tasks in the task list.",
		IsReadOnly:     true,
		MaxResultChars: 5000,
		DefaultRisk:    "low",
		Creator: func(basePath string) interface{} {
			return NewTaskListTool(basePath)
		},
	})
	GlobalRegistry.Register(ToolMeta{
		Name:           "TaskUpdate",
		Desc:           "Update a task's status, details, or ownership.",
		IsReadOnly:     false,
		MaxResultChars: 500,
		DefaultRisk:    "low",
		Creator: func(basePath string) interface{} {
			return NewTaskUpdateTool(basePath)
		},
	})
	GlobalRegistry.Register(ToolMeta{
		Name:           "TodoWrite",
		Desc:           "Create and manage a todo list for tracking progress.",
		IsReadOnly:     false,
		MaxResultChars: 2000,
		DefaultRisk:    "low",
		Creator: func(basePath string) interface{} {
			return NewTodoWriteTool(basePath)
		},
	})
}

func (t *TodoWriteTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "TodoWrite",
		Desc: "Create and manage a todo list for tracking progress.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"action": {
				Type:        schema.String,
				Desc:        "Action: add (add todo), done (mark complete), remove (delete), clear (replace all)",
				Required:    true,
			},
			"content": {
				Type:        schema.String,
				Desc:        "Todo content (for add action)",
				Required:    false,
			},
			"index": {
				Type:        schema.Integer,
				Desc:        "Todo index (for done/remove actions, 0-indexed)",
				Required:    false,
			},
			"todos": {
				Type:        schema.Array,
				Desc:        "Full todo list (for clear action)",
				Required:    false,
			},
		}),
	}, nil
}

func (t *TodoWriteTool) ValidateInput(ctx context.Context, input string) *ValidationResult {
	var todoInput TodoInput
	if err := json.Unmarshal([]byte(input), &todoInput); err != nil {
		return &ValidationResult{Valid: false, Message: fmt.Sprintf("invalid JSON: %v", err), ErrorCode: 1}
	}
	if todoInput.Action == "" {
		return &ValidationResult{Valid: false, Message: "action is required", ErrorCode: 2}
	}
	validActions := map[string]bool{"add": true, "done": true, "remove": true, "clear": true}
	if !validActions[todoInput.Action] {
		return &ValidationResult{Valid: false, Message: "action must be one of: add, done, remove, clear", ErrorCode: 3}
	}
	return &ValidationResult{Valid: true}
}

func (t *TodoWriteTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var todoInput TodoInput
	if err := json.Unmarshal([]byte(input), &todoInput); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	t.todosMu.Lock()
	defer t.todosMu.Unlock()

	switch todoInput.Action {
	case "add":
		t.todos = append(t.todos, TodoItem{Content: todoInput.Content, Done: false})

	case "done":
		if todoInput.Index >= 0 && todoInput.Index < len(t.todos) {
			t.todos[todoInput.Index].Done = true
		}

	case "remove":
		if todoInput.Index >= 0 && todoInput.Index < len(t.todos) {
			t.todos = append(t.todos[:todoInput.Index], t.todos[todoInput.Index+1:]...)
		}

	case "clear":
		t.todos = make([]TodoItem, 0)
		for _, todo := range todoInput.Todos {
			t.todos = append(t.todos, TodoItem{
				Content: todo["content"],
				Done:    todo["done"] == "true" || todo["done"] == "1",
			})
		}
	}

	type TodoOutput struct {
		Todos []TodoItem `json:"todos"`
		Count int        `json:"count"`
	}

	output := TodoOutput{Todos: t.todos, Count: len(t.todos)}
	result, _ := json.Marshal(output)
	return string(result), nil
}
