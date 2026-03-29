package knowledge_base

import (
	"time"

	"github.com/jinzhu/copier"

	entity "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/knowledge_base"
	po "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/knowledge_base"
)

// E2PSysKnowledgeBaseAdd entity数据转换成数据库po
func E2PSysKnowledgeBaseAdd(en *entity.SysKnowledgeBase) *po.SysKnowledgeBase {
	var po po.SysKnowledgeBase
	po.CreatedAt = time.Now().UnixNano()
	if err := copier.Copy(&po, &en); err != nil {
		panic(any(err))
	}

	return &po
}

// E2PSysKnowledgeBaseDel entity数据转换成数据库po
func E2PSysKnowledgeBaseDel(en *entity.SysKnowledgeBase) *po.SysKnowledgeBase {
	var po po.SysKnowledgeBase
	po.DeletedAt = time.Now().UnixMilli()

	return &po
}

// E2PSysKnowledgeBaseUpdate entity数据转换成数据库po
func E2PSysKnowledgeBaseUpdate(en *entity.SysKnowledgeBase) *po.SysKnowledgeBase {
	var po po.SysKnowledgeBase
	if err := copier.Copy(&po, &en); err != nil {
		panic(any(err))
	}

	po.UpdatedAt = time.Now().UnixNano()
	return &po
}

// P2ESysKnowledgeBase 数据库po转换成entity
func P2ESysKnowledgeBase(p *po.SysKnowledgeBase) *entity.SysKnowledgeBase {
	var en entity.SysKnowledgeBase
	if err := copier.Copy(&en, &p); err != nil {
		panic(any(err))
	}

	return &en
}

func P2ESysKnowledgeBases(pos []*po.SysKnowledgeBase) []*entity.SysKnowledgeBase {
	ens := make([]*entity.SysKnowledgeBase, 0)
	if len(pos) == 0 {
		return ens
	}

	for _, val := range pos {
		cfg := P2ESysKnowledgeBase(val)
		ens = append(ens, cfg)
	}

	return ens
}
