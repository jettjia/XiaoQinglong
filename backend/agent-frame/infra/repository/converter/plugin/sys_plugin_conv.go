package plugin

import (
	"encoding/json"
	"time"

	"github.com/jinzhu/copier"

	entityPlugin "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/plugin"
	poPlugin "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/plugin"
)

// E2PInstanceAdd entity转换为PO（创建）
func E2PInstanceAdd(en *entityPlugin.PluginInstance) *poPlugin.PluginInstancePO {
	var po poPlugin.PluginInstancePO
	po.CreatedAt = time.Now().UnixMilli()
	if err := copier.Copy(&po, en); err != nil {
		panic(any(err))
	}
	// 序列化UserInfo
	if en.UserInfo != nil {
		userInfoBytes, _ := json.Marshal(en.UserInfo)
		po.UserInfo = string(userInfoBytes)
	}
	return &po
}

// E2PInstanceDel entity转换为PO（删除）
func E2PInstanceDel(en *entityPlugin.PluginInstance) *poPlugin.PluginInstancePO {
	var po poPlugin.PluginInstancePO
	po.DeletedBy = en.DeletedBy
	po.DeletedAt = time.Now().UnixMilli()
	return &po
}

// E2PInstanceUpdate entity转换为PO（更新）
func E2PInstanceUpdate(en *entityPlugin.PluginInstance) *poPlugin.PluginInstancePO {
	var po poPlugin.PluginInstancePO
	if err := copier.Copy(&po, en); err != nil {
		panic(any(err))
	}
	po.UpdatedAt = time.Now().UnixMilli()
	// 序列化UserInfo
	if en.UserInfo != nil {
		userInfoBytes, _ := json.Marshal(en.UserInfo)
		po.UserInfo = string(userInfoBytes)
	}
	return &po
}

// P2EInstance PO转换为entity
func P2EInstance(po *poPlugin.PluginInstancePO) *entityPlugin.PluginInstance {
	var entity entityPlugin.PluginInstance
	if err := copier.Copy(&entity, po); err != nil {
		panic(any(err))
	}
	// 反序列化UserInfo
	if po.UserInfo != "" {
		var userInfo entityPlugin.PluginUserInfo
		if err := json.Unmarshal([]byte(po.UserInfo), &userInfo); err == nil {
			entity.UserInfo = &userInfo
		}
	}
	return &entity
}

// P2EInstances PO列表转换为entity列表
func P2EInstances(pos []*poPlugin.PluginInstancePO) []*entityPlugin.PluginInstance {
	ens := make([]*entityPlugin.PluginInstance, 0)
	if len(pos) == 0 {
		return ens
	}
	for _, val := range pos {
		cfg := P2EInstance(val)
		ens = append(ens, cfg)
	}
	return ens
}

// E2POAuthStateAdd entity转换为PO（创建OAuthState）
func E2POAuthStateAdd(en *entityPlugin.OAuthState) *poPlugin.OAuthStatePO {
	var po poPlugin.OAuthStatePO
	po.CreatedAt = time.Now().UnixMilli()
	if err := copier.Copy(&po, en); err != nil {
		panic(any(err))
	}
	return &po
}

// P2EOAuthState PO转换为entity
func P2EOAuthState(po *poPlugin.OAuthStatePO) *entityPlugin.OAuthState {
	var entity entityPlugin.OAuthState
	if err := copier.Copy(&entity, po); err != nil {
		panic(any(err))
	}
	return &entity
}