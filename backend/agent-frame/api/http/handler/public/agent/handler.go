package agent

import (
	service "github.com/jettjia/xiaoqinglong/agent-frame/application/service/agent"
)

type Handler struct {
	SysAgentSrv *service.SysAgentService
}

func NewHandler() *Handler {
	return &Handler{
		SysAgentSrv: service.NewSysAgentService(),
	}
}
