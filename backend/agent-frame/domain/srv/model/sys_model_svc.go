package model

import (
	"context"

	"github.com/jettjia/igo-pkg/pkg/xsql/builder"

	entityModel "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/model"
	repoModel "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/repo/model"
)

// SysModelSvc domain service
type SysModelSvc struct {
	sysModelRepo *repoModel.SysModel
}

// NewSysModelSvc NewSysModelSvc
func NewSysModelSvc() *SysModelSvc {
	return &SysModelSvc{
		sysModelRepo: repoModel.NewSysModelImpl(),
	}
}

// CreateSysModel 创建
func (s *SysModelSvc) CreateSysModel(ctx context.Context, sysModelEn *entityModel.SysModel) (ulid string, err error) {
	return s.sysModelRepo.Create(ctx, sysModelEn)
}

// DeleteSysModel 删除
func (s *SysModelSvc) DeleteSysModel(ctx context.Context, sysModelEn *entityModel.SysModel) (err error) {
	return s.sysModelRepo.Delete(ctx, sysModelEn)
}

// UpdateSysModel 修改
func (s *SysModelSvc) UpdateSysModel(ctx context.Context, sysModelEn *entityModel.SysModel) (err error) {
	return s.sysModelRepo.Update(ctx, sysModelEn)
}

// FindSysModelById 查看byId
func (s *SysModelSvc) FindSysModelById(ctx context.Context, ulid string) (sysModelEn *entityModel.SysModel, err error) {
	return s.sysModelRepo.FindById(ctx, ulid)
}

// FindSysModelAll 所有
func (s *SysModelSvc) FindSysModelAll(ctx context.Context, queries []*builder.Query) (entries []*entityModel.SysModel, err error) {
	return s.sysModelRepo.FindAll(ctx, queries)
}

// FindSysModelPage 列表
func (s *SysModelSvc) FindSysModelPage(ctx context.Context, queries []*builder.Query, reqPage *builder.PageData, reqSort *builder.SortData) (entries []*entityModel.SysModel, pageData *builder.PageData, err error) {
	return s.sysModelRepo.FindPage(ctx, queries, reqPage, reqSort)
}