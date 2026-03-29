package knowledge_base

import (
	"github.com/jettjia/igo-pkg/pkg/util"
	"gorm.io/gorm"
)

type SysKnowledgeBase struct {
	Ulid         string `gorm:"column:ulid;primaryKey;type:varchar(128);comment:ulid;" json:"ulid"`
	CreatedAt    int64  `gorm:"column:created_at;autoCreateTime:milli;type:bigint;comment:创建时间;" json:"created_at"`
	UpdatedAt    int64  `gorm:"column:updated_at;autoUpdateTime:milli;type:bigint;comment:修改时间;" json:"updated_at"`
	DeletedAt    int64  `gorm:"column:deleted_at;type:bigint;comment:删除时间;" json:"deleted_at"`
	CreatedBy    string `gorm:"column:created_by;type:varchar(128);comment:创建者;" json:"created_by"`
	UpdatedBy    string `gorm:"column:updated_by;type:varchar(128);comment:修改者;" json:"updated_by"`
	Name         string `gorm:"column:name;type:varchar(100);comment:知识库名称;" json:"name"`
	Description  string `gorm:"column:description;type:varchar(500);comment:描述;" json:"description"`
	RetrievalUrl string `gorm:"column:retrieval_url;type:varchar(255);comment:检索服务URL;" json:"retrieval_url"`
	Token        string `gorm:"column:token;type:varchar(500);comment:认证Token;" json:"token"`
	Enabled      bool   `gorm:"column:enabled;type:boolean;default:true;comment:是否启用;" json:"enabled"`
}

func (po *SysKnowledgeBase) BeforeCreate(tx *gorm.DB) (err error) {
	po.Ulid = util.Ulid()
	return
}

func (po *SysKnowledgeBase) TableName() string {
	return "sys_knowledge_base"
}
