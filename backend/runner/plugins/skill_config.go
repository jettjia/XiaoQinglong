package plugins

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ========== Skill Configuration ==========

// SkillConfigManager 管理 skill 的全局配置
type SkillConfigManager struct {
	configs    map[string]map[string]string // skillName -> configKey -> configValue
	configPath string
}

// NewSkillConfigManager 创建配置管理器
func NewSkillConfigManager(configPath string) (*SkillConfigManager, error) {
	mgr := &SkillConfigManager{
		configs:    make(map[string]map[string]string),
		configPath: configPath,
	}

	if configPath != "" {
		if err := mgr.Load(configPath); err != nil {
			return nil, fmt.Errorf("load skill config failed: %w", err)
		}
	}

	return mgr, nil
}

// Load 从文件加载配置
func (m *SkillConfigManager) Load(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 配置文件不存在，使用默认空配置
		}
		return fmt.Errorf("read config file failed: %w", err)
	}

	return m.Parse(data)
}

// Parse 解析配置数据
func (m *SkillConfigManager) Parse(data []byte) error {
	// 支持两种格式：
	// 1. 全局配置：key: value
	// 2. 按 skill 分组：skillName.key: value
	// 3. 嵌套格式：skills.skillName.env.KEY: value

	type rawConfig map[string]any

	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse yaml failed: %w", err)
	}

	// 解析配置
	for k, v := range raw {
		if strings.Contains(k, ".") {
			// skill.key 格式
			parts := strings.SplitN(k, ".", 2)
			skillName := parts[0]
			key := parts[1]

			// 检查是否是嵌套的 skills 格式：skills.skillName.env.KEY
			if skillName == "skills" {
				if m.configs[key] == nil {
					m.configs[key] = make(map[string]string)
				}
				if envMap, ok := v.(map[string]any); ok {
					for envK, envV := range envMap {
						m.configs[key][envK] = toString(envV)
					}
				}
			} else {
				if m.configs[skillName] == nil {
					m.configs[skillName] = make(map[string]string)
				}
				m.configs[skillName][key] = toString(v)
			}
		} else {
			// 全局配置
			if m.configs["_global"] == nil {
				m.configs["_global"] = make(map[string]string)
			}
			m.configs["_global"][k] = toString(v)
		}
	}

	return nil
}

// Get 获取 skill 的配置
func (m *SkillConfigManager) Get(skillName string, key string) string {
	// 先查找 skill 专用配置
	if cfg, ok := m.configs[skillName]; ok {
		if v, ok := cfg[key]; ok {
			return v
		}
	}
	// 再查找全局配置
	if globalCfg, ok := m.configs["_global"]; ok {
		if v, ok := globalCfg[key]; ok {
			return v
		}
	}
	return ""
}

// GetAll 获取 skill 的所有配置
func (m *SkillConfigManager) GetAll(skillName string) map[string]string {
	result := make(map[string]string)

	// 先复制全局配置
	if globalCfg, ok := m.configs["_global"]; ok {
		for k, v := range globalCfg {
			result[k] = v
		}
	}

	// 再覆盖 skill 专用配置
	if skillCfg, ok := m.configs[skillName]; ok {
		for k, v := range skillCfg {
			result[k] = v
		}
	}

	return result
}

// ToEnvVars 转换为环境变量格式
func (m *SkillConfigManager) ToEnvVars(skillName string) map[string]string {
	config := m.GetAll(skillName)
	envVars := make(map[string]string)

	for k, v := range config {
		// 转换为大写，替换特殊字符
		envKey := strings.ToUpper(k)
		envKey = strings.ReplaceAll(envKey, ".", "_")
		envKey = strings.ReplaceAll(envKey, "-", "_")
		envVars[envKey] = v
	}

	return envVars
}

// Set 设置配置（用于运行时修改）
func (m *SkillConfigManager) Set(skillName, key, value string) {
	if m.configs[skillName] == nil {
		m.configs[skillName] = make(map[string]string)
	}
	m.configs[skillName][key] = value
}

// Merge 合并配置
func (m *SkillConfigManager) Merge(skillName string, overrides map[string]string) {
	for k, v := range overrides {
		m.Set(skillName, k, v)
	}
}

// toString 将任意类型转换为字符串
func toString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case int:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%f", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", val)
	}
}

// ========== Default Config File Path ==========

// DefaultSkillConfigPath 返回默认的 skill 配置路径
func DefaultSkillConfigPath() string {
	// 依次查找：
	// 1. ./dev-skills-config.yaml (开发环境配置，优先)
	// 2. ./skills-config.yaml (默认配置)
	// 3. ./config/skills-config.yaml
	// 4. ~/.xiaoqinglong/config/skills-config.yaml (用户配置目录)
	paths := []string{
		"dev-skills-config.yaml",
		"skills-config.yaml",
		"config/skills-config.yaml",
		filepath.Join(os.Getenv("HOME"), ".xiaoqinglong", "config", "skills-config.yaml"),
		filepath.Join(os.Getenv("HOME"), ".skills-config.yaml"),
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return "" // 没有找到，返回空
}

// ========== Skill Sandbox Config ==========

// SkillSandboxEnv skill 沙箱环境变量配置
type SkillSandboxEnv struct {
	// Env 要传递给沙箱的环境变量
	Env map[string]string `yaml:"env"`
	// Mounts 要挂载到沙箱的文件
	Mounts []SkillMount `yaml:"mounts"`
}

// SkillMount 文件挂载配置
type SkillMount struct {
	Source   string `yaml:"source"`
	Target   string `yaml:"target"`
	ReadOnly bool   `yaml:"read_only"`
}

// GetSkillSandboxEnv 获取 skill 的沙箱环境配置
func GetSkillSandboxEnv(skillName string, configPath string) *SkillSandboxEnv {
	if configPath == "" {
		configPath = DefaultSkillConfigPath()
	}
	if configPath == "" {
		return nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}

	type config struct {
		Skills map[string]SkillSandboxEnv `yaml:"skills"`
	}

	var cfg config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil
	}

	if env, ok := cfg.Skills[skillName]; ok {
		return &env
	}

	return nil
}
