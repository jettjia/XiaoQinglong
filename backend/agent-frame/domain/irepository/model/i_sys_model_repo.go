package model

import (
	"context"

	"github.com/jettjia/igo-pkg/pkg/xsql/builder"

	entityModel "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/model"
)

// ISysModelRepo sys_model
type ISysModelRepo interface {
	Create(ctx context.Context, sysModelEn *entityModel.SysModel) (ulid string, err error)
	Delete(ctx context.Context, sysModelEn *entityModel.SysModel) (err error)
	Update(ctx context.Context, sysModelEn *entityModel.SysModel) (err error)
	FindById(ctx context.Context, ulid string, selectColumn ...string) (sysModelEn *entityModel.SysModel, err error)
	FindAll(ctx context.Context, queries []*builder.Query, selectArgs ...[]string) (entries []*entityModel.SysModel, err error)
	FindPage(ctx context.Context, queries []*builder.Query, reqPage *builder.PageData, reqSort *builder.SortData, selectArgs ...[]string) (entries []*entityModel.SysModel, rspPag *builder.PageData, err error)
}