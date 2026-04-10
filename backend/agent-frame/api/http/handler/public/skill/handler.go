package skill

import (
	service "github.com/jettjia/xiaoqinglong/agent-frame/application/service/skill"
)

type Handler struct {
	SysSkillSrv *service.SysSkillService
}

func NewHandler() *Handler {
	return &Handler{
		SysSkillSrv: service.NewSysSkillService(),
	}
}
