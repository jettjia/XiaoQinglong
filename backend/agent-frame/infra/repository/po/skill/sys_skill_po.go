package skill

import (
	"github.com/jettjia/igo-pkg/pkg/util"
	"gorm.io/gorm"
)

type SysSkill struct {
	Ulid         string `gorm:"column:ulid;primaryKey;type:varchar(128);comment:ulid;" json:"ulid"`
	CreatedAt    int64  `gorm:"column:created_at;autoCreateTime:milli;type:bigint;comment:创建时间;" json:"created_at"`
	UpdatedAt    int64  `gorm:"column:updated_at;autoUpdateTime:milli;type:bigint;comment:修改时间;" json:"updated_at"`
	DeletedAt    int64  `gorm:"column:deleted_at;type:bigint;comment:删除时间;" json:"deleted_at"`
	CreatedBy    string `gorm:"column:created_by;type:varchar(128);comment:创建者;" json:"created_by"`
	UpdatedBy    string `gorm:"column:updated_by;type:varchar(128);comment:修改者;" json:"updated_by"`
	Name         string `gorm:"column:name;type:varchar(100);comment:技能名称;" json:"name"`
	Description  string `gorm:"column:description;type:text;comment:描述;" json:"description"`
	SkillType    string `gorm:"column:skill_type;type:varchar(20);comment:类型:mcp/tool/a2a;" json:"skill_type"`
	Version      string `gorm:"column:version;type:varchar(20);comment:版本号;" json:"version"`
	Path         string `gorm:"column:path;type:varchar(255);comment:存储路径;" json:"path"`
	Enabled      bool   `gorm:"column:enabled;type:boolean;default:true;comment:是否启用;" json:"enabled"`
	Config       string `gorm:"column:config;type:text;comment:扩展配置JSON;" json:"config"`
	IsSystem     bool   `gorm:"column:is_system;type:boolean;default:false;comment:是否系统内置;" json:"is_system"`
	RiskLevel    string `gorm:"column:risk_level;type:varchar(20);default:'low';comment:风险等级:low/medium/high;" json:"risk_level"`
}

func (po *SysSkill) BeforeCreate(tx *gorm.DB) (err error) {
	po.Ulid = util.Ulid()
	return
}

func (po *SysSkill) TableName() string {
	return "sys_skill"
}
