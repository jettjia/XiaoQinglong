package plugin

import (
	"github.com/jinzhu/copier"

	dtoPlugin "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/plugin"
	entityPlugin "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/plugin"
)

// SysPluginDto 请求参数
type SysPluginDto struct {
}

// NewSysPluginDto NewSysPluginDto
func NewSysPluginDto() *SysPluginDto {
	return &SysPluginDto{}
}

//////////////////////////////////////////////////////////////////
// dto to entity

// D2EDeleteInstance dto转换成entity
func (a *SysPluginDto) D2EDeleteInstance(dto *dtoPlugin.DeleteInstanceReq) *entityPlugin.PluginInstance {
	var rspEn entityPlugin.PluginInstance
	rspEn.Ulid = dto.Ulid
	return &rspEn
}

// D2ERefreshInstance dto转换成entity
func (a *SysPluginDto) D2ERefreshInstance(dto *dtoPlugin.RefreshTokenReq) *entityPlugin.PluginInstance {
	var rspEn entityPlugin.PluginInstance
	rspEn.Ulid = dto.Ulid
	return &rspEn
}

// D2EStartAuth dto转换成entity
func (a *SysPluginDto) D2EStartAuth(dto *dtoPlugin.StartAuthReq) *entityPlugin.OAuthState {
	var rspEn entityPlugin.OAuthState
	rspEn.PluginID = dto.PluginID
	rspEn.CallbackURL = dto.CallbackURL
	return &rspEn
}

//////////////////////////////////////////////////////////////////
// entity to dto

// E2DGetPluginsRsp entity转换成dto
func (a *SysPluginDto) E2DGetPluginsRsp(plugins []entityPlugin.PluginDefinition, instances []*entityPlugin.PluginInstance) *dtoPlugin.GetPluginsRsp {
	rsp := &dtoPlugin.GetPluginsRsp{}
	rsp.Plugins = make([]dtoPlugin.PluginItem, 0)

	// 构建instance map
	instanceMap := make(map[string]*entityPlugin.PluginInstance)
	for _, ins := range instances {
		instanceMap[ins.PluginID] = ins
	}

	for _, plugin := range plugins {
		item := dtoPlugin.PluginItem{
			ID:          plugin.ID,
			Name:        plugin.Name,
			Icon:        plugin.Icon,
			Description: plugin.Description,
			AuthType:    plugin.AuthType,
			Version:     plugin.Version,
			Author:      plugin.Author,
			Status:      "available",
		}

		// 检查是否已安装或已授权
		if ins, ok := instanceMap[plugin.ID]; ok {
			item.InstanceID = ins.Ulid
			item.Status = "installed"
			if ins.Status == "active" {
				item.Status = "authorized"
			}
		}

		rsp.Plugins = append(rsp.Plugins, item)
	}

	return rsp
}

// E2DGetUserInstancesRsp entity转换成dto
func (a *SysPluginDto) E2DGetUserInstancesRsp(entries []*entityPlugin.PluginInstance) *dtoPlugin.GetUserInstancesRsp {
	rsp := &dtoPlugin.GetUserInstancesRsp{}
	rsp.Instances = make([]dtoPlugin.InstanceItem, 0)

	for _, en := range entries {
		item := dtoPlugin.InstanceItem{
			Ulid:         en.Ulid,
			PluginID:     en.PluginID,
			Status:       en.Status,
			AuthorizedAt: en.AuthorizedAt,
			ExpiresAt:    en.ExpiresAt,
		}

		if en.UserInfo != nil {
			item.UserInfo = &dtoPlugin.PluginUserInfo{
				OpenID: en.UserInfo.OpenID,
				Name:   en.UserInfo.Name,
				Avatar: en.UserInfo.Avatar,
				Email:  en.UserInfo.Email,
			}
		}

		rsp.Instances = append(rsp.Instances, item)
	}

	return rsp
}

// E2DGetInstanceByIdRsp entity转换成dto
func (a *SysPluginDto) E2DGetInstanceByIdRsp(en *entityPlugin.PluginInstance) *dtoPlugin.GetInstanceByIdRsp {
	rsp := &dtoPlugin.GetInstanceByIdRsp{}
	if err := copier.Copy(rsp, en); err != nil {
		panic(any(err))
	}

	if en.UserInfo != nil {
		rsp.UserInfo = &dtoPlugin.PluginUserInfo{
			OpenID: en.UserInfo.OpenID,
			Name:   en.UserInfo.Name,
			Avatar: en.UserInfo.Avatar,
			Email:  en.UserInfo.Email,
		}
	}

	return rsp
}

// E2DStartAuthRsp entity转换成dto
func (a *SysPluginDto) E2DStartAuthRsp(authURL, state string) *dtoPlugin.StartAuthRsp {
	return &dtoPlugin.StartAuthRsp{
		AuthType: "oauth2",
		AuthURL:  authURL,
		State:    state,
	}
}

// E2DStartAuthRspDevice entity转换成dto (device flow)
func (a *SysPluginDto) E2DStartAuthRspDevice(deviceCode, userCode, verificationURL, state string, expiresIn, interval int) *dtoPlugin.StartAuthRsp {
	return &dtoPlugin.StartAuthRsp{
		AuthType:       "device",
		DeviceCode:     deviceCode,
		UserCode:       userCode,
		VerificationURL: verificationURL,
		State:          state,
		ExpiresIn:      expiresIn,
		Interval:       interval,
	}
}

// E2DPollAuthRspPending entity转换成dto
func (a *SysPluginDto) E2DPollAuthRspPending() *dtoPlugin.PollAuthRsp {
	return &dtoPlugin.PollAuthRsp{
		Status: "pending",
	}
}

// E2DPollAuthRspAuthorized entity转换成dto
func (a *SysPluginDto) E2DPollAuthRspAuthorized(instanceID string, userInfo *entityPlugin.PluginUserInfo) *dtoPlugin.PollAuthRsp {
	rsp := &dtoPlugin.PollAuthRsp{
		Status:     "authorized",
		InstanceID: instanceID,
	}

	if userInfo != nil {
		rsp.UserInfo = &dtoPlugin.PluginUserInfo{
			OpenID: userInfo.OpenID,
			Name:   userInfo.Name,
			Avatar: userInfo.Avatar,
			Email:  userInfo.Email,
		}
	}

	return rsp
}

// E2DRefreshTokenRsp entity转换成dto
func (a *SysPluginDto) E2DRefreshTokenRsp(status string) *dtoPlugin.RefreshTokenRsp {
	return &dtoPlugin.RefreshTokenRsp{
		Status: status,
	}
}

// E2DPublicKeyRsp entity转换成dto
func (a *SysPluginDto) E2DPublicKeyRsp(publicKey string) *dtoPlugin.GetPublicKeyRsp {
	return &dtoPlugin.GetPublicKeyRsp{
		PublicKey: publicKey,
	}
}