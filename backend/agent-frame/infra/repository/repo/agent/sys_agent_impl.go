package agent

import (
	"context"

	"github.com/jettjia/igo-pkg/pkg/data"
	"github.com/jettjia/igo-pkg/pkg/xsql/builder"
	"gorm.io/gorm"

	entity "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/agent"
	irepository "github.com/jettjia/xiaoqinglong/agent-frame/domain/irepository/agent"
	"github.com/jettjia/xiaoqinglong/agent-frame/infra/pkg/idata"
	converter "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/converter/agent"
	po "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/agent"
)

var _ irepository.ISysAgentRepo = (*SysAgent)(nil)

type SysAgent struct {
	data *data.Data
}

func NewSysAgentImpl() *SysAgent {
	return &SysAgent{
		data: idata.NewDataOptionCli(),
	}
}

func (r *SysAgent) Create(ctx context.Context, sysAgentEn *entity.SysAgent) (ulid string, err error) {
	sysAgentPo := converter.E2PSysAgentAdd(sysAgentEn)
	if err = r.data.DB(ctx).Create(&sysAgentPo).Error; err != nil {
		return
	}

	return sysAgentPo.Ulid, nil
}

func (r *SysAgent) Delete(ctx context.Context, sysAgentEn *entity.SysAgent) (err error) {
	sysAgentPo := converter.E2PSysAgentDel(sysAgentEn)

	return r.data.DB(ctx).Model(&po.SysAgent{}).Where("ulid = ? ", sysAgentEn.Ulid).Updates(sysAgentPo).Error
}

func (r *SysAgent) Update(ctx context.Context, sysAgentEn *entity.SysAgent) (err error) {
	sysAgentPo := converter.E2PSysAgentUpdate(sysAgentEn)

	return r.data.DB(ctx).Model(&po.SysAgent{}).Where("ulid = ? ", sysAgentEn.Ulid).Updates(sysAgentPo).Error
}

func (r *SysAgent) FindById(ctx context.Context, ulid string, selectColumn ...string) (sysAgentEn *entity.SysAgent, err error) {
	var sysAgentPo po.SysAgent
	if err = r.data.DB(ctx).Select(selectColumn).Limit(1).Find(&sysAgentPo, "ulid = ? ", ulid).Error; err != nil {
		return
	}

	sysAgentEn = converter.P2ESysAgent(&sysAgentPo)

	return
}

func (r *SysAgent) FindAll(ctx context.Context, queries []*builder.Query, selectArgs ...[]string) (entries []*entity.SysAgent, err error) {
	whereStr, values, err := builder.GormBuildWhere(queries)
	if err != nil {
		return
	}

	selectField := builder.BuildSelectVariable(selectArgs...)

	sysAgentPos := make([]*po.SysAgent, 0)
	if err = r.data.DB(ctx).Model(&po.SysAgent{}).Select(selectField).Where(whereStr, values...).Order("ulid desc").Find(&sysAgentPos).Error; err != nil {
		return
	}

	entries = converter.P2ESysAgents(sysAgentPos)

	return
}

func (r *SysAgent) FindPage(ctx context.Context, queries []*builder.Query, reqPage *builder.PageData, reqSort *builder.SortData, selectArgs ...[]string) (entries []*entity.SysAgent, rspPag *builder.PageData, err error) {
	var (
		total int64
	)
	sysAgentPos := make([]*po.SysAgent, 0)

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

	dbQuery := r.data.DB(ctx).Model(&po.SysAgent{}).Where(whereStr, values...)

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
		Find(&sysAgentPos).
		Error

	if err != nil {
		return
	}

	entries = converter.P2ESysAgents(sysAgentPos)

	return
}

func (r *SysAgent) FindByName(ctx context.Context, name string) (sysAgentEn *entity.SysAgent, err error) {
	var sysAgentPo po.SysAgent
	if err = r.data.DB(ctx).Limit(1).Find(&sysAgentPo, "name = ? ", name).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return
	}

	sysAgentEn = converter.P2ESysAgent(&sysAgentPo)

	return
}
