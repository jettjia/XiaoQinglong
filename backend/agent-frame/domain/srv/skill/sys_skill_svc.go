package skill

import (
	"context"

	"github.com/jettjia/igo-pkg/pkg/xsql/builder"

	entity "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/skill"
	repo "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/repo/skill"
)

// SysSkillSvc domain service
type SysSkillSvc struct {
	sysSkillRepo *repo.SysSkill
}

// NewSysSkillSvc NewSysSkillSvc
func NewSysSkillSvc() *SysSkillSvc {
	return &SysSkillSvc{
		sysSkillRepo: repo.NewSysSkillImpl(),
	}
}

// Create 创建
func (s *SysSkillSvc) Create(ctx context.Context, sysSkillEn *entity.SysSkill) (ulid string, err error) {
	return s.sysSkillRepo.Create(ctx, sysSkillEn)
}

// Delete 删除
func (s *SysSkillSvc) Delete(ctx context.Context, sysSkillEn *entity.SysSkill) (err error) {
	return s.sysSkillRepo.Delete(ctx, sysSkillEn)
}

// Update 修改
func (s *SysSkillSvc) Update(ctx context.Context, sysSkillEn *entity.SysSkill) (err error) {
	return s.sysSkillRepo.Update(ctx, sysSkillEn)
}

// FindById 查看byId
func (s *SysSkillSvc) FindById(ctx context.Context, ulid string) (sysSkillEn *entity.SysSkill, err error) {
	return s.sysSkillRepo.FindById(ctx, ulid)
}

// FindAll 所有
func (s *SysSkillSvc) FindAll(ctx context.Context, queries []*builder.Query) (entries []*entity.SysSkill, err error) {
	return s.sysSkillRepo.FindAll(ctx, queries)
}

// FindPage 列表
func (s *SysSkillSvc) FindPage(ctx context.Context, queries []*builder.Query, reqPage *builder.PageData, reqSort *builder.SortData) (entries []*entity.SysSkill, pageData *builder.PageData, err error) {
	return s.sysSkillRepo.FindPage(ctx, queries, reqPage, reqSort)
}

// FindByName 按名称查找
func (s *SysSkillSvc) FindByName(ctx context.Context, name string) (sysSkillEn *entity.SysSkill, err error) {
	return s.sysSkillRepo.FindByName(ctx, name)
}

// FindByNameAndType 按名称和类型查找
func (s *SysSkillSvc) FindByNameAndType(ctx context.Context, name string, skillType string) (sysSkillEn *entity.SysSkill, err error) {
	return s.sysSkillRepo.FindByNameAndType(ctx, name, skillType)
}
