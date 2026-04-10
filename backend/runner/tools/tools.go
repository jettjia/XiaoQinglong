package tools

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ToolInterface is the interface all tools must implement
type ToolInterface interface {
	Info(ctx context.Context) (*schema.ToolInfo, error)
	InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error)
}

// AllTools returns all available built-in tools
func AllTools() []ToolInterface {
	return []ToolInterface{
		NewGlobTool(),
		NewGrepTool(),
		NewFileReadTool(),
		NewFileEditTool(),
		NewFileWriteTool(),
		NewBashTool("."),
		NewSleepTool(),
		NewWebFetchTool(),
		NewWebSearchTool(),
		NewTaskCreateTool(),
		NewTaskGetTool(),
		NewTaskListTool(),
		NewTaskUpdateTool(),
		NewTodoWriteTool(),
		NewEnterPlanModeTool(),
		NewExitPlanModeTool(),
		NewAskUserQuestionTool(),
	}
}

// ToolNames returns the names of all available tools
func ToolNames() []string {
	return []string{
		"Glob",
		"Grep",
		"Read",
		"Edit",
		"Write",
		"Bash",
		"Sleep",
		"WebFetch",
		"WebSearch",
		"TaskCreate",
		"TaskGet",
		"TaskList",
		"TaskUpdate",
		"TodoWrite",
		"EnterPlanMode",
		"ExitPlanMode",
		"AskUserQuestion",
	}
}
