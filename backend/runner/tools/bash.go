package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ========== BashTool ==========

// BashInput for bash tool
type BashInput struct {
	Command string `json:"command"` // Command to execute
	Timeout int    `json:"timeout,omitempty"` // Timeout in seconds (default 30)
}

// BashOutput for bash result
type BashOutput struct {
	Stdout   string `json:"stdout"`    // Standard output
	Stderr   string `json:"stderr"`    // Standard error
	ExitCode int    `json:"exit_code"`  // Exit code
	Duration int64  `json:"duration_ms"` // Execution duration in milliseconds
}

// BashTool executes bash commands
type BashTool struct {
	workingDir string
	env       map[string]string
}

func NewBashTool(workingDir string) *BashTool {
	return &BashTool{
		workingDir: workingDir,
		env:       make(map[string]string),
	}
}

func (t *BashTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "Bash",
		Desc: "Execute bash commands in the shell. Use this exclusively for system commands that cannot be done through other tools.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"command": {
				Type:        schema.String,
				Desc:        "The bash command to execute",
				Required:    true,
			},
			"timeout": {
				Type:        schema.Integer,
				Desc:        "Timeout in seconds (default 30, max 300)",
				Required:    false,
			},
		}),
	}, nil
}

func (t *BashTool) ValidateInput(ctx context.Context, input string) *ValidationResult {
	var bashInput BashInput
	if err := json.Unmarshal([]byte(input), &bashInput); err != nil {
		return &ValidationResult{Valid: false, Message: fmt.Sprintf("invalid JSON: %v", err), ErrorCode: 1}
	}
	if bashInput.Command == "" {
		return &ValidationResult{Valid: false, Message: "command is required", ErrorCode: 2}
	}
	return &ValidationResult{Valid: true}
}

func (t *BashTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var bashInput BashInput
	if err := json.Unmarshal([]byte(input), &bashInput); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	timeout := 30
	if bashInput.Timeout > 0 && bashInput.Timeout <= 300 {
		timeout = bashInput.Timeout
	}

	// Create context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	// Build command
	var cmd *exec.Cmd
	if strings.Contains(bashInput.Command, "\n") {
		// Multi-line command: use bash -c
		cmd = exec.CommandContext(cmdCtx, "bash", "-c", bashInput.Command)
	} else {
		// Single command
		cmd = exec.CommandContext(cmdCtx, "bash", "-c", bashInput.Command)
	}

	// Set working directory
	if t.workingDir != "" {
		cmd.Dir = t.workingDir
	}

	// Set environment
	cmd.Env = osEnvironToMap()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start).Milliseconds()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	output := BashOutput{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Duration: duration,
	}

	result, _ := json.Marshal(output)
	return string(result), nil
}

func osEnvironToMap() []string {
	return append([]string{}, os.Environ()...)
}

// Getenv gets an environment variable
func (t *BashTool) Getenv(key string) string {
	if v, ok := t.env[key]; ok {
		return v
	}
	return ""
}

// Setenv sets an environment variable
func (t *BashTool) Setenv(key, value string) {
	t.env[key] = value
}
