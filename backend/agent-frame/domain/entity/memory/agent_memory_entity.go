package memory

// 记忆类型枚举（与 Claude Code 一致）
const (
	MemoryTypeUser      = "user"      // 用户角色、偏好、知识
	MemoryTypeFeedback  = "feedback"  // 用户指导（避免什么、保持什么）
	MemoryTypeProject   = "project"   // 项目上下文（谁在做什么、为什么）
	MemoryTypeReference = "reference" // 外部系统指针

	// 兼容旧的类型（后续可以迁移）
	LegacyMemoryTypeSummary    = "summary"
	LegacyMemoryTypeEntity     = "entity"
	LegacyMemoryTypePreference = "preference"
	LegacyMemoryTypeFact       = "fact"
)

// AllMemoryTypes 所有支持的记忆类型
var AllMemoryTypes = []string{
	MemoryTypeUser,
	MemoryTypeFeedback,
	MemoryTypeProject,
	MemoryTypeReference,
}

// IsValidMemoryType 检查类型是否有效
func IsValidMemoryType(t string) bool {
	for _, valid := range AllMemoryTypes {
		if t == valid {
			return true
		}
	}
	return false
}

type AgentMemory struct {
	Ulid        string `json:"ulid"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
	DeletedAt   int64  `json:"deleted_at"`
	AgentId     string `json:"agent_id"`
	UserId      string `json:"user_id"`
	SessionId   string `json:"session_id"`
	MemoryType  string `json:"memory_type"` // user, feedback, project, reference (旧: summary, entity, preference, fact)
	Name        string `json:"name"`        // 记忆名称，如 "user_role", "feedback_testing"
	Description string `json:"description"` // 一句话描述（用于索引）
	Content     string `json:"content"`
	Keywords    string `json:"keywords"`
	Importance  int    `json:"importance"`
	Source      string `json:"source"`      // private/team
	SourceMsgId string `json:"source_msg_id"`
	ExpiresAt   int64  `json:"expires_at"` // 过期时间，0=永不过期
}