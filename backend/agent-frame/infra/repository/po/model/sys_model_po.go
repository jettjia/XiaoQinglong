package model

import (
	"github.com/jettjia/igo-pkg/pkg/util"
	"gorm.io/gorm"
)

type SysModel struct {
	Ulid          string `gorm:"column:ulid;primaryKey;type:varchar(128);comment:ulid;" json:"ulid"`
	CreatedAt     int64  `gorm:"column:created_at;autoCreateTime:milli;type:bigint;comment:创建时间;" json:"created_at"`
	UpdatedAt     int64  `gorm:"column:updated_at;autoUpdateTime:milli;type:bigint;comment:修改时间;" json:"updated_at"`
	DeletedAt     int64  `gorm:"column:deleted_at;type:bigint;comment:删除时间;" json:"deleted_at"`
	CreatedBy     string `gorm:"column:created_by;type:varchar(128);comment:创建者;" json:"created_by"`
	UpdatedBy     string `gorm:"column:updated_by;type:varchar(128);comment:修改者;" json:"updated_by"`
	Name          string `gorm:"column:name;type:varchar(100);comment:模型名称;" json:"name"`
	Provider      string `gorm:"column:provider;type:varchar(50);comment:提供商;" json:"provider"`
	BaseUrl       string `gorm:"column:base_url;type:varchar(255);comment:API地址;" json:"base_url"`
	ApiKey        string `gorm:"column:api_key;type:varchar(500);comment:API密钥;" json:"api_key"`
	ModelType     string `gorm:"column:model_type;type:varchar(20);comment:llm/embedding;" json:"model_type"`
	Category      string `gorm:"column:category;type:varchar(20);comment:default/rewrite/skill/summarize;" json:"category"`
	Status        string `gorm:"column:status;type:varchar(20);comment:active/configured/error;" json:"status"`
	Latency       string `gorm:"column:latency;type:varchar(20);comment:平均延迟;" json:"latency"`
	ContextWindow string `gorm:"column:context_window;type:varchar(20);comment:上下文窗口;" json:"context_window"`
	Usage         int    `gorm:"column:usage;type:int;default:0;comment:使用次数;" json:"usage"`
}

func (po *SysModel) BeforeCreate(tx *gorm.DB) (err error) {
	po.Ulid = util.Ulid()
	return
}

func (po *SysModel) TableName() string {
	return "sys_model"
}