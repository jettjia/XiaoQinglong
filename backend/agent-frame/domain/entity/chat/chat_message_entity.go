package chat

// ChatMessage 聊天消息
type ChatMessage struct {
	Ulid         string `json:"ulid"`
	SessionId    string `json:"session_id"`
	CreatedAt    int64  `json:"created_at"`
	Role         string `json:"role"` // user, assistant, system
	Content      string `json:"content"`
	Model        string `json:"model"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	TotalTokens  int    `json:"total_tokens"`
	LatencyMs    int    `json:"latency_ms"`
	Trace        string `json:"trace"` // JSON string of trace steps
	Status       string `json:"status"` // sending, success, failed, pending_approval
	ErrorMsg     string `json:"error_msg"`
	Metadata     string `json:"metadata"` // JSON string of additional metadata
}

// IsPendingApproval 判断消息是否等待审批
func (m *ChatMessage) IsPendingApproval() bool {
	return m.Status == "pending_approval"
}

// IsSuccess 判断消息是否成功
func (m *ChatMessage) IsSuccess() bool {
	return m.Status == "success"
}

// IsFailed 判断消息是否失败
func (m *ChatMessage) IsFailed() bool {
	return m.Status == "failed"
}

// IsUserMessage 判断是否用户消息
func (m *ChatMessage) IsUserMessage() bool {
	return m.Role == "user"
}

// IsAssistantMessage 判断是否助手消息
func (m *ChatMessage) IsAssistantMessage() bool {
	return m.Role == "assistant"
}