package chat

import (
	"github.com/gin-gonic/gin"

	dto "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/chat"
)

// ====== ChatSession ======

// CreateChatSession 创建会话
func (h *Handler) CreateChatSession(c *gin.Context) {
	var req dto.CreateChatSessionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	rsp, err := h.ChatSessionService.CreateChatSession(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, rsp)
}

// DeleteChatSession 删除会话
func (h *Handler) DeleteChatSession(c *gin.Context) {
	var req dto.DelChatSessionReq
	if err := c.ShouldBindUri(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	err := h.ChatSessionService.DeleteChatSession(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "deleted"})
}

// UpdateChatSession 更新会话
func (h *Handler) UpdateChatSession(c *gin.Context) {
	var req dto.UpdateChatSessionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	err := h.ChatSessionService.UpdateChatSession(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "updated"})
}

// UpdateChatSessionStatus 更新会话状态
func (h *Handler) UpdateChatSessionStatus(c *gin.Context) {
	var req dto.UpdateChatSessionStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	err := h.ChatSessionService.UpdateChatSessionStatus(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "status updated"})
}

// FindChatSessionById 查看会话byId
func (h *Handler) FindChatSessionById(c *gin.Context) {
	var req dto.FindChatSessionByIdReq
	if err := c.ShouldBindUri(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	rsp, err := h.ChatSessionService.FindChatSessionById(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, rsp)
}

// FindChatSessionsByUserId 查看用户的会话列表
func (h *Handler) FindChatSessionsByUserId(c *gin.Context) {
	var req dto.FindChatSessionsByUserIdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	rsp, err := h.ChatSessionService.FindChatSessionsByUserId(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, rsp)
}

// FindChatSessionPage 分页查询会话
func (h *Handler) FindChatSessionPage(c *gin.Context) {
	var req dto.FindChatSessionPageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	rsp, err := h.ChatSessionService.FindChatSessionPage(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, rsp)
}

// ====== ChatMessage ======

// CreateChatMessage 创建消息
func (h *Handler) CreateChatMessage(c *gin.Context) {
	var req dto.CreateChatMessageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	rsp, err := h.ChatMessageService.CreateChatMessage(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, rsp)
}

// UpdateChatMessage 更新消息
func (h *Handler) UpdateChatMessage(c *gin.Context) {
	var req dto.UpdateChatMessageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	err := h.ChatMessageService.UpdateChatMessage(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "updated"})
}

// UpdateChatMessageStatus 更新消息状态
func (h *Handler) UpdateChatMessageStatus(c *gin.Context) {
	var req dto.UpdateChatMessageStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	err := h.ChatMessageService.UpdateChatMessageStatus(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "status updated"})
}

// FindChatMessageById 查看消息byId
func (h *Handler) FindChatMessageById(c *gin.Context) {
	var req dto.FindChatMessageByIdReq
	if err := c.ShouldBindUri(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	rsp, err := h.ChatMessageService.FindChatMessageById(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, rsp)
}

// FindChatMessagesBySessionId 查看会话的消息列表
func (h *Handler) FindChatMessagesBySessionId(c *gin.Context) {
	var req dto.FindChatMessagesBySessionIdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	rsp, err := h.ChatMessageService.FindChatMessagesBySessionId(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, rsp)
}

// ====== ChatApproval ======

// CreateChatApproval 创建审批
func (h *Handler) CreateChatApproval(c *gin.Context) {
	var req dto.CreateChatApprovalReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	rsp, err := h.ChatApprovalService.CreateChatApproval(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, rsp)
}

// ApproveChatApproval 批准审批
func (h *Handler) ApproveChatApproval(c *gin.Context) {
	var req dto.ApproveChatApprovalReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	err := h.ChatApprovalService.ApproveChatApproval(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "approved"})
}

// RejectChatApproval 拒绝审批
func (h *Handler) RejectChatApproval(c *gin.Context) {
	var req dto.RejectChatApprovalReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	err := h.ChatApprovalService.RejectChatApproval(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "rejected"})
}

// FindChatApprovalById 查看审批byId
func (h *Handler) FindChatApprovalById(c *gin.Context) {
	var req dto.FindChatApprovalByIdReq
	if err := c.ShouldBindUri(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	rsp, err := h.ChatApprovalService.FindChatApprovalById(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, rsp)
}

// FindChatApprovalByMessageId 查看消息的审批
func (h *Handler) FindChatApprovalByMessageId(c *gin.Context) {
	var req dto.FindChatApprovalByMessageIdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	rsp, err := h.ChatApprovalService.FindChatApprovalByMessageId(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, rsp)
}

// FindPendingChatApprovals 查看待审批列表
func (h *Handler) FindPendingChatApprovals(c *gin.Context) {
	var req dto.FindPendingChatApprovalsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	rsp, err := h.ChatApprovalService.FindPendingChatApprovals(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, rsp)
}

// FindChatApprovalsByUserId 查看用户的审批列表
func (h *Handler) FindChatApprovalsByUserId(c *gin.Context) {
	var req dto.FindChatApprovalsByUserIdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	rsp, err := h.ChatApprovalService.FindChatApprovalsByUserId(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, rsp)
}
