package skill

import (
	"context"

	"github.com/jettjia/igo-pkg/pkg/xsql/builder"

	entity "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/skill"
)

// ISysSkillRepo 仓库接口
type ISysSkillRepo interface {
	Create(ctx context.Context, sysSkillEn *entity.SysSkill) (ulid string, err error)                                       // 创建
	Delete(ctx context.Context, sysSkillEn *entity.SysSkill) (err error)                                                 // 删除
	Update(ctx context.Context, sysSkillEn *entity.SysSkill) (err error)                                                  // 修改
	FindById(ctx context.Context, ulid string, selectColumn ...string) (sysSkillEn *entity.SysSkill, err error)         // 查看byId
	FindAll(ctx context.Context, queries []*builder.Query, selectArgs ...[]string) (entries []*entity.SysSkill, err error)       // 所有
	FindPage(ctx context.Context, queries []*builder.Query, reqPage *builder.PageData, reqSort *builder.SortData, selectArgs ...[]string) (entries []*entity.SysSkill, rspPag *builder.PageData, err error) // 列表
	FindByName(ctx context.Context, name string) (sysSkillEn *entity.SysSkill, err error) // 按名称查找
}
