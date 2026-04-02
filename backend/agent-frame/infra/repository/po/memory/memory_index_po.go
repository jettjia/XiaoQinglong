package memory

import (
	"time"

	"github.com/jettjia/igo-pkg/pkg/util"
	"gorm.io/gorm"
)

// MemoryIndex 记忆索引，替代 Claude Code 的 MEMORY.md 文本索引
// 每个记忆对应一条索引记录，用于快速检索和展示
type MemoryIndex struct {
	Ulid       string `gorm:"column:ulid;primaryKey;type:varchar(128);comment:ulid;" json:"ulid"`
	MemoryID   string `gorm:"column:memory_id;type:varchar(128);index;comment:关联的记忆ID;" json:"memory_id"`
	HookLine   string `gorm:"column:hook_line;type:varchar(500);comment:索引行，如 - [Title](file.md) — one-line hook;" json:"hook_line"`
	MemoryType string `gorm:"column:memory_type;type:varchar(50);index;comment:记忆类型;" json:"memory_type"`
	AgentId    string `gorm:"column:agent_id;type:varchar(128);index;comment:Agent ID;" json:"agent_id"`
	UserId     string `gorm:"column:user_id;type:varchar(128);index;comment:用户ID;" json:"user_id"`
	CreatedAt  int64  `gorm:"column:created_at;autoCreateTime:milli;type:bigint;comment:创建时间;" json:"created_at"`
	UpdatedAt  int64  `gorm:"column:updated_at;autoUpdateTime:milli;type:bigint;comment:修改时间;" json:"updated_at"`
}

func (po *MemoryIndex) BeforeCreate(tx *gorm.DB) (err error) {
	po.Ulid = util.Ulid()
	po.CreatedAt = time.Now().UnixMilli()
	po.UpdatedAt = po.CreatedAt
	return
}

func (po *MemoryIndex) TableName() string {
	return "memory_index"
}

// BuildHookLine 从记忆内容构建索引行
// 格式: - [Name](memory_ulid) — description
func BuildHookLine(memoryID, name, description string) string {
	if name == "" {
		name = memoryID
	}
	if description == "" {
		description = "..."
	}
	// 截断过长的描述
	if len(description) > 150 {
		description = description[:147] + "..."
	}
	return "- [" + name + "](" + memoryID + ") — " + description
}
