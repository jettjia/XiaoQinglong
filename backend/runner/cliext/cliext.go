package cliext

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ========== CLI Extension Types ==========

// CLIConfig CLI 工具配置
type CLIConfig struct {
	// Name CLI 名称，如 "lark", "dingtalk"
	Name string `json:"name"`
	// Command CLI 命令，如 "lark-cli", "dingtalk"
	Command string `json:"command"`
	// ConfigDir token 配置目录
	ConfigDir string `json:"config_dir"`
	// SkillsDir skills 文件目录
	SkillsDir string `json:"skills_dir"`
	// RiskLevel 风险级别
	RiskLevel string `json:"risk_level"`
	// AuthType 授权类型：none, oauth2_device, oauth2_browser
	AuthType string `json:"auth_type"`
}

// CLIExtension CLI 扩展管理器
type CLIExtension struct {
	configs map[string]*CLIConfig // name -> config
	tokens  *TokenManager

	mu sync.RWMutex
}

// NewCLIExtension 创建 CLI 扩展管理器
func NewCLIExtension(baseDir string) *CLIExtension {
	return &CLIExtension{
		configs: make(map[string]*CLIConfig),
		tokens:  NewTokenManager(baseDir),
	}
}

// Register 注册一个 CLI 配置
func (e *CLIExtension) Register(cfg CLIConfig) error {
	if cfg.Name == "" {
		return fmt.Errorf("CLI name is required")
	}
	if cfg.Command == "" {
		return fmt.Errorf("CLI command is required")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.configs[cfg.Name] = &cfg
	return nil
}

// GetConfig 获取 CLI 配置
func (e *CLIExtension) GetConfig(name string) (*CLIConfig, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	cfg, ok := e.configs[name]
	return cfg, ok
}

// ListConfigs 列出所有 CLI 配置
func (e *CLIExtension) ListConfigs() []*CLIConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*CLIConfig, 0, len(e.configs))
	for _, cfg := range e.configs {
		result = append(result, cfg)
	}
	return result
}

// ========== CLI Tool for Agent ==========

// CLITool CLI 执行工具（暴露给 Agent）
type CLITool struct {
	ext    *CLIExtension
	name   string
	config *CLIConfig
}

// NewCLITool 创建 CLI 工具
func NewCLITool(ext *CLIExtension, name string) *CLITool {
	return &CLITool{
		ext:  ext,
		name: name,
	}
}

func (t *CLITool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	cfg, ok := t.ext.GetConfig(t.name)
	if !ok {
		return nil, fmt.Errorf("CLI not found: %s", t.name)
	}

	return &schema.ToolInfo{
		Name: fmt.Sprintf("cli_%s", t.name),
		Desc: fmt.Sprintf("Execute %s CLI commands. Auth status: %s",
			cfg.Command, t.ext.tokens.AuthStatus(t.name)),
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"command": {
				Type:     schema.String,
				Desc:     fmt.Sprintf("CLI command to execute (without '%s' prefix)", cfg.Command),
				Required: true,
			},
			"args": {
				Type:     schema.String,
				Desc:     "Additional arguments as JSON string",
				Required: false,
			},
			"format": {
				Type:     schema.String,
				Desc:     "Output format: json, pretty, table",
				Required: false,
			},
		}),
	}, nil
}

func (t *CLITool) ValidateInput(ctx context.Context, input string) *ValidationResult {
	type cliInput struct {
		Command string `json:"command"`
		Args    string `json:"args"`
		Format  string `json:"format"`
	}

	var in cliInput
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return &ValidationResult{Valid: false, Message: fmt.Sprintf("invalid JSON: %v", err), ErrorCode: 1}
	}

	if in.Command == "" {
		return &ValidationResult{Valid: false, Message: "command is required", ErrorCode: 2}
	}

	return &ValidationResult{Valid: true}
}

func (t *CLITool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	_, ok := t.ext.GetConfig(t.name)
	if !ok {
		return "", fmt.Errorf("CLI not found: %s", t.name)
	}

	type cliInput struct {
		Command string `json:"command"`
		Args    string `json:"args"`
		Format  string `json:"format"`
	}
	var in cliInput
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	// 检查授权状态
	if !t.ext.tokens.IsAuthenticated(t.name) {
		return "", fmt.Errorf("CLI %s not authenticated. Please run 'cli_%s auth' first", t.name, t.name)
	}

	// 执行 CLI 命令
	result, err := t.ext.ExecCLI(ctx, t.name, in.Command, in.Args, in.Format)
	if err != nil {
		return "", err
	}

	return result, nil
}

// ========== Auth Tool ==========

// CLIAuthTool CLI 授权工具
type CLIAuthTool struct {
	ext *CLIExtension
}

// NewCLIAuthTool 创建授权工具
func NewCLIAuthTool(ext *CLIExtension) *CLIAuthTool {
	return &CLIAuthTool{ext: ext}
}

func (t *CLIAuthTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "cli_auth",
		Desc: "Manage CLI authentication/authorization",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"action": {
				Type:     schema.String,
				Desc:     "Action: status, start, complete, logout",
				Required: true,
			},
			"cli": {
				Type:     schema.String,
				Desc:     "CLI name (e.g., lark, dingtalk)",
				Required: false,
			},
			"device_code": {
				Type:     schema.String,
				Desc:     "Device code (for complete action)",
				Required: false,
			},
		}),
	}, nil
}

func (t *CLIAuthTool) ValidateInput(ctx context.Context, input string) *ValidationResult {
	type authInput struct {
		Action     string `json:"action"`
		CLI        string `json:"cli"`
		DeviceCode string `json:"device_code"`
	}

	var in authInput
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return &ValidationResult{Valid: false, Message: fmt.Sprintf("invalid JSON: %v", err), ErrorCode: 1}
	}

	validActions := map[string]bool{"status": true, "start": true, "complete": true, "logout": true}
	if !validActions[in.Action] {
		return &ValidationResult{Valid: false, Message: "action must be one of: status, start, complete, logout", ErrorCode: 2}
	}

	if in.Action != "status" && in.Action != "logout" && in.CLI == "" {
		return &ValidationResult{Valid: false, Message: "cli name is required for this action", ErrorCode: 3}
	}

	return &ValidationResult{Valid: true}
}

func (t *CLIAuthTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	type authInput struct {
		Action     string `json:"action"`
		CLI        string `json:"cli"`
		DeviceCode string `json:"device_code"`
	}

	var in authInput
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	switch in.Action {
	case "status":
		return t.ext.tokens.StatusAll()

	case "start":
		_, ok := t.ext.GetConfig(in.CLI)
		if !ok {
			return "", fmt.Errorf("CLI not found: %s", in.CLI)
		}
		return t.ext.StartAuth(ctx, in.CLI)

	case "complete":
		if in.DeviceCode == "" {
			return "", fmt.Errorf("device_code is required for complete action")
		}
		return t.ext.CompleteAuth(ctx, in.CLI, in.DeviceCode)

	case "logout":
		return "", t.ext.tokens.Logout(in.CLI)
	}

	return "", fmt.Errorf("unknown action: %s", in.Action)
}

// ========== Validation Result ==========

// ValidationResult 验证结果
type ValidationResult struct {
	Valid     bool
	Message   string
	ErrorCode int
}

// ========== CLI Status ==========

// CLIStatus CLI 授权状态
type CLIStatus struct {
	Name       string `json:"name"`
	Command    string `json:"command"`
	AuthStatus string `json:"auth_status"` // unknown, unauthenticated, authenticating, authenticated
	Message    string `json:"message,omitempty"`
}
