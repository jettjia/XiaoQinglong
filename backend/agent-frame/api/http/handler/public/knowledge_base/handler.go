package knowledge_base

import (
	service "github.com/jettjia/xiaoqinglong/agent-frame/application/service/knowledge_base"
)

type Handler struct {
	SysKnowledgeBaseSrv *service.SysKnowledgeBaseService
}

func NewHandler() *Handler {
	return &Handler{
		SysKnowledgeBaseSrv: service.NewSysKnowledgeBaseService(),
	}
}
