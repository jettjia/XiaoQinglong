package agent

import (
	"context"

	"github.com/jettjia/igo-pkg/pkg/xsql/builder"

	entity "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/agent"
	repo "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/repo/agent"
)

// SysAgentSvc domain service
type SysAgentSvc struct {
	sysAgentRepo *repo.SysAgent
}

// NewSysAgentSvc NewSysAgentSvc
func NewSysAgentSvc() *SysAgentSvc {
	return &SysAgentSvc{
		sysAgentRepo: repo.NewSysAgentImpl(),
	}
}

// Create 创建
func (s *SysAgentSvc) Create(ctx context.Context, sysAgentEn *entity.SysAgent) (ulid string, err error) {
	return s.sysAgentRepo.Create(ctx, sysAgentEn)
}

// Delete 删除
func (s *SysAgentSvc) Delete(ctx context.Context, sysAgentEn *entity.SysAgent) (err error) {
	return s.sysAgentRepo.Delete(ctx, sysAgentEn)
}

// Update 修改
func (s *SysAgentSvc) Update(ctx context.Context, sysAgentEn *entity.SysAgent) (err error) {
	return s.sysAgentRepo.Update(ctx, sysAgentEn)
}

// UpdateEnabled 修改启用状态
func (s *SysAgentSvc) UpdateEnabled(ctx context.Context, ulid string, enabled bool) error {
	return s.sysAgentRepo.UpdateEnabled(ctx, ulid, enabled)
}

// FindById 查看byId
func (s *SysAgentSvc) FindById(ctx context.Context, ulid string) (sysAgentEn *entity.SysAgent, err error) {
	return s.sysAgentRepo.FindById(ctx, ulid)
}

// FindAll 所有
func (s *SysAgentSvc) FindAll(ctx context.Context, queries []*builder.Query) (entries []*entity.SysAgent, err error) {
	return s.sysAgentRepo.FindAll(ctx, queries)
}

// FindPage 列表
func (s *SysAgentSvc) FindPage(ctx context.Context, queries []*builder.Query, reqPage *builder.PageData, reqSort *builder.SortData) (entries []*entity.SysAgent, pageData *builder.PageData, err error) {
	return s.sysAgentRepo.FindPage(ctx, queries, reqPage, reqSort)
}

// FindByName 按名称查找
func (s *SysAgentSvc) FindByName(ctx context.Context, name string) (sysAgentEn *entity.SysAgent, err error) {
	return s.sysAgentRepo.FindByName(ctx, name)
}
