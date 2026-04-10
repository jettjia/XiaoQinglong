package cliext

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ========== Token Manager ==========

// TokenManager 多租户 Token 管理器
type TokenManager struct {
	baseDir string

	// 内存缓存
	sessions map[string]*SessionTokens // tenantID -> tokens
	mu       sync.RWMutex
}

// SessionTokens 会话 Token 信息
type SessionTokens struct {
	CLI       string            `json:"cli"`
	AppID     string            `json:"app_id"`
	AppSecret string            `json:"app_secret"`
	Tokens    map[string]string `json:"tokens"` // userOpenId -> token JSON
	UpdatedAt int64             `json:"updated_at"`
}

// LarkToken 飞书 OAuth Token
type LarkToken struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	Scope            string `json:"scope"`
	TokenType        string `json:"token_type"`
}

// NewTokenManager 创建 Token 管理器
func NewTokenManager(baseDir string) *TokenManager {
	if baseDir == "" {
		baseDir = "/var/run/cliext"
	}
	return &TokenManager{
		baseDir: baseDir,
		sessions: make(map[string]*SessionTokens),
	}
}

// GetTokenDir 获取租户的 token 目录
func (m *TokenManager) GetTokenDir(tenantID, cliName string) string {
	return filepath.Join(m.baseDir, tenantID, cliName, "config")
}

// GetTokenDirByCLI 根据 CLI 名称获取 token 目录（单用户模式）
func (m *TokenManager) GetTokenDirByCLI(cliName string) string {
	return filepath.Join(m.baseDir, cliName, "config")
}

// SetupTenant 设置租户配置
func (m *TokenManager) SetupTenant(tenantID, cliName, appID, appSecret string) error {
	dir := m.GetTokenDir(tenantID, cliName)

	// 创建目录
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create token dir failed: %w", err)
	}

	// 保存配置
	config := map[string]string{
		"app_id":     appID,
		"app_secret": appSecret,
	}

	configPath := filepath.Join(dir, "config.json")
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config failed: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("write config failed: %w", err)
	}

	return nil
}

// SaveToken 保存 Token
func (m *TokenManager) SaveToken(tenantID, cliName, userOpenId string, token *LarkToken) error {
	dir := m.GetTokenDir(tenantID, cliName)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create token dir failed: %w", err)
	}

	tokenPath := filepath.Join(dir, fmt.Sprintf("token_%s.json", userOpenId))
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal token failed: %w", err)
	}

	if err := os.WriteFile(tokenPath, data, 0600); err != nil {
		return fmt.Errorf("write token failed: %w", err)
	}

	return nil
}

// LoadToken 加载 Token
func (m *TokenManager) LoadToken(tenantID, cliName, userOpenId string) (*LarkToken, error) {
	tokenPath := filepath.Join(m.GetTokenDir(tenantID, cliName), fmt.Sprintf("token_%s.json", userOpenId))

	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return nil, fmt.Errorf("read token failed: %w", err)
	}

	var token LarkToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("parse token failed: %w", err)
	}

	return &token, nil
}

// IsAuthenticated 检查是否已授权
func (m *TokenManager) IsAuthenticated(cliName string) bool {
	// 单用户模式：检查默认目录
	tokenDir := m.GetTokenDirByCLI(cliName)

	entries, err := os.ReadDir(tokenDir)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if entry.Name() == "config.json" {
			continue
		}
		if filepath.Ext(entry.Name()) == ".json" {
			// 有 token 文件
			tokenPath := filepath.Join(tokenDir, entry.Name())
			data, err := os.ReadFile(tokenPath)
			if err != nil {
				continue
			}

			var token LarkToken
			if err := json.Unmarshal(data, &token); err != nil {
				continue
			}

			// 检查是否过期（简单检查 access_token 是否存在）
			if token.AccessToken != "" {
				return true
			}
		}
	}

	return false
}

// AuthStatus 获取授权状态
func (m *TokenManager) AuthStatus(cliName string) string {
	if m.IsAuthenticated(cliName) {
		return "authenticated"
	}
	return "unauthenticated"
}

// StatusAll 获取所有 CLI 的授权状态
func (m *TokenManager) StatusAll() (string, error) {
	type Status struct {
		CLI        string `json:"cli"`
		AuthStatus string `json:"auth_status"`
	}

	var statuses []Status
	dirs, err := os.ReadDir(m.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return `{"clis": []}`, nil
		}
		return "", fmt.Errorf("read base dir failed: %w", err)
	}

	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}

		cliName := dir.Name()
		status := Status{
			CLI:        cliName,
			AuthStatus: m.AuthStatus(cliName),
		}
		statuses = append(statuses, status)
	}

	result := map[string]any{
		"clis": statuses,
	}

	data, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal status failed: %w", err)
	}

	return string(data), nil
}

// Logout 登出
func (m *TokenManager) Logout(cliName string) error {
	dir := m.GetTokenDirByCLI(cliName)

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read token dir failed: %w", err)
	}

	for _, entry := range entries {
		if entry.Name() == "config.json" {
			continue
		}
		if filepath.Ext(entry.Name()) == ".json" {
			tokenPath := filepath.Join(dir, entry.Name())
			if err := os.Remove(tokenPath); err != nil {
				return fmt.Errorf("remove token failed: %w", err)
			}
		}
	}

	return nil
}

// SetEnvForCLI 设置 CLI 执行所需的环境变量
func (m *TokenManager) SetEnvForCLI(cliName string) []string {
	tokenDir := m.GetTokenDirByCLI(cliName)

	return []string{
		fmt.Sprintf("LARK_CONFIG_DIR=%s", tokenDir),
	}
}
