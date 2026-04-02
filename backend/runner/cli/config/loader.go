package config

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/jettjia/XiaoQinglong/runner/types"
)

// DefaultEnvPrefix 是环境变量的前缀
const DefaultEnvPrefix = "RUNNER"

// EnvPrefix 环境变量前缀
var EnvPrefix = DefaultEnvPrefix

// SetEnvPrefix 设置环境变量前缀
func SetEnvPrefix(prefix string) {
	EnvPrefix = prefix
}

// LoadConfig 从环境变量加载配置，构建 RunRequest
func LoadConfig() (*types.RunRequest, error) {
	req := &types.RunRequest{
		Models: make(map[string]types.ModelConfig),
	}

	// 加载所有模型配置
	if err := loadModels(req); err != nil {
		return nil, err
	}

	// 如果没有配置任何模型，报错
	if len(req.Models) == 0 {
		return nil, fmt.Errorf("no model configured, please set RUNNER_MODEL_* environment variables")
	}

	return req, nil
}

// loadModels 加载所有模型配置
func loadModels(req *types.RunRequest) error {
	envVars := os.Environ()
	roles := make(map[string]bool)

	// 找出所有已配置的模型角色
	prefix := fmt.Sprintf("%s_MODEL_", EnvPrefix)
	for _, env := range envVars {
		if strings.HasPrefix(env, prefix) {
			rest := strings.TrimPrefix(env, prefix)
			parts := strings.SplitN(rest, "_", 2)
			if len(parts) >= 1 && parts[0] != "" {
				roles[strings.ToUpper(parts[0])] = true
			}
		}
	}

	modelRoles := []string{"default", "rewrite", "skill", "summarize"}
	for _, role := range modelRoles {
		roleUpper := strings.ToUpper(role)
		if !roles[roleUpper] {
			continue
		}

		cfg := types.ModelConfig{
			Provider:    getEnv(fmt.Sprintf("%s_MODEL_%s_PROVIDER", EnvPrefix, roleUpper)),
			Name:        getEnv(fmt.Sprintf("%s_MODEL_%s_NAME", EnvPrefix, roleUpper)),
			APIKey:      getEnv(fmt.Sprintf("%s_MODEL_%s_APIKEY", EnvPrefix, roleUpper)),
			APIBase:     getEnv(fmt.Sprintf("%s_MODEL_%s_APIBASE", EnvPrefix, roleUpper)),
			Temperature: getEnvFloat(fmt.Sprintf("%s_MODEL_%s_TEMPERATURE", EnvPrefix, roleUpper), 0.7),
			MaxTokens:   getEnvInt(fmt.Sprintf("%s_MODEL_%s_MAXTOKENS", EnvPrefix, roleUpper), 4096),
			TopP:        getEnvFloat(fmt.Sprintf("%s_MODEL_%s_TOPP", EnvPrefix, roleUpper), 0.9),
		}

		if cfg.Name != "" {
			req.Models[role] = cfg
		}
	}

	return nil
}

// GetMode 获取运行模式: local 或 http
func GetMode() string {
	mode := getEnv(fmt.Sprintf("%s_MODE", EnvPrefix))
	if mode == "" {
		return "http"
	}
	return mode
}

// GetEndpoint 获取 HTTP 模式的端点
func GetEndpoint() string {
	endpoint := getEnv(fmt.Sprintf("%s_HTTP_ENDPOINT", EnvPrefix))
	if endpoint == "" {
		return "http://localhost:18080"
	}
	return endpoint
}

// GetDefaultModel 获取默认模型角色
func GetDefaultModel() string {
	return getEnv(fmt.Sprintf("%s_DEFAULT_MODEL", EnvPrefix))
}

// getEnv 获取环境变量并展开 ${ENV_VAR} 格式
func getEnv(key string) string {
	val := os.Getenv(key)
	return expandEnvStr(val)
}

// getEnvFloat 获取浮点型环境变量
func getEnvFloat(key string, defaultVal float64) float64 {
	val := getEnv(key)
	if val == "" {
		return defaultVal
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return defaultVal
	}
	return f
}

// getEnvInt 获取整型环境变量
func getEnvInt(key string, defaultVal int) int {
	val := getEnv(key)
	if val == "" {
		return defaultVal
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return i
}

// expandEnvStr 展开 ${ENV_VAR} 格式的环境变量
func expandEnvStr(s string) string {
	if s == "" {
		return s
	}
	re := regexp.MustCompile(`\$\{([^}]+)\}`)
	return re.ReplaceAllStringFunc(s, func(match string) string {
		envVar := match[2 : len(match)-1]
		return os.Getenv(envVar)
	})
}

// ShowConfig 返回当前配置的字符串表示
func ShowConfig(req *types.RunRequest) string {
	var b strings.Builder
	b.WriteString("=== Runner CLI Configuration ===\n\n")
	b.WriteString(fmt.Sprintf("Mode: %s\n", GetMode()))
	b.WriteString(fmt.Sprintf("HTTP Endpoint: %s\n", GetEndpoint()))
	b.WriteString(fmt.Sprintf("Default Model: %s\n\n", GetDefaultModel()))
	b.WriteString("Models:\n")
	for role, cfg := range req.Models {
		b.WriteString(fmt.Sprintf("  [%s]\n", role))
		b.WriteString(fmt.Sprintf("    Provider: %s\n", cfg.Provider))
		b.WriteString(fmt.Sprintf("    Name: %s\n", cfg.Name))
		if cfg.APIBase != "" {
			b.WriteString(fmt.Sprintf("    APIBase: %s\n", cfg.APIBase))
		}
		b.WriteString(fmt.Sprintf("    Temperature: %.2f\n", cfg.Temperature))
		b.WriteString(fmt.Sprintf("    MaxTokens: %d\n", cfg.MaxTokens))
		b.WriteString(fmt.Sprintf("    TopP: %.2f\n", cfg.TopP))
	}
	return b.String()
}
