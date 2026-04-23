package plugin

import (
	"context"

	"github.com/jettjia/igo-pkg/pkg/xsql/builder"

	entityPlugin "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/plugin"
)

// IPluginRepo plugin repository interface
//
//go:generate mockgen --source ./i_plugin_repo.go --destination ./mock/mock_i_plugin_repo.go --package mock
type IPluginRepo interface {
	// CreateInstance 创建插件实例
	CreateInstance(ctx context.Context, instanceEn *entityPlugin.PluginInstance) (ulid string, err error)
	// DeleteInstance 删除插件实例
	DeleteInstance(ctx context.Context, instanceEn *entityPlugin.PluginInstance) (err error)
	// UpdateInstance 更新插件实例
	UpdateInstance(ctx context.Context, instanceEn *entityPlugin.PluginInstance) (err error)
	// FindInstanceById 根据ID查询实例
	FindInstanceById(ctx context.Context, ulid string) (instanceEn *entityPlugin.PluginInstance, err error)
	// FindInstanceByQuery 根据条件查询单个实例
	FindInstanceByQuery(ctx context.Context, queries []*builder.Query) (instanceEn *entityPlugin.PluginInstance, err error)
	// FindInstanceByUserAndPlugin 根据用户ID和插件ID查询实例
	FindInstanceByUserAndPlugin(ctx context.Context, userID, pluginID string) (instanceEn *entityPlugin.PluginInstance, err error)
	// FindAllInstance 查询用户所有插件实例
	FindAllInstance(ctx context.Context, queries []*builder.Query) (entries []*entityPlugin.PluginInstance, err error)
	// FindPageInstance 分页查询插件实例
	FindPageInstance(ctx context.Context, queries []*builder.Query, reqPage *builder.PageData, reqSort *builder.SortData) ([]*entityPlugin.PluginInstance, *builder.PageData, error)

	// CreateOAuthState 创建OAuth状态
	CreateOAuthState(ctx context.Context, stateEn *entityPlugin.OAuthState) (err error)
	// DeleteOAuthState 删除OAuth状态
	DeleteOAuthState(ctx context.Context, state string) (err error)
	// FindOAuthStateByState 根据state查询OAuth状态
	FindOAuthStateByState(ctx context.Context, state string) (stateEn *entityPlugin.OAuthState, err error)
}

// IRSAKeyManager RSA密钥管理接口
//
//go:generate mockgen --source ./i_plugin_repo.go --destination ./mock/mock_irsa_key_mgr.go --package mock
type IRSAKeyManager interface {
	// GetPublicKey 获取RSA公钥
	GetPublicKey(ctx context.Context) (publicKey string, err error)
	// EncryptWithRSA 用RSA公钥加密
	EncryptWithRSA(ctx context.Context, data string) (encrypted string, err error)
	// DecryptWithRSA 用RSA私钥解密
	DecryptWithRSA(ctx context.Context, encrypted string) (data string, err error)
}