package skill

import (
	"time"

	"github.com/jinzhu/copier"

	entity "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/skill"
	po "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/skill"
)

// E2PSysSkillAdd entity数据转换成数据库po
func E2PSysSkillAdd(en *entity.SysSkill) *po.SysSkill {
	var po po.SysSkill
	po.CreatedAt = time.Now().UnixNano()
	if err := copier.Copy(&po, &en); err != nil {
		panic(any(err))
	}

	return &po
}

// E2PSysSkillDel entity数据转换成数据库po
func E2PSysSkillDel(en *entity.SysSkill) *po.SysSkill {
	var po po.SysSkill
	po.DeletedAt = time.Now().UnixMilli()

	return &po
}

// E2PSysSkillUpdate entity数据转换成数据库po
func E2PSysSkillUpdate(en *entity.SysSkill) *po.SysSkill {
	var po po.SysSkill
	if err := copier.Copy(&po, &en); err != nil {
		panic(any(err))
	}

	po.UpdatedAt = time.Now().UnixNano()
	return &po
}

// P2ESysSkill 数据库po转换成entity
func P2ESysSkill(p *po.SysSkill) *entity.SysSkill {
	var en entity.SysSkill
	if err := copier.Copy(&en, &p); err != nil {
		panic(any(err))
	}

	return &en
}

func P2ESysSkills(pos []*po.SysSkill) []*entity.SysSkill {
	ens := make([]*entity.SysSkill, 0)
	if len(pos) == 0 {
		return ens
	}

	for _, val := range pos {
		cfg := P2ESysSkill(val)
		ens = append(ens, cfg)
	}

	return ens
}
