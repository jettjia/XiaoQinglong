package chat

import (
	service "github.com/jettjia/xiaoqinglong/agent-frame/application/service/chat"
)

type Handler struct {
	ChatSessionService   *service.ChatSessionService
	ChatMessageService   *service.ChatMessageService
	ChatApprovalService  *service.ChatApprovalService
	ChatTokenStatsService *service.ChatTokenStatsService
}

func NewHandler() *Handler {
	return &Handler{
		ChatSessionService:   service.NewChatSessionService(),
		ChatMessageService:   service.NewChatMessageService(),
		ChatApprovalService:  service.NewChatApprovalService(),
		ChatTokenStatsService: service.NewChatTokenStatsService(),
	}
}