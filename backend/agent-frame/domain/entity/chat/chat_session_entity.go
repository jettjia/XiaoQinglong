package chat

// ChatSession 聊天会话
type ChatSession struct {
	Ulid      string `json:"ulid"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
	DeletedAt int64  `json:"deleted_at"`
	UserId    string `json:"user_id"`
	AgentId   string `json:"agent_id"`
	Title     string `json:"title"`
	Status    string `json:"status"` // active, archived
	Channel   string `json:"channel"` // web, api, feishu, dingtalk
}

// IsActive 判断会话是否活跃
func (c *ChatSession) IsActive() bool {
	return c.Status == "active"
}

// IsArchived 判断会话是否已归档
func (c *ChatSession) IsArchived() bool {
	return c.Status == "archived"
}

// IsDeleted 判断会话是否已删除
func (c *ChatSession) IsDeleted() bool {
	return c.DeletedAt > 0
}