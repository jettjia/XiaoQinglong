package plugin

import (
	"encoding/json"
	"time"

	"github.com/jettjia/igo-pkg/pkg/util"
	"gorm.io/gorm"
)

// PluginInstancePO 插件实例持久化对象
type PluginInstancePO struct {
	Ulid           string         `gorm:"column:ulid;primaryKey;type:varchar(128);comment:ulid;" json:"ulid"`
	CreatedAt      int64          `gorm:"column:created_at;autoCreateTime:milli;type:bigint;comment:创建时间;" json:"created_at"`
	UpdatedAt      int64          `gorm:"column:updated_at;autoUpdateTime:milli;type:bigint;comment:修改时间;" json:"updated_at"`
	DeletedAt      int64          `gorm:"column:deleted_at;autoDeletedTime:milli;type:bigint;comment:删除时间;" json:"deleted_at"`
	DeletedBy      string         `gorm:"column:deleted_by;type:varchar(128);comment:删除者;" json:"deleted_by"`
	CreatedBy      string         `gorm:"column:created_by;type:varchar(128);comment:创建者;" json:"created_by"`
	UpdatedBy      string         `gorm:"column:updated_by;type:varchar(128);comment:修改者;" json:"updated_by"`
	TenantID       string         `gorm:"column:tenant_id;type:varchar(64);comment:租户ID;" json:"tenant_id"`
	UserID         string         `gorm:"column:user_id;type:varchar(64);comment:用户ID;" json:"user_id"`
	PluginID       string         `gorm:"column:plugin_id;type:varchar(64);comment:插件ID;" json:"plugin_id"`
	PluginVersion  string         `gorm:"column:plugin_version;type:varchar(32);comment:插件版本;" json:"plugin_version"`
	Status         string         `gorm:"column:status;type:varchar(32);default:active;comment:状态:active/revoked/expired;" json:"status"`
	EncryptedToken string         `gorm:"column:encrypted_token;type:text;comment:加密的token;" json:"encrypted_token"`
	EncryptedAES   string         `gorm:"column:encrypted_aes;type:text;comment:加密的AES密钥;" json:"encrypted_aes"`
	TokenVersion   int            `gorm:"column:token_version;type:int;default:1;comment:token版本号;" json:"token_version"`
	Config         string         `gorm:"column:config;type:json;comment:插件配置;" json:"config"`
	UserInfo       string         `gorm:"column:user_info;type:json;comment:用户信息JSON;" json:"user_info"`
	AuthorizedAt   int64          `gorm:"column:authorized_at;type:bigint;comment:授权时间;" json:"authorized_at"`
	ExpiresAt      *int64         `gorm:"column:expires_at;type:bigint;comment:过期时间;" json:"expires_at"`
}

func (po *PluginInstancePO) BeforeCreate(tx *gorm.DB) (err error) {
	po.Ulid = util.Ulid()
	return
}

func (po *PluginInstancePO) TableName() string {
	return "plugin_instance"
}

// OAuthStatePO OAuth状态持久化对象
type OAuthStatePO struct {
	State       string `gorm:"column:state;primaryKey;type:varchar(128);comment:OAuth state;" json:"state"`
	TenantID    string `gorm:"column:tenant_id;type:varchar(64);comment:租户ID;" json:"tenant_id"`
	UserID      string `gorm:"column:user_id;type:varchar(64);comment:用户ID;" json:"user_id"`
	PluginID    string `gorm:"column:plugin_id;type:varchar(64);comment:插件ID;" json:"plugin_id"`
	CallbackURL string `gorm:"column:callback_url;type:varchar(512);comment:回调URL;" json:"callback_url"`
	ExpiresAt   int64  `gorm:"column:expires_at;type:bigint;comment:过期时间;" json:"expires_at"`
	CreatedAt   int64  `gorm:"column:created_at;autoCreateTime:milli;type:bigint;comment:创建时间;" json:"created_at"`
}

func (po *OAuthStatePO) TableName() string {
	return "oauth_state"
}

// PluginUserInfoPO 用户信息PO
type PluginUserInfoPO struct {
	OpenID string `json:"open_id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
	Email  string `json:"email"`
}

// ParseUserInfo 解析用户信息
func (po *PluginInstancePO) ParseUserInfo() json.RawMessage {
	if po.UserInfo == "" {
		return nil
	}
	return json.RawMessage(po.UserInfo)
}

// GetConfigMap 获取配置Map
func (po *PluginInstancePO) GetConfigMap() map[string]string {
	if po.Config == "" {
		return make(map[string]string)
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(po.Config), &m); err != nil {
		return make(map[string]string)
	}
	return m
}

// IsExpired 检查是否过期
func (po *PluginInstancePO) IsExpired() bool {
	if po.ExpiresAt == nil {
		return false
	}
	return time.Now().UnixMilli() > *po.ExpiresAt
}