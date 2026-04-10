package agent

import (
	"time"

	"github.com/jinzhu/copier"

	entity "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/agent"
	po "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/agent"
)

// E2PSysAgentAdd entity数据转换成数据库po
func E2PSysAgentAdd(en *entity.SysAgent) *po.SysAgent {
	var po po.SysAgent
	po.CreatedAt = time.Now().UnixNano()
	if err := copier.Copy(&po, &en); err != nil {
		panic(any(err))
	}

	return &po
}

// E2PSysAgentDel entity数据转换成数据库po
func E2PSysAgentDel(en *entity.SysAgent) *po.SysAgent {
	var po po.SysAgent
	po.DeletedAt = time.Now().UnixMilli()

	return &po
}

// E2PSysAgentUpdate entity数据转换成数据库po
func E2PSysAgentUpdate(en *entity.SysAgent) *po.SysAgent {
	var po po.SysAgent
	if err := copier.Copy(&po, &en); err != nil {
		panic(any(err))
	}

	po.UpdatedAt = time.Now().UnixNano()
	return &po
}

// P2ESysAgent 数据库po转换成entity
func P2ESysAgent(p *po.SysAgent) *entity.SysAgent {
	var en entity.SysAgent
	if err := copier.Copy(&en, &p); err != nil {
		panic(any(err))
	}

	return &en
}

func P2ESysAgents(pos []*po.SysAgent) []*entity.SysAgent {
	ens := make([]*entity.SysAgent, 0)
	if len(pos) == 0 {
		return ens
	}

	for _, val := range pos {
		cfg := P2ESysAgent(val)
		ens = append(ens, cfg)
	}

	return ens
}
