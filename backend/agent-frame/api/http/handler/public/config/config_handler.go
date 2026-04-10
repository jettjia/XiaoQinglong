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

const (
	ConfigFilePath   = "./manifest/config/config.yaml"
	SkillsConfigPath = "../runner/skills-config.yaml"
)

// GetAppConfig 获取应用配置 config.yaml
func (h *Handler) GetAppConfig(c *gin.Context) {
	content, err := os.ReadFile(ConfigFilePath)
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
	dir := filepath.Dir(ConfigFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		err = xerror.NewErrorOpt(apierror.InternalServerErr, xerror.WithCause("Failed to create directory: "+err.Error()))
		_ = c.Error(err)
		return
	}

	if err := os.WriteFile(ConfigFilePath, []byte(req.Content), 0644); err != nil {
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
	content, err := os.ReadFile(SkillsConfigPath)
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
	dir := filepath.Dir(SkillsConfigPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		err = xerror.NewErrorOpt(apierror.InternalServerErr, xerror.WithCause("Failed to create directory: "+err.Error()))
		_ = c.Error(err)
		return
	}

	if err := os.WriteFile(SkillsConfigPath, []byte(req.Content), 0644); err != nil {
		err = xerror.NewErrorOpt(apierror.InternalServerErr, xerror.WithCause("Failed to write skills config file: "+err.Error()))
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, map[string]string{
		"message": "Skills config saved successfully",
	})
}