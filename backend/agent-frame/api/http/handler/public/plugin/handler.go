package plugin

import (
	servicePlugin "github.com/jettjia/xiaoqinglong/agent-frame/application/service/plugin"
)

type Handler struct {
	SysPluginSrv *servicePlugin.SysPluginService
}

func NewHandler() *Handler {
	return &Handler{
		SysPluginSrv: servicePlugin.NewSysPluginService(),
	}
}