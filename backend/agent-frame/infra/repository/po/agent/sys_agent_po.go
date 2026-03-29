package agent

import (
	"github.com/jettjia/igo-pkg/pkg/util"
	"gorm.io/gorm"
)

type SysAgent struct {
	Ulid         string `gorm:"column:ulid;primaryKey;type:varchar(128);comment:ulid;" json:"ulid"`
	CreatedAt    int64  `gorm:"column:created_at;autoCreateTime:milli;type:bigint;comment:创建时间;" json:"created_at"`
	UpdatedAt    int64  `gorm:"column:updated_at;autoUpdateTime:milli;type:bigint;comment:修改时间;" json:"updated_at"`
	DeletedAt    int64  `gorm:"column:deleted_at;type:bigint;comment:删除时间;" json:"deleted_at"`
	CreatedBy    string `gorm:"column:created_by;type:varchar(128);comment:创建者;" json:"created_by"`
	UpdatedBy    string `gorm:"column:updated_by;type:varchar(128);comment:修改者;" json:"updated_by"`
	Name         string `gorm:"column:name;type:varchar(100);comment:Agent名称;" json:"name"`
	Description  string `gorm:"column:description;type:text;comment:描述;" json:"description"`
	Icon         string `gorm:"column:icon;type:varchar(50);comment:图标名称;" json:"icon"`
	Model        string `gorm:"column:model;type:varchar(100);comment:默认模型;" json:"model"`
	Config       string `gorm:"column:config;type:text;comment:完整JSON配置;" json:"config"`
	ConfigJson   string `gorm:"column:config_json;type:text;comment:可运行JSON配置;" json:"config_json"`
	IsSystem     bool   `gorm:"column:is_system;type:boolean;default:false;comment:是否系统内置;" json:"is_system"`
	Enabled      bool   `gorm:"column:enabled;type:boolean;default:true;comment:是否启用;" json:"enabled"`
	Channels     string `gorm:"column:channels;type:varchar(500);comment:渠道;" json:"channels"`
	IsPeriodic   bool   `gorm:"column:is_periodic;type:boolean;default:false;comment:是否周期任务;" json:"is_periodic"`
	CronRule     string `gorm:"column:cron_rule;type:varchar(100);comment:Cron规则;" json:"cron_rule"`
}

func (po *SysAgent) BeforeCreate(tx *gorm.DB) (err error) {
	po.Ulid = util.Ulid()
	return
}

func (po *SysAgent) TableName() string {
	return "sys_agent"
}
