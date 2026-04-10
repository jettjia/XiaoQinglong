package knowledge_base

import (
	"context"

	"github.com/jettjia/igo-pkg/pkg/xsql/builder"

	entity "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/knowledge_base"
	repo "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/repo/knowledge_base"
)

// SysKnowledgeBaseSvc domain service
type SysKnowledgeBaseSvc struct {
	sysKnowledgeBaseRepo *repo.SysKnowledgeBase
}

// NewSysKnowledgeBaseSvc NewSysKnowledgeBaseSvc
func NewSysKnowledgeBaseSvc() *SysKnowledgeBaseSvc {
	return &SysKnowledgeBaseSvc{
		sysKnowledgeBaseRepo: repo.NewSysKnowledgeBaseImpl(),
	}
}

// Create 创建
func (s *SysKnowledgeBaseSvc) Create(ctx context.Context, sysKnowledgeBaseEn *entity.SysKnowledgeBase) (ulid string, err error) {
	return s.sysKnowledgeBaseRepo.Create(ctx, sysKnowledgeBaseEn)
}

// Delete 删除
func (s *SysKnowledgeBaseSvc) Delete(ctx context.Context, sysKnowledgeBaseEn *entity.SysKnowledgeBase) (err error) {
	return s.sysKnowledgeBaseRepo.Delete(ctx, sysKnowledgeBaseEn)
}

// Update 修改
func (s *SysKnowledgeBaseSvc) Update(ctx context.Context, sysKnowledgeBaseEn *entity.SysKnowledgeBase) (err error) {
	return s.sysKnowledgeBaseRepo.Update(ctx, sysKnowledgeBaseEn)
}

// FindById 查看byId
func (s *SysKnowledgeBaseSvc) FindById(ctx context.Context, ulid string) (sysKnowledgeBaseEn *entity.SysKnowledgeBase, err error) {
	return s.sysKnowledgeBaseRepo.FindById(ctx, ulid)
}

// FindAll 所有
func (s *SysKnowledgeBaseSvc) FindAll(ctx context.Context, queries []*builder.Query) (entries []*entity.SysKnowledgeBase, err error) {
	return s.sysKnowledgeBaseRepo.FindAll(ctx, queries)
}

// FindPage 列表
func (s *SysKnowledgeBaseSvc) FindPage(ctx context.Context, queries []*builder.Query, reqPage *builder.PageData, reqSort *builder.SortData) (entries []*entity.SysKnowledgeBase, pageData *builder.PageData, err error) {
	return s.sysKnowledgeBaseRepo.FindPage(ctx, queries, reqPage, reqSort)
}
