package memory

type AgentMemory struct {
	Ulid        string `json:"ulid"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
	DeletedAt   int64  `json:"deleted_at"`
	AgentId     string `json:"agent_id"`
	UserId      string `json:"user_id"`
	SessionId   string `json:"session_id"`
	MemoryType  string `json:"memory_type"` // summary, entity, preference, fact
	Content     string `json:"content"`
	Keywords    string `json:"keywords"`
	Importance  int    `json:"importance"`
	SourceMsgId string `json:"source_msg_id"`
	ExpiresAt   int64  `json:"expires_at"`
}