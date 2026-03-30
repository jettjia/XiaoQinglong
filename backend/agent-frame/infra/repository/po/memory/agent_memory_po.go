package memory

import (
	"time"

	"github.com/jinzhu/copier"
	"github.com/jettjia/igo-pkg/pkg/util"
	"gorm.io/gorm"
)

type AgentMemory struct {
	Ulid         string `gorm:"column:ulid;primaryKey;type:varchar(128);comment:ulid;" json:"ulid"`
	CreatedAt    int64  `gorm:"column:created_at;autoCreateTime:milli;type:bigint;comment:创建时间;" json:"created_at"`
	UpdatedAt    int64  `gorm:"column:updated_at;autoUpdateTime:milli;type:bigint;comment:修改时间;" json:"updated_at"`
	DeletedAt    int64  `gorm:"column:deleted_at;type:bigint;comment:删除时间;" json:"deleted_at"`
	AgentId      string `gorm:"column:agent_id;type:varchar(128);comment:Agent ID;" json:"agent_id"`
	UserId       string `gorm:"column:user_id;type:varchar(128);comment:用户ID;" json:"user_id"`
	SessionId    string `gorm:"column:session_id;type:varchar(128);comment:会话ID;" json:"session_id"`
	MemoryType   string `gorm:"column:memory_type;type:varchar(50);comment:记忆类型:summary|entity|preference|fact;" json:"memory_type"`
	Content      string `gorm:"column:content;type:text;comment:记忆内容;" json:"content"`
	Keywords     string `gorm:"column:keywords;type:text;comment:关键词，用于检索;" json:"keywords"`
	Importance   int    `gorm:"column:importance;type:int;default:1;comment:重要性评分;" json:"importance"`
	SourceMsgId  string `gorm:"column:source_msg_id;type:varchar(128);comment:来源消息ID;" json:"source_msg_id"`
	ExpiresAt    int64  `gorm:"column:expires_at;type:bigint;comment:过期时间，0表示不过期;" json:"expires_at"`
}

func (po *AgentMemory) BeforeCreate(tx *gorm.DB) (err error) {
	po.Ulid = util.Ulid()
	po.CreatedAt = time.Now().UnixMilli()
	po.UpdatedAt = po.CreatedAt
	return
}

func (po *AgentMemory) TableName() string {
	return "agent_memory"
}

// EntityToPO entity转po
func EntityToPO(en *AgentMemory) *AgentMemory {
	var po AgentMemory
	copier.Copy(&po, en)
	po.UpdatedAt = time.Now().UnixMilli()
	return &po
}

// POToEntity po转entity
func POToEntity(po *AgentMemory) *AgentMemory {
	var en AgentMemory
	copier.Copy(&en, po)
	return &en
}

// POToEntities po列表转entity列表
func POToEntities(pos []*AgentMemory) []*AgentMemory {
	ens := make([]*AgentMemory, 0)
	for _, po := range pos {
		ens = append(ens, POToEntity(po))
	}
	return ens
}