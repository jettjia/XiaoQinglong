package plugin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/jettjia/igo-pkg/pkg/xsql/builder"

	entityPlugin "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/plugin"
	"github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/repo/plugin"
)

// SysPluginSvc plugin domain service
//
//go:generate mockgen --source ./sys_plugin_svc.go --destination ./mock/mock_sys_plugin_svc.go --package mock
type SysPluginSvc interface {
	// CreateInstance 创建插件实例
	CreateInstance(ctx context.Context, instanceEn *entityPlugin.PluginInstance) (ulid string, err error)
	// DeleteInstance 删除插件实例
	DeleteInstance(ctx context.Context, instanceEn *entityPlugin.PluginInstance) (err error)
	// UpdateInstance 更新插件实例
	UpdateInstance(ctx context.Context, instanceEn *entityPlugin.PluginInstance) (err error)
	// FindInstanceById 根据ID查询实例
	FindInstanceById(ctx context.Context, ulid string) (instanceEn *entityPlugin.PluginInstance, err error)
	// FindInstanceByQuery 根据条件查询实例
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

type SysPlugin struct {
	pluginRepo *plugin.SysPlugin
}

func NewSysPluginSvc() *SysPlugin {
	return &SysPlugin{
		pluginRepo: plugin.NewSysPluginImpl(),
	}
}

// CreateInstance 创建插件实例
func (s *SysPlugin) CreateInstance(ctx context.Context, instanceEn *entityPlugin.PluginInstance) (ulid string, err error) {
	return s.pluginRepo.CreateInstance(ctx, instanceEn)
}

// DeleteInstance 删除插件实例
func (s *SysPlugin) DeleteInstance(ctx context.Context, instanceEn *entityPlugin.PluginInstance) (err error) {
	return s.pluginRepo.DeleteInstance(ctx, instanceEn)
}

// UpdateInstance 更新插件实例
func (s *SysPlugin) UpdateInstance(ctx context.Context, instanceEn *entityPlugin.PluginInstance) (err error) {
	return s.pluginRepo.UpdateInstance(ctx, instanceEn)
}

// FindInstanceById 根据ID查询实例
func (s *SysPlugin) FindInstanceById(ctx context.Context, ulid string) (instanceEn *entityPlugin.PluginInstance, err error) {
	return s.pluginRepo.FindInstanceById(ctx, ulid)
}

// FindInstanceByQuery 根据条件查询实例
func (s *SysPlugin) FindInstanceByQuery(ctx context.Context, queries []*builder.Query) (instanceEn *entityPlugin.PluginInstance, err error) {
	return s.pluginRepo.FindInstanceByQuery(ctx, queries)
}

// FindInstanceByUserAndPlugin 根据用户ID和插件ID查询实例
func (s *SysPlugin) FindInstanceByUserAndPlugin(ctx context.Context, userID, pluginID string) (instanceEn *entityPlugin.PluginInstance, err error) {
	return s.pluginRepo.FindInstanceByUserAndPlugin(ctx, userID, pluginID)
}

// FindAllInstance 查询用户所有插件实例
func (s *SysPlugin) FindAllInstance(ctx context.Context, queries []*builder.Query) (entries []*entityPlugin.PluginInstance, err error) {
	return s.pluginRepo.FindAllInstance(ctx, queries)
}

// FindPageInstance 分页查询插件实例
func (s *SysPlugin) FindPageInstance(ctx context.Context, queries []*builder.Query, reqPage *builder.PageData, reqSort *builder.SortData) ([]*entityPlugin.PluginInstance, *builder.PageData, error) {
	return s.pluginRepo.FindPageInstance(ctx, queries, reqPage, reqSort)
}

// CreateOAuthState 创建OAuth状态
func (s *SysPlugin) CreateOAuthState(ctx context.Context, stateEn *entityPlugin.OAuthState) (err error) {
	return s.pluginRepo.CreateOAuthState(ctx, stateEn)
}

// DeleteOAuthState 删除OAuth状态
func (s *SysPlugin) DeleteOAuthState(ctx context.Context, state string) (err error) {
	return s.pluginRepo.DeleteOAuthState(ctx, state)
}

// FindOAuthStateByState 根据state查询OAuth状态
func (s *SysPlugin) FindOAuthStateByState(ctx context.Context, state string) (stateEn *entityPlugin.OAuthState, err error) {
	return s.pluginRepo.FindOAuthStateByState(ctx, state)
}

// TokenEncryptionService Token加密服务接口
type TokenEncryptionService interface {
	// EncryptToken 加密token
	EncryptToken(ctx context.Context, token string) (encryptedToken, encryptedAES string, err error)
	// DecryptToken 解密token
	DecryptToken(ctx context.Context, encryptedToken, encryptedAES string) (token string, err error)
}

// SysTokenEncryption token加密服务实现
type SysTokenEncryption struct {
	rsaKeyMgr *plugin.RSAKeyManager
}

func NewSysTokenEncryption() *SysTokenEncryption {
	return &SysTokenEncryption{
		rsaKeyMgr: plugin.GetRSAKeyManager(),
	}
}

// EncryptToken 加密token
func (s *SysTokenEncryption) EncryptToken(ctx context.Context, token string) (encryptedToken, encryptedAES string, err error) {
	return s.rsaKeyMgr.EncryptToken(ctx, token)
}

// DecryptToken 解密token
func (s *SysTokenEncryption) DecryptToken(ctx context.Context, encryptedToken, encryptedAES string) (token string, err error) {
	return s.rsaKeyMgr.DecryptToken(ctx, encryptedToken, encryptedAES)
}

// OAuthStateService OAuth状态服务
type OAuthStateService struct {
	pluginRepo *plugin.SysPlugin
}

func NewOAuthStateService() *OAuthStateService {
	return &OAuthStateService{
		pluginRepo: plugin.NewSysPluginImpl(),
	}
}

// GenerateAndSaveOAuthState 生成并保存OAuth状态
func (s *OAuthStateService) GenerateAndSaveOAuthState(ctx context.Context, tenantID, userID, pluginID, callbackURL string, expiresInMinutes int) (state string, err error) {
	state = generateState()
	now := time.Now()
	oauthState := &entityPlugin.OAuthState{
		State:       state,
		TenantID:    tenantID,
		UserID:      userID,
		PluginID:    pluginID,
		CallbackURL: callbackURL,
		ExpiresAt:   now.Add(time.Duration(expiresInMinutes) * time.Minute).UnixMilli(),
		CreatedAt:   now.UnixMilli(),
	}

	err = s.pluginRepo.CreateOAuthState(ctx, oauthState)
	if err != nil {
		return "", err
	}
	return state, nil
}

// ValidateAndConsumeOAuthState 验证并消费OAuth状态（验证后删除）
func (s *OAuthStateService) ValidateAndConsumeOAuthState(ctx context.Context, state string) (oauthState *entityPlugin.OAuthState, err error) {
	oauthState, err = s.pluginRepo.FindOAuthStateByState(ctx, state)
	if err != nil {
		return nil, err
	}

	// 检查是否过期
	if time.Now().UnixMilli() > oauthState.ExpiresAt {
		return nil, ErrOAuthStateExpired
	}

	// 删除状态（一次性使用）
	err = s.pluginRepo.DeleteOAuthState(ctx, state)
	if err != nil {
		return nil, err
	}

	return oauthState, nil
}

// generateState 生成随机state
func generateState() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// Error definitions
var (
	ErrOAuthStateExpired = &OAuthError{Code: "OAUTH_STATE_EXPIRED", Message: "OAuth state expired"}
)

type OAuthError struct {
	Code    string
	Message string
}

func (e *OAuthError) Error() string {
	return e.Message
}