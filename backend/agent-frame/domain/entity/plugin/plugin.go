package plugin

import (
	"time"
)

// PluginInstance - 插件实例
type PluginInstance struct {
	Ulid           string         `json:"ulid"`           // ulid
	CreatedAt      int64          `json:"created_at"`     // 创建时间
	UpdatedAt      int64          `json:"updated_at"`     // 更新时间
	DeletedAt      int64          `json:"deleted_at"`     // 删除时间
	DeletedBy      string         `json:"deleted_by"`     // 删除者
	CreatedBy      string         `json:"created_by"`     // 创建者
	UpdatedBy      string         `json:"updated_by"`     // 更新者
	TenantID       string         `json:"tenant_id"`      // 租户ID
	UserID         string         `json:"user_id"`        // 用户ID
	PluginID       string         `json:"plugin_id"`      // 插件ID
	PluginVersion  string         `json:"plugin_version"` // 插件版本
	Status         string         `json:"status"`         // active, revoked, expired
	EncryptedToken string         `json:"encrypted_token"` // 加密的token (RSA + AES双重加密)
	EncryptedAES   string         `json:"encrypted_aes"`  // 加密的AES密钥 (RSA加密)
	TokenVersion   int            `json:"token_version"`  // token版本号
	Config         string         `json:"config"`         // 插件配置JSON
	UserInfo       *PluginUserInfo `json:"user_info"`      // 用户信息
	AuthorizedAt   int64          `json:"authorized_at"`  // 授权时间
	ExpiresAt      *int64         `json:"expires_at"`     // 过期时间
}

// OAuthState - OAuth 状态
type OAuthState struct {
	State      string `json:"state"`       // OAuth state
	TenantID   string `json:"tenant_id"`  // 租户ID
	UserID     string `json:"user_id"`    // 用户ID
	PluginID   string `json:"plugin_id"`  // 插件ID
	CallbackURL string `json:"callback_url"` // 回调URL
	ExpiresAt  int64  `json:"expires_at"` // 过期时间
	CreatedAt  int64  `json:"created_at"` // 创建时间
}

// PluginDefinition - 插件定义
type PluginDefinition struct {
	ID          string `json:"id"`           // 插件唯一标识
	Name        string `json:"name"`         // 插件名称
	Icon        string `json:"icon"`         // 插件图标
	Description string `json:"description"`  // 插件描述
	AuthType    string `json:"auth_type"`    // 授权类型: oauth2, device
	Version     string `json:"version"`      // 插件版本
	Author      string `json:"author"`      // 插件作者
	Status      string `json:"status"`       // 状态: available, unavailable
}

// PluginUserInfo - 用户信息
type PluginUserInfo struct {
	OpenID string `json:"open_id"` // 数据源用户ID
	Name   string `json:"name"`   // 用户名称
	Avatar string `json:"avatar"` // 用户头像
	Email  string `json:"email"`  // 用户邮箱
}

// GetAuthorizedAt 获权时间
func (p *PluginInstance) GetAuthorizedAt() time.Time {
	return time.Unix(0, p.AuthorizedAt)
}

// GetExpiresAt 过期时间
func (p *PluginInstance) GetExpiresAt() *time.Time {
	if p.ExpiresAt == nil {
		return nil
	}
	t := time.Unix(0, *p.ExpiresAt)
	return &t
}

// GetConfigMap 获取配置Map
func (p *PluginInstance) GetConfigMap() map[string]string {
	if p.Config == "" {
		return make(map[string]string)
	}
	// 实际实现会用json解析
	return make(map[string]string)
}