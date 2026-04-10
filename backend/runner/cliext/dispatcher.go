package cliext

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/jettjia/XiaoQinglong/runner/types"
)

// ========== Dispatcher Integration ==========

// RegisterToDispatcher 将 CLI 工具注册到 Dispatcher
func (e *CLIExtension) RegisterToDispatcher(d DispatcherInterface) error {
	// 注册每个 CLI 工具
	for _, cfg := range e.ListConfigs() {
		cliTool := NewCLITool(e, cfg.Name)

		// 根据 risk level 决定是否需要包装审批
		riskLevel := cfg.RiskLevel
		if riskLevel == "" {
			riskLevel = "medium"
		}

		wrapped := d.WrapToolWithApproval(cliTool, cfg.Name, "cli", riskLevel)
		d.AddTool(wrapped)
	}

	return nil
}

// DispatcherInterface Dispatcher 需要实现的接口
type DispatcherInterface interface {
	WrapToolWithApproval(t tool.BaseTool, name, toolType, riskLevel string) tool.BaseTool
	AddTool(t tool.BaseTool)
}

// ========== Types for Dispatcher ==========

// CLIConfigRequest CLI 配置请求（用于 RunRequest）
type CLIConfigRequest struct {
	// Name CLI 名称，如 "lark"
	Name string `json:"name"`
	// Command CLI 命令，如 "lark-cli"
	Command string `json:"command"`
	// ConfigDir token 配置目录
	ConfigDir string `json:"config_dir"`
	// SkillsDir skills 文件目录
	SkillsDir string `json:"skills_dir"`
	// RiskLevel 风险级别
	RiskLevel string `json:"risk_level"`
	// AuthType 授权类型
	AuthType string `json:"auth_type"`
	// AppID 飞书应用 App ID（可选，用于初始化配置）
	AppID string `json:"app_id"`
	// AppSecret 飞书应用 App Secret（可选）
	AppSecret string `json:"app_secret"`
}

// ToCLIConfig 转换为 CLIConfig
func (r *CLIConfigRequest) ToCLIConfig() CLIConfig {
	return CLIConfig{
		Name:      r.Name,
		Command:   r.Command,
		ConfigDir: r.ConfigDir,
		SkillsDir: r.SkillsDir,
		RiskLevel: r.RiskLevel,
		AuthType:  r.AuthType,
	}
}

// InitFromRunRequest 从 RunRequest 初始化 CLI 扩展
func (e *CLIExtension) InitFromRunRequest(ctx context.Context, clis []CLIConfigRequest) error {
	for _, cliReq := range clis {
		cfg := cliReq.ToCLIConfig()

		// 注册 CLI 配置
		if err := e.Register(cfg); err != nil {
			return err
		}

		// 如果提供了 AppID 和 AppSecret，设置租户配置
		if cliReq.AppID != "" && cliReq.AppSecret != "" {
			// 使用 "default" 作为默认租户 ID
			if err := e.tokens.SetupTenant("default", cfg.Name, cliReq.AppID, cliReq.AppSecret); err != nil {
				return err
			}
		}
	}

	return nil
}

// ConvertToTypesCLI 转换为 types.CLIConfig（如果 runner 需要）
func (r *CLIConfigRequest) ConvertToTypesCLI() types.CLIConfig {
	return types.CLIConfig{
		Name:      r.Name,
		Command:   r.Command,
		ConfigDir: r.ConfigDir,
		SkillsDir: r.SkillsDir,
		RiskLevel: r.RiskLevel,
		AuthType:  r.AuthType,
	}
}
