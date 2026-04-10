package channel

import (
	service "github.com/jettjia/xiaoqinglong/agent-frame/application/service/channel"
)

type Handler struct {
	SysChannelSrv *service.SysChannelService
}

func NewHandler() *Handler {
	return &Handler{
		SysChannelSrv: service.NewSysChannelService(),
	}
}