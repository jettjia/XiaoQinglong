package config

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/jettjia/igo-pkg/pkg/xerror"
	"github.com/jettjia/igo-pkg/pkg/xresponse"

	"github.com/jettjia/xiaoqinglong/agent-frame/types/apierror"
)

// GetConfigFilePath returns the actual config file path
func GetConfigFilePath() string {
	// 开发环境使用 dev-config.yaml
	if os.Getenv("env") == "debug" {
		return "agent-frame/manifest/config/dev-config.yaml"
	}
	// 生产环境优先使用 XQL_CONFIG_PATH
	if xqlConfigPath := os.Getenv("XQL_CONFIG_PATH"); xqlConfigPath != "" {
		return xqlConfigPath
	}
	// 生产环境使用用户配置目录
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/tmp"
	}
	return filepath.Join(home, ".xiaoqinglong", "config", "config.yaml")
}

// GetSkillsConfigFilePath returns the actual skills config file path
func GetSkillsConfigFilePath() string {
	// 开发环境使用 dev-skills-config.yaml
	if os.Getenv("env") == "debug" {
		return "backend/runner/dev-skills-config.yaml"
	}
	// 生产环境优先使用 XQL_SKILLS_CONFIG_PATH
	if xqlSkillsConfigPath := os.Getenv("XQL_SKILLS_CONFIG_PATH"); xqlSkillsConfigPath != "" {
		return xqlSkillsConfigPath
	}
	// 生产环境使用用户配置目录
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/tmp"
	}
	return filepath.Join(home, ".xiaoqinglong", "config", "skills-config.yaml")
}

// GetAppConfig 获取应用配置 config.yaml
func (h *Handler) GetAppConfig(c *gin.Context) {
	configFilePath := GetConfigFilePath()
	content, err := os.ReadFile(configFilePath)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.InternalServerErr, xerror.WithCause("Failed to read config file: "+err.Error()))
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, map[string]string{
		"content": string(content),
	})
}

// SaveAppConfig 保存应用配置 config.yaml
func (h *Handler) SaveAppConfig(c *gin.Context) {
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.BindJSON(&req); err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause("Invalid request: "+err.Error()))
		_ = c.Error(err)
		return
	}

	// 确保目录存在
	configFilePath := GetConfigFilePath()
	dir := filepath.Dir(configFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		err = xerror.NewErrorOpt(apierror.InternalServerErr, xerror.WithCause("Failed to create directory: "+err.Error()))
		_ = c.Error(err)
		return
	}

	if err := os.WriteFile(configFilePath, []byte(req.Content), 0644); err != nil {
		err = xerror.NewErrorOpt(apierror.InternalServerErr, xerror.WithCause("Failed to write config file: "+err.Error()))
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, map[string]string{
		"message": "Config saved successfully",
	})
}

// GetSkillsConfig 获取技能配置 skills-config.yaml
func (h *Handler) GetSkillsConfig(c *gin.Context) {
	skillsConfigPath := GetSkillsConfigFilePath()
	content, err := os.ReadFile(skillsConfigPath)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.InternalServerErr, xerror.WithCause("Failed to read skills config file: "+err.Error()))
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, map[string]string{
		"content": string(content),
	})
}

// SaveSkillsConfig 保存技能配置 skills-config.yaml
func (h *Handler) SaveSkillsConfig(c *gin.Context) {
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.BindJSON(&req); err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause("Invalid request: "+err.Error()))
		_ = c.Error(err)
		return
	}

	// 确保目录存在
	skillsConfigPath := GetSkillsConfigFilePath()
	dir := filepath.Dir(skillsConfigPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		err = xerror.NewErrorOpt(apierror.InternalServerErr, xerror.WithCause("Failed to create directory: "+err.Error()))
		_ = c.Error(err)
		return
	}

	if err := os.WriteFile(skillsConfigPath, []byte(req.Content), 0644); err != nil {
		err = xerror.NewErrorOpt(apierror.InternalServerErr, xerror.WithCause("Failed to write skills config file: "+err.Error()))
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, map[string]string{
		"message": "Skills config saved successfully",
	})
}