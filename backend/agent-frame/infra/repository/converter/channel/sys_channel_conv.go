package channel

import (
	"encoding/json"
	"time"

	"github.com/jinzhu/copier"

	entity "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/channel"
	po "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/channel"
)

// E2PSysChannelAdd entity数据转换成数据库po
func E2PSysChannelAdd(en *entity.SysChannel) *po.SysChannel {
	var po po.SysChannel
	po.CreatedAt = time.Now().UnixNano()
	if err := copier.Copy(&po, &en); err != nil {
		panic(any(err))
	}
	// Config 序列化成 JSON 字符串
	if en.Config != nil {
		configBytes, _ := json.Marshal(en.Config)
		po.Config = string(configBytes)
	}

	return &po
}

// E2PSysChannelDel entity数据转换成数据库po
func E2PSysChannelDel(en *entity.SysChannel) *po.SysChannel {
	var po po.SysChannel
	po.DeletedAt = time.Now().UnixMilli()

	return &po
}

// E2PSysChannelUpdate entity数据转换成数据库po
func E2PSysChannelUpdate(en *entity.SysChannel) *po.SysChannel {
	var po po.SysChannel
	if err := copier.Copy(&po, &en); err != nil {
		panic(any(err))
	}
	// Config 序列化成 JSON 字符串
	if en.Config != nil {
		configBytes, _ := json.Marshal(en.Config)
		po.Config = string(configBytes)
	}

	po.UpdatedAt = time.Now().UnixNano()
	return &po
}

// P2ESysChannel 数据库po转换成entity
func P2ESysChannel(p *po.SysChannel) *entity.SysChannel {
	var en entity.SysChannel
	if err := copier.Copy(&en, &p); err != nil {
		panic(any(err))
	}
	// Config 从 JSON 字符串反序列化
	if p.Config != "" {
		json.Unmarshal([]byte(p.Config), &en.Config)
	}

	return &en
}

func P2ESysChannels(pos []*po.SysChannel) []*entity.SysChannel {
	ens := make([]*entity.SysChannel, 0)
	if len(pos) == 0 {
		return ens
	}

	for _, val := range pos {
		cfg := P2ESysChannel(val)
		ens = append(ens, cfg)
	}

	return ens
}