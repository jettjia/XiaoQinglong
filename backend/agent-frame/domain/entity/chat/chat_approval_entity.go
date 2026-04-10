package chat

import "time"

// ChatApproval 聊天审批
type ChatApproval struct {
	Ulid        string `json:"ulid"`
	MessageId   string `json:"message_id"`
	AgentId     string `json:"agent_id"`
	ToolName    string `json:"tool_name"`
	RiskLevel   string `json:"risk_level"` // low, medium, high
	Parameters  string `json:"parameters"` // JSON string of tool parameters
	Status      string `json:"status"` // pending, approved, rejected
	ApprovedBy  string `json:"approved_by"`
	ApprovedAt  int64  `json:"approved_at"`
	Reason      string `json:"reason"`
	CreatedAt   int64  `json:"created_at"`
}

// IsPending 判断是否待审批
func (a *ChatApproval) IsPending() bool {
	return a.Status == "pending"
}

// IsApproved 判断是否已批准
func (a *ChatApproval) IsApproved() bool {
	return a.Status == "approved"
}

// IsRejected 判断是否已拒绝
func (a *ChatApproval) IsRejected() bool {
	return a.Status == "rejected"
}

// Approve 批准审批
func (a *ChatApproval) Approve(approvedBy string) {
	a.Status = "approved"
	a.ApprovedBy = approvedBy
	a.ApprovedAt = nowMilli()
}

// Reject 拒绝审批
func (a *ChatApproval) Reject(approvedBy, reason string) {
	a.Status = "rejected"
	a.ApprovedBy = approvedBy
	a.ApprovedAt = nowMilli()
	a.Reason = reason
}

func nowMilli() int64 {
	return time.Now().UnixMilli()
}