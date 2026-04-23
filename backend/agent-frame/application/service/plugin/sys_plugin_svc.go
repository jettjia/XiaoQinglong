package plugin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	assPlugin "github.com/jettjia/xiaoqinglong/agent-frame/application/assembler/plugin"
	dtoPlugin "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/plugin"
	entityPlugin "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/plugin"
	srvPlugin "github.com/jettjia/xiaoqinglong/agent-frame/domain/srv/plugin"
	"github.com/jettjia/xiaoqinglong/agent-frame/infra/plugins/feishu"
	"github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/repo/plugin"
)

type SysPluginService struct {
	sysPluginDto *assPlugin.SysPluginDto
}

func NewSysPluginService() *SysPluginService {
	return &SysPluginService{
		sysPluginDto: assPlugin.NewSysPluginDto(),
	}
}

// GetPlugins 获取插件列表
func (s *SysPluginService) GetPlugins(ctx context.Context) (*dtoPlugin.GetPluginsRsp, error) {
	plugins := []entityPlugin.PluginDefinition{
		{ID: "feishu", Name: "飞书", Icon: "📦", Description: "搜索飞书文档、知识库", AuthType: "device", Version: "1.0.0", Author: "xiaoqinglong", Status: "available"},
		{ID: "tengxun_docs", Name: "腾讯文档", Icon: "📄", Description: "搜索腾讯文档、表格", AuthType: "oauth2", Version: "1.0.0", Author: "xiaoqinglong", Status: "available"},
		{ID: "dingtalk", Name: "钉钉文档", Icon: "💬", Description: "搜索钉钉文档、知识库", AuthType: "device", Version: "1.0.0", Author: "xiaoqinglong", Status: "available"},
	}

	instances, err := s.findUserInstances(ctx)
	if err != nil {
		return nil, err
	}

	return s.sysPluginDto.E2DGetPluginsRsp(plugins, instances), nil
}

// GetUserInstances 获取用户插件实例
func (s *SysPluginService) GetUserInstances(ctx context.Context) (*dtoPlugin.GetUserInstancesRsp, error) {
	instances, err := s.findUserInstances(ctx)
	if err != nil {
		return nil, err
	}

	return s.sysPluginDto.E2DGetUserInstancesRsp(instances), nil
}

// GetInstanceById 获取实例详情
func (s *SysPluginService) GetInstanceById(ctx context.Context, ulid string) (*dtoPlugin.GetInstanceByIdRsp, error) {
	instance, err := s.getDomainSvc().FindInstanceById(ctx, ulid)
	if err != nil {
		return nil, err
	}

	return s.sysPluginDto.E2DGetInstanceByIdRsp(instance), nil
}

// DeleteInstance 删除插件实例
func (s *SysPluginService) DeleteInstance(ctx context.Context, req *dtoPlugin.DeleteInstanceReq, deletedBy string) error {
	instanceEn := s.sysPluginDto.D2EDeleteInstance(req)
	instanceEn.DeletedBy = deletedBy
	return s.getDomainSvc().DeleteInstance(ctx, instanceEn)
}

// RefreshToken 刷新令牌
func (s *SysPluginService) RefreshToken(ctx context.Context, req *dtoPlugin.RefreshTokenReq) (*dtoPlugin.RefreshTokenRsp, error) {
	instance, err := s.getDomainSvc().FindInstanceById(ctx, req.Ulid)
	if err != nil {
		return nil, err
	}

	// 解密 token
	rsaKeyMgr := plugin.GetRSAKeyManager()
	tokenData, err := s.decryptTokenData(ctx, instance, rsaKeyMgr)
	if err != nil {
		return &dtoPlugin.RefreshTokenRsp{Status: "expired"}, nil
	}

	// 检查是否过期
	if tokenData.ExpiresAt < time.Now().UnixMilli() {
		// 尝试刷新 token
		if tokenData.RefreshToken != "" {
			feishuAuth := feishu.NewAuthHandler()
			newToken, err := feishuAuth.RefreshToken(ctx, tokenData.RefreshToken)
			if err == nil && newToken.AccessToken != "" {
				// 更新存储的 token
				err = s.updateInstanceToken(ctx, instance, newToken, rsaKeyMgr)
				if err == nil {
					return &dtoPlugin.RefreshTokenRsp{Status: "active"}, nil
				}
			}
		}
		return &dtoPlugin.RefreshTokenRsp{Status: "expired"}, nil
	}

	return &dtoPlugin.RefreshTokenRsp{Status: "active"}, nil
}

// StartAuth 开始授权（Device Flow）
func (s *SysPluginService) StartAuth(ctx context.Context, req *dtoPlugin.StartAuthReq) (*dtoPlugin.StartAuthRsp, error) {
	tenantID := getValueFromCtx(ctx, "tenant_id")
	userID := getValueFromCtx(ctx, "user_id")

	// 生成 state
	state := generateState()

	switch req.PluginID {
	case "feishu":
		return s.startFeishuAuth(ctx, req, tenantID, userID, state)
	default:
		// 其他插件暂时不支持
		return nil, fmt.Errorf("unsupported plugin: %s", req.PluginID)
	}
}

// startFeishuAuth 飞书设备授权
func (s *SysPluginService) startFeishuAuth(ctx context.Context, req *dtoPlugin.StartAuthReq, tenantID, userID, state string) (*dtoPlugin.StartAuthRsp, error) {
	feishuAuth := feishu.NewAuthHandler()

	// 请求设备授权
	feishuPlugin := feishu.NewPlugin()
	deviceAuthResp, err := feishuAuth.RequestDeviceAuthorization(feishuPlugin.DefaultScopes())
	if err != nil {
		return nil, fmt.Errorf("feishu device auth failed: %w", err)
	}

	// 保存 OAuth 状态到数据库
	oauthState := &entityPlugin.OAuthState{
		State:       state,
		TenantID:    tenantID,
		UserID:      userID,
		PluginID:    req.PluginID,
		CallbackURL: deviceAuthResp.DeviceCode, // 临时存储 device_code
		ExpiresAt:   time.Now().Add(time.Duration(deviceAuthResp.ExpiresIn) * time.Second).UnixMilli(),
		CreatedAt:  time.Now().UnixMilli(),
	}
	err = s.getDomainSvc().CreateOAuthState(ctx, oauthState)
	if err != nil {
		return nil, fmt.Errorf("failed to save oauth state: %w", err)
	}

	return &dtoPlugin.StartAuthRsp{
		AuthType:        "device",
		DeviceCode:      deviceAuthResp.DeviceCode,
		UserCode:        deviceAuthResp.UserCode,
		VerificationURL: deviceAuthResp.VerificationURIComplete,
		State:           state,
		ExpiresIn:       deviceAuthResp.ExpiresIn,
		Interval:        deviceAuthResp.Interval,
	}, nil
}

// PollAuth 轮询授权状态
func (s *SysPluginService) PollAuth(ctx context.Context, req *dtoPlugin.PollAuthReq) (*dtoPlugin.PollAuthRsp, error) {
	// 查询 OAuth 状态
	oauthState, err := s.getDomainSvc().FindOAuthStateByState(ctx, req.State)
	if err != nil {
		return nil, fmt.Errorf("oauth state not found")
	}

	// 检查是否过期
	if time.Now().UnixMilli() > oauthState.ExpiresAt {
		return &dtoPlugin.PollAuthRsp{Status: "expired"}, nil
	}

	switch oauthState.PluginID {
	case "feishu":
		return s.pollFeishuAuth(ctx, oauthState)
	default:
		return &dtoPlugin.PollAuthRsp{Status: "pending"}, nil
	}
}

// pollFeishuAuth 轮询飞书授权状态
func (s *SysPluginService) pollFeishuAuth(ctx context.Context, oauthState *entityPlugin.OAuthState) (*dtoPlugin.PollAuthRsp, error) {
	// 从 oauthState 的 CallbackURL 中提取 device_code
	// device_code 存储在 encrypted_token 字段（临时存储）
	deviceCode := oauthState.CallbackURL // TODO: 更好的存储方式

	fmt.Printf("[DEBUG] pollFeishuAuth: state=%s, deviceCode=%s\n", oauthState.State, deviceCode)

	feishuAuth := feishu.NewAuthHandler()

	// 轮询 token
	tokenData, err := feishuAuth.PollDeviceToken(ctx, deviceCode, 5, 300)
	if err != nil {
		// 授权失败
		if err.Error() == "authorization denied by user" {
			return &dtoPlugin.PollAuthRsp{Status: "denied"}, nil
		}
		// 可能是仍在等待，返回pending
		fmt.Printf("[DEBUG] PollDeviceToken error (pending): %v\n", err)
		return &dtoPlugin.PollAuthRsp{Status: "pending"}, nil
	}

	// tokenData为空也视为pending
	if tokenData == nil || tokenData.AccessToken == "" {
		fmt.Printf("[DEBUG] PollDeviceToken returned empty token, treating as pending\n")
		return &dtoPlugin.PollAuthRsp{Status: "pending"}, nil
	}

	fmt.Printf("[DEBUG] PollDeviceToken success: accessToken=%s...\n", tokenData.AccessToken[:20])

	// 授权成功，获取用户信息
	userInfo, err := feishuAuth.GetUserInfo(ctx, tokenData.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}

	// 加密并存储 token
	instanceID, err := s.saveFeishuToken(ctx, oauthState, tokenData, userInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to save token: %w", err)
	}

	// 删除 OAuth 状态
	_ = s.getDomainSvc().DeleteOAuthState(ctx, oauthState.State)

	return &dtoPlugin.PollAuthRsp{
		Status:     "authorized",
		InstanceID: instanceID,
		UserInfo: &dtoPlugin.PluginUserInfo{
			OpenID: userInfo.OpenID,
			Name:   userInfo.Name,
			Avatar: userInfo.Avatar,
			Email:  userInfo.Email,
		},
	}, nil
}

// saveFeishuToken 保存飞书token
func (s *SysPluginService) saveFeishuToken(ctx context.Context, oauthState *entityPlugin.OAuthState, tokenData *feishu.DeviceFlowTokenData, userInfo *feishu.UserInfo) (string, error) {
	rsaKeyMgr := plugin.GetRSAKeyManager()

	// 加密 token
	encryptedToken, encryptedAES, err := rsaKeyMgr.EncryptToken(ctx, tokenData.AccessToken)
	if err != nil {
		return "", err
	}

	// 存储 refresh_token（也加密）
	encryptedRefreshToken, encryptedRefreshAES := "", ""
	if tokenData.RefreshToken != "" {
		encryptedRefreshToken, encryptedRefreshAES, err = rsaKeyMgr.EncryptToken(ctx, tokenData.RefreshToken)
		if err != nil {
			return "", err
		}
	}

	// 构建实例
	now := time.Now().UnixMilli()
	expiresAt := now + int64(tokenData.ExpiresIn)*1000

	instance := &entityPlugin.PluginInstance{
		TenantID:        oauthState.TenantID,
		UserID:          oauthState.UserID,
		PluginID:        oauthState.PluginID,
		PluginVersion:   "1.0.0",
		Status:          "active",
		EncryptedToken:  encryptedToken,
		EncryptedAES:    encryptedAES,
		TokenVersion:    1,
		AuthorizedAt:     now,
		ExpiresAt:        &expiresAt,
		UserInfo: &entityPlugin.PluginUserInfo{
			OpenID: userInfo.OpenID,
			Name:   userInfo.Name,
			Avatar: userInfo.Avatar,
			Email:  userInfo.Email,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	// 存储到数据库
	ulid, err := s.getDomainSvc().CreateInstance(ctx, instance)
	if err != nil {
		return "", err
	}

	// TODO: 单独存储 refresh_token（暂时放在 config 字段）
	refreshTokenData, _ := json.Marshal(map[string]string{
		"refresh_token":        encryptedRefreshToken,
		"refresh_token_aes":    encryptedRefreshAES,
		"refresh_expires_in":  fmt.Sprintf("%d", tokenData.RefreshExpiresIn),
	})
	instance.Ulid = ulid
	instance.Config = string(refreshTokenData)
	_ = s.getDomainSvc().UpdateInstance(ctx, instance)

	return ulid, nil
}

// updateInstanceToken 更新实例token
func (s *SysPluginService) updateInstanceToken(ctx context.Context, instance *entityPlugin.PluginInstance, tokenResp *feishu.TokenResponse, rsaKeyMgr *plugin.RSAKeyManager) error {
	encryptedToken, encryptedAES, err := rsaKeyMgr.EncryptToken(ctx, tokenResp.AccessToken)
	if err != nil {
		return err
	}

	encryptedRefreshToken, encryptedRefreshAES := "", ""
	if tokenResp.RefreshToken != "" {
		encryptedRefreshToken, encryptedRefreshAES, err = rsaKeyMgr.EncryptToken(ctx, tokenResp.RefreshToken)
		if err != nil {
			return err
		}
	}

	now := time.Now().UnixMilli()
	expiresAt := now + int64(tokenResp.ExpiresIn)*1000

	instance.EncryptedToken = encryptedToken
	instance.EncryptedAES = encryptedAES
	instance.ExpiresAt = &expiresAt
	instance.UpdatedAt = now
	instance.TokenVersion++

	// 更新 refresh_token
	refreshTokenData, _ := json.Marshal(map[string]string{
		"refresh_token":       encryptedRefreshToken,
		"refresh_token_aes":  encryptedRefreshAES,
		"refresh_expires_in": fmt.Sprintf("%d", tokenResp.RefreshExpiresIn),
	})
	instance.Config = string(refreshTokenData)

	return s.getDomainSvc().UpdateInstance(ctx, instance)
}

// decryptTokenData 解密token数据
func (s *SysPluginService) decryptTokenData(ctx context.Context, instance *entityPlugin.PluginInstance, rsaKeyMgr *plugin.RSAKeyManager) (*TokenData, error) {
	if instance.EncryptedToken == "" {
		return nil, fmt.Errorf("no token stored")
	}

	token, err := rsaKeyMgr.DecryptToken(ctx, instance.EncryptedToken, instance.EncryptedAES)
	if err != nil {
		return nil, err
	}

	td := &TokenData{
		AccessToken: token,
	}

	// 解析 refresh_token
	if instance.Config != "" {
		var refreshData map[string]string
		if err := json.Unmarshal([]byte(instance.Config), &refreshData); err == nil {
			if refreshToken, ok := refreshData["refresh_token"]; ok && refreshToken != "" {
				if refreshAES, ok := refreshData["refresh_token_aes"]; ok {
					td.RefreshToken, _ = rsaKeyMgr.DecryptToken(ctx, refreshToken, refreshAES)
				}
			}
		}
	}

	if instance.ExpiresAt != nil {
		td.ExpiresAt = *instance.ExpiresAt
	}

	return td, nil
}

// TokenData Token数据
type TokenData struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt   int64
}

// GetPublicKey 获取RSA公钥
func (s *SysPluginService) GetPublicKey(ctx context.Context) (*dtoPlugin.GetPublicKeyRsp, error) {
	rsaKeyMgr := plugin.GetRSAKeyManager()
	publicKey, err := rsaKeyMgr.GetPublicKey(ctx)
	if err != nil {
		return nil, err
	}

	return s.sysPluginDto.E2DPublicKeyRsp(publicKey), nil
}

// findUserInstances 查询当前用户的实例
func (s *SysPluginService) findUserInstances(ctx context.Context) ([]*entityPlugin.PluginInstance, error) {
	userID := getValueFromCtx(ctx, "user_id")

	instance, err := s.getDomainSvc().FindInstanceByUserAndPlugin(ctx, userID, "")
	if err != nil {
		return []*entityPlugin.PluginInstance{}, nil
	}
	if instance == nil {
		return []*entityPlugin.PluginInstance{}, nil
	}
	return []*entityPlugin.PluginInstance{instance}, nil
}

// getValueFromCtx 从context获取值
func getValueFromCtx(ctx context.Context, key string) string {
	if val := ctx.Value(key); val != nil {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// generateState 生成随机state
func generateState() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// getDomainSvc 获取domain service
func (s *SysPluginService) getDomainSvc() *srvPlugin.SysPlugin {
	return srvPlugin.NewSysPluginSvc()
}
