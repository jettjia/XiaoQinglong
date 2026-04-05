package chat

import (
	"github.com/jettjia/igo-pkg/pkg/xsql/builder"
)

// ====== ChatSession ======

// 请求对象
type (
	// CreateChatSessionReq 创建会话请求对象
	CreateChatSessionReq struct {
		Ulid      string `json:"ulid"`
		UserId    string `json:"user_id" validate:"required"`
		AgentId   string `json:"agent_id" validate:"required"`
		Title     string `json:"title"`
		Channel   string `json:"channel"`
		Model     string `json:"model"`
		Status    string `json:"status"`
		CreatedBy string `json:"created_by"`
	}

	// DelChatSessionReq 删除会话请求对象
	DelChatSessionReq struct {
		Ulid string `validate:"required" uri:"ulid" json:"ulid"`
	}

	// UpdateChatSessionReq 更新会话请求对象
	UpdateChatSessionReq struct {
		Ulid      string `validate:"required" json:"ulid"`
		AgentId   string `json:"agent_id"`
		Title     string `json:"title"`
		Status    string `json:"status"`
		UpdatedBy string `json:"updated_by"`
	}

	// UpdateChatSessionStatusReq 更新会话状态请求对象
	UpdateChatSessionStatusReq struct {
		Ulid   string `validate:"required" json:"ulid"`
		Status string `json:"status"`
	}

	// FindChatSessionByIdReq 查询会话请求对象
	FindChatSessionByIdReq struct {
		Ulid string `validate:"required" uri:"ulid" json:"ulid"`
	}

	// FindChatSessionsByUserIdReq 查询用户会话列表请求对象
	FindChatSessionsByUserIdReq struct {
		UserId string `validate:"required" json:"user_id"`
		Status string `json:"status"`
	}

	// FindChatSessionPageReq 分页查询会话请求对象
	FindChatSessionPageReq struct {
		Query    []*builder.Query  `json:"query"`
		PageData *builder.PageData `json:"page_data"`
		SortData *builder.SortData `json:"sort_data"`
	}
)

// 输出对象
type (
	// CreateChatSessionRsp 创建会话返回对象
	CreateChatSessionRsp struct {
		Ulid string `json:"ulid"`
	}

	// ChatSessionRsp 会话返回对象
	ChatSessionRsp struct {
		Ulid      string `json:"ulid"`
		UserId    string `json:"user_id"`
		AgentId   string `json:"agent_id"`
		Title     string `json:"title"`
		Channel   string `json:"channel"`
		Model     string `json:"model"`
		Status    string `json:"status"`
		CreatedAt int64  `json:"created_at"`
		UpdatedAt int64  `json:"updated_at"`
		CreatedBy string `json:"created_by"`
		UpdatedBy string `json:"updated_by"`
	}

	// FindChatSessionPageRsp 分页查询会话返回对象
	FindChatSessionPageRsp struct {
		Entries  []*ChatSessionRsp `json:"entries"`
		PageData *builder.PageData `json:"page_data"`
	}
)

// ====== ChatMessage ======

// 请求对象
type (
	// CreateChatMessageReq 创建消息请求对象
	CreateChatMessageReq struct {
		Ulid         string `json:"ulid"`
		SessionId    string `json:"session_id" validate:"required"`
		Role         string `json:"role" validate:"required"`
		Content      string `json:"content"`
		Model        string `json:"model"`
		InputTokens  int    `json:"input_tokens"`
		OutputTokens int    `json:"output_tokens"`
		TotalTokens  int    `json:"total_tokens"`
		LatencyMs    int64  `json:"latency_ms"`
		Trace        string `json:"trace"`
		Status       string `json:"status"`
		ErrorMsg     string `json:"error_msg"`
		Metadata     string `json:"metadata"`
	}

	// UpdateChatMessageReq 更新消息请求对象
	UpdateChatMessageReq struct {
		Ulid     string `validate:"required" json:"ulid"`
		Content  string `json:"content"`
		Tokens   int    `json:"tokens"`
		Status   string `json:"status"`
		ErrorMsg string `json:"error_msg"`
	}

	// UpdateChatMessageStatusReq 更新消息状态请求对象
	UpdateChatMessageStatusReq struct {
		Ulid   string `validate:"required" json:"ulid"`
		Status string `json:"status"`
	}

	// FindChatMessageByIdReq 查询消息请求对象
	FindChatMessageByIdReq struct {
		Ulid string `validate:"required" uri:"ulid" json:"ulid"`
	}

	// FindChatMessagesBySessionIdReq 查询会话消息列表请求对象
	FindChatMessagesBySessionIdReq struct {
		SessionId string `validate:"required" json:"session_id"`
	}
)

// 输出对象
type (
	// CreateChatMessageRsp 创建消息返回对象
	CreateChatMessageRsp struct {
		Ulid string `json:"ulid"`
	}

	// ChatMessageRsp 消息返回对象
	ChatMessageRsp struct {
		Ulid      string `json:"ulid"`
		SessionId string `json:"session_id"`
		Role      string `json:"role"`
		Content   string `json:"content"`
		Model     string `json:"model"`
		Tokens    int    `json:"tokens"`
		LatencyMs int64  `json:"latency_ms"`
		Trace     string `json:"trace"`
		Status    string `json:"status"`
		ErrorMsg  string `json:"error_msg"`
		Metadata  string `json:"metadata"`
		CreatedAt int64  `json:"created_at"`
		UpdatedAt int64  `json:"updated_at"`
	}
)

// ====== ChatApproval ======

// 请求对象
type (
	// CreateChatApprovalReq 创建审批请求对象
	CreateChatApprovalReq struct {
		Ulid        string `json:"ulid"`
		MessageId   string `json:"message_id" validate:"required"`
		SessionId   string `json:"session_id" validate:"required"`
		ToolName    string `json:"tool_name" validate:"required"`
		ToolType    string `json:"tool_type"`
		RiskLevel   string `json:"risk_level"`
		Parameters  string `json:"parameters"`
		Status      string `json:"status"`
		InterruptID string `json:"interrupt_id"`
		ApprovedBy  string `json:"approved_by"`
		Reason      string `json:"reason"`
	}

	// UpdateChatApprovalStatusReq 更新审批状态请求对象
	UpdateChatApprovalStatusReq struct {
		Ulid       string `validate:"required" json:"ulid"`
		Status     string `json:"status"`
		ApprovedBy string `json:"approved_by"`
		Reason     string `json:"reason"`
	}

	// ApproveChatApprovalReq 批准审批请求对象
	ApproveChatApprovalReq struct {
		Ulid       string `validate:"required" json:"ulid"`
		ApprovedBy string `json:"approved_by"`
		Reason     string `json:"reason"`
	}

	// RejectChatApprovalReq 拒绝审批请求对象
	RejectChatApprovalReq struct {
		Ulid       string `validate:"required" json:"ulid"`
		ApprovedBy string `json:"approved_by"`
		Reason     string `json:"reason"`
	}

	// FindChatApprovalByIdReq 查询审批请求对象
	FindChatApprovalByIdReq struct {
		Ulid string `validate:"required" uri:"ulid" json:"ulid"`
	}

	// FindChatApprovalByMessageIdReq 查询消息审批请求对象
	FindChatApprovalByMessageIdReq struct {
		MessageId string `validate:"required" json:"message_id"`
	}

	// FindPendingChatApprovalsReq 查询待审批列表请求对象
	FindPendingChatApprovalsReq struct {
	}

	// FindChatApprovalsByUserIdReq 查询用户审批列表请求对象
	FindChatApprovalsByUserIdReq struct {
		UserId string `validate:"required" json:"user_id"`
	}
)

// 输出对象
type (
	// CreateChatApprovalRsp 创建审批返回对象
	CreateChatApprovalRsp struct {
		Ulid string `json:"ulid"`
	}

	// ChatApprovalRsp 审批返回对象
	ChatApprovalRsp struct {
		Ulid        string `json:"ulid"`
		MessageId   string `json:"message_id"`
		SessionId   string `json:"session_id"`
		ToolName    string `json:"tool_name"`
		ToolType    string `json:"tool_type"`
		RiskLevel   string `json:"risk_level"`
		Parameters  string `json:"parameters"`
		Status      string `json:"status"`
		InterruptID string `json:"interrupt_id"`
		ApprovedBy  string `json:"approved_by"`
		ApprovedAt  int64  `json:"approved_at"`
		Reason      string `json:"reason"`
		CreatedAt   int64  `json:"created_at"`
		UpdatedAt   int64  `json:"updated_at"`
	}
)

// ====== ChatTokenStats ======

// ChatTokenStatsRsp Token统计返回对象
type ChatTokenStatsRsp struct {
	Ulid         string  `json:"ulid"`
	SessionId    string  `json:"session_id"`
	AgentId      string  `json:"agent_id"`
	UserId       string  `json:"user_id"`
	Date         string  `json:"date"`
	Model        string  `json:"model"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	TotalTokens  int     `json:"total_tokens"`
	RequestCount int     `json:"request_count"`
	CostAmount   float64 `json:"cost_amount"`
	CreatedAt    int64   `json:"created_at"`
	UpdatedAt    int64   `json:"updated_at"`
}
