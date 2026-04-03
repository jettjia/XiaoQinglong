package channel

import (
	"github.com/jettjia/igo-pkg/pkg/util"
	"gorm.io/gorm"
)

type SysChannel struct {
	Ulid        string `gorm:"column:ulid;primaryKey;type:varchar(128);comment:ulid;" json:"ulid"`
	CreatedAt   int64  `gorm:"column:created_at;autoCreateTime:milli;type:bigint;comment:创建时间;" json:"created_at"`
	UpdatedAt   int64  `gorm:"column:updated_at;autoUpdateTime:milli;type:bigint;comment:修改时间;" json:"updated_at"`
	DeletedAt   int64  `gorm:"column:deleted_at;type:bigint;comment:删除时间;" json:"deleted_at"`
	CreatedBy   string `gorm:"column:created_by;type:varchar(128);comment:创建者;" json:"created_by"`
	UpdatedBy   string `gorm:"column:updated_by;type:varchar(128);comment:修改者;" json:"updated_by"`
	Name        string `gorm:"column:name;type:varchar(100);comment:渠道名称;" json:"name"`
	Code        string `gorm:"column:code;type:varchar(50);comment:渠道代码;" json:"code"`
	Description string `gorm:"column:description;type:text;comment:描述;" json:"description"`
	Icon        string `gorm:"column:icon;type:varchar(50);comment:图标;" json:"icon"`
	Enabled     bool   `gorm:"column:enabled;type:boolean;default:true;comment:是否启用;" json:"enabled"`
	Sort        int    `gorm:"column:sort;type:int;default:0;comment:排序;" json:"sort"`
	Config      string `gorm:"column:config;type:text;comment:渠道配置;serializer:json" json:"config"` // JSON 配置
}

func (po *SysChannel) BeforeCreate(tx *gorm.DB) (err error) {
	po.Ulid = util.Ulid()
	return
}

func (po *SysChannel) TableName() string {
	return "sys_channel"
}