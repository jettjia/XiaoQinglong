package model

import (
	"context"

	"github.com/jettjia/igo-pkg/pkg/data"
	"github.com/jettjia/igo-pkg/pkg/xsql/builder"

	entityModel "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/model"
	irepositoryModel "github.com/jettjia/xiaoqinglong/agent-frame/domain/irepository/model"
	"github.com/jettjia/xiaoqinglong/agent-frame/infra/pkg/idata"
	converterModel "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/converter/model"
	poModel "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/model"
)

var _ irepositoryModel.ISysModelRepo = (*SysModel)(nil)

type SysModel struct {
	data *data.Data
}

func NewSysModelImpl() *SysModel {
	return &SysModel{
		data: idata.NewDataOptionCli(),
	}
}

func (r *SysModel) Create(ctx context.Context, sysModelEn *entityModel.SysModel) (ulid string, err error) {
	sysModelPo := converterModel.E2PSysModelAdd(sysModelEn)
	if err = r.data.DB(ctx).Create(&sysModelPo).Error; err != nil {
		return
	}

	return sysModelPo.Ulid, nil
}

func (r *SysModel) Delete(ctx context.Context, sysModelEn *entityModel.SysModel) (err error) {
	sysModelPo := converterModel.E2PSysModelDel(sysModelEn)

	return r.data.DB(ctx).Model(&poModel.SysModel{}).Where("ulid = ? ", sysModelEn.Ulid).Updates(sysModelPo).Error
}

func (r *SysModel) Update(ctx context.Context, sysModelEn *entityModel.SysModel) (err error) {
	sysModelPo := converterModel.E2PSysModelUpdate(sysModelEn)

	return r.data.DB(ctx).Model(&poModel.SysModel{}).Where("ulid = ? ", sysModelEn.Ulid).Updates(sysModelPo).Error
}

func (r *SysModel) FindById(ctx context.Context, ulid string, selectColumn ...string) (sysModelEn *entityModel.SysModel, err error) {
	var sysModelPo poModel.SysModel
	if err = r.data.DB(ctx).Select(selectColumn).Limit(1).Find(&sysModelPo, "ulid = ? ", ulid).Error; err != nil {
		return
	}

	sysModelEn = converterModel.P2ESysModel(&sysModelPo)

	return
}

func (r *SysModel) FindAll(ctx context.Context, queries []*builder.Query, selectArgs ...[]string) (entries []*entityModel.SysModel, err error) {
	whereStr, values, err := builder.GormBuildWhere(queries)
	if err != nil {
		return
	}

	selectField := builder.BuildSelectVariable(selectArgs...)

	sysModelPos := make([]*poModel.SysModel, 0)
	if err = r.data.DB(ctx).Model(&poModel.SysModel{}).Select(selectField).Where(whereStr, values...).Order("ulid desc").Find(&sysModelPos).Error; err != nil {
		return
	}

	entries = converterModel.P2ESysModels(sysModelPos)

	return
}

func (r *SysModel) FindPage(ctx context.Context, queries []*builder.Query, reqPage *builder.PageData, reqSort *builder.SortData, selectArgs ...[]string) (entries []*entityModel.SysModel, rspPag *builder.PageData, err error) {
	var (
		total int64
	)
	sysModelPos := make([]*poModel.SysModel, 0)

	whereStr, values, err := builder.GormBuildWhere(queries)
	if err != nil {
		return
	}

	// default reqSort
	if reqSort == nil {
		reqSort = &builder.SortData{Sort: "ulid", Direction: "desc"}
	}
	// default reqPage
	if reqPage == nil {
		reqPage = &builder.PageData{PageNum: 1, PageSize: 10}
	}
	// select
	selectField := builder.BuildSelectVariable(selectArgs...)

	dbQuery := r.data.DB(ctx).Model(&poModel.SysModel{}).Where(whereStr, values...)

	if err = dbQuery.Count(&total).Error; err != nil {
		return
	}

	rspPag = &builder.PageData{
		PageNum:     reqPage.PageNum,
		PageSize:    reqPage.PageSize,
		TotalNumber: total,
		TotalPage:   builder.CeilPageNum(total, reqPage.PageSize),
	}

	if total == 0 {
		return
	}

	err = dbQuery.
		Select(selectField).
		Order(reqSort.Sort + " " + reqSort.Direction).
		Scopes(builder.GormPaginate(reqPage.PageNum, reqPage.PageSize)).
		Find(&sysModelPos).
		Error

	if err != nil {
		return
	}

	entries = converterModel.P2ESysModels(sysModelPos)

	return
}