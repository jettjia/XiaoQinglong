package cliext

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ========== CLI Skill Executor ==========

// ExecCLI 执行 CLI 命令
func (e *CLIExtension) ExecCLI(ctx context.Context, cliName, command, args, format string) (string, error) {
	cfg, ok := e.GetConfig(cliName)
	if !ok {
		return "", fmt.Errorf("CLI not found: %s", cliName)
	}

	// 构建完整的 CLI 命令
	cliCmd := fmt.Sprintf("%s %s", cfg.Command, command)
	if args != "" {
		cliCmd = fmt.Sprintf("%s %s", cliCmd, args)
	}

	// 添加输出格式
	if format != "" {
		cliCmd = fmt.Sprintf("%s --format %s", cliCmd, format)
	} else {
		// 默认 JSON 输出
		cliCmd = fmt.Sprintf("%s --format json", cliCmd)
	}

	// 执行命令
	output, err := e.execCommand(ctx, cfg, cliCmd, 60)
	if err != nil {
		return "", fmt.Errorf("exec %s failed: %w, output: %s", cliCmd, err, string(output))
	}

	return string(output), nil
}

// execCommand 执行命令
func (e *CLIExtension) execCommand(ctx context.Context, cfg *CLIConfig, command string, timeoutSec int) (string, error) {
	// 构建环境变量
	env := os.Environ()

	// 添加 CLI 配置目录到环境变量
	if cfg.ConfigDir != "" {
		env = append(env, fmt.Sprintf("LARK_CONFIG_DIR=%s", cfg.ConfigDir))
	}

	// 设置 HOME 为配置目录的父目录（让 CLI 在正确位置查找配置）
	if cfg.ConfigDir != "" {
		env = append(env, fmt.Sprintf("HOME=%s", substringBeforeLast(cfg.ConfigDir, "/")))
	}

	// 设置超时
	if timeoutSec <= 0 {
		timeoutSec = 60
	}

	callCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	// 执行命令
	cmd := exec.CommandContext(callCtx, "sh", "-c", command)
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	output := stdout.String()
	if err != nil {
		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
		return output, fmt.Errorf("%s", errMsg)
	}

	return output, nil
}

// substringBeforeLast 获取字符串最后一次出现分隔符之前的内容
func substringBeforeLast(s, sep string) string {
	idx := strings.LastIndex(s, sep)
	if idx == -1 {
		return s
	}
	return s[:idx]
}

// ========== Lark CLI Specific Methods ==========

// StartAuth 开始 OAuth 授权流程
func (e *CLIExtension) StartAuth(ctx context.Context, cliName string) (string, error) {
	cfg, ok := e.GetConfig(cliName)
	if !ok {
		return "", fmt.Errorf("CLI not found: %s", cliName)
	}

	if cfg.AuthType != "oauth2_device" {
		return "", fmt.Errorf("CLI %s does not support oauth2_device auth", cliName)
	}

	// 执行 lark-cli auth login --no-wait --json
	command := fmt.Sprintf("%s auth login --recommend --no-wait --json", cfg.Command)

	output, err := e.execCommand(ctx, cfg, command, 30)
	if err != nil {
		return "", fmt.Errorf("start auth failed: %w", err)
	}

	// 解析 JSON 输出
	var result struct {
		VerificationURL string `json:"verification_url"`
		DeviceCode      string `json:"device_code"`
		ExpiresIn       int    `json:"expires_in"`
		Hint            string `json:"hint"`
	}

	if err := json.Unmarshal([]byte(output), &result); err != nil {
		// 可能不是 JSON，直接返回原始输出
		return output, nil
	}

	// 返回格式化的结果
	authInfo := map[string]any{
		"cli": cliName,
		"status": "pending",
		"verification_url": result.VerificationURL,
		"device_code": result.DeviceCode,
		"expires_in": result.ExpiresIn,
		"message": "Please open the verification_url in browser and authorize",
		"complete_command": fmt.Sprintf("cli_auth {\"action\": \"complete\", \"cli\": \"%s\", \"device_code\": \"%s\"}", cliName, result.DeviceCode),
	}

	data, _ := json.Marshal(authInfo)
	return string(data), nil
}

// CompleteAuth 完成 OAuth 授权流程
func (e *CLIExtension) CompleteAuth(ctx context.Context, cliName, deviceCode string) (string, error) {
	cfg, ok := e.GetConfig(cliName)
	if !ok {
		return "", fmt.Errorf("CLI not found: %s", cliName)
	}

	// 执行 lark-cli auth login --device-code <code>
	command := fmt.Sprintf("%s auth login --device-code %s --json", cfg.Command, deviceCode)

	output, err := e.execCommand(ctx, cfg, command, 180) // OAuth 可能需要较长时间
	if err != nil {
		return "", fmt.Errorf("complete auth failed: %w", err)
	}

	// 解析输出
	var result struct {
		Event      string `json:"event"`
		UserOpenID string `json:"user_open_id"`
		UserName   string `json:"user_name"`
		Scope      string `json:"scope"`
		Error      string `json:"error"`
	}

	if err := json.Unmarshal([]byte(output), &result); err != nil {
		// 可能不是 JSON，返回原始输出
		return output, nil
	}

	if result.Error != "" {
		return "", fmt.Errorf("auth failed: %s", result.Error)
	}

	if result.Event == "authorization_complete" || result.UserOpenID != "" {
		// 授权成功，保存状态
		authInfo := map[string]any{
			"cli": cliName,
			"status": "authenticated",
			"user_open_id": result.UserOpenID,
			"user_name": result.UserName,
			"scope": result.Scope,
		}

		data, _ := json.Marshal(authInfo)
		return string(data), nil
	}

	return output, nil
}

// CheckAuthStatus 检查授权状态
func (e *CLIExtension) CheckAuthStatus(ctx context.Context, cliName string) (string, error) {
	cfg, ok := e.GetConfig(cliName)
	if !ok {
		return "", fmt.Errorf("CLI not found: %s", cliName)
	}

	// 执行 lark-cli auth status --json
	command := fmt.Sprintf("%s auth status --json", cfg.Command)

	output, err := e.execCommand(ctx, cfg, command, 30)
	if err != nil {
		// auth status 失败可能是未授权
		status := map[string]any{
			"cli": cliName,
			"authenticated": false,
			"message": err.Error(),
		}
		data, _ := json.Marshal(status)
		return string(data), nil
	}

	return output, nil
}

// ========== Helper Methods ==========

// ListSkills 列出 CLI 支持的 Skills
func (e *CLIExtension) ListSkills(cliName string) ([]string, error) {
	cfg, ok := e.GetConfig(cliName)
	if !ok {
		return nil, fmt.Errorf("CLI not found: %s", cliName)
	}

	if cfg.SkillsDir == "" {
		return nil, nil
	}

	// 列出 skills 目录下的子目录
	entries, err := os.ReadDir(cfg.SkillsDir)
	if err != nil {
		return nil, fmt.Errorf("read skills dir failed: %w", err)
	}

	var skills []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			skills = append(skills, entry.Name())
		}
	}

	return skills, nil
}
