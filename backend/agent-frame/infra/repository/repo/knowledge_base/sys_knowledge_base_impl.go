package knowledge_base

import (
	"context"

	"github.com/jettjia/igo-pkg/pkg/data"
	"github.com/jettjia/igo-pkg/pkg/xsql/builder"

	entity "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/knowledge_base"
	irepository "github.com/jettjia/xiaoqinglong/agent-frame/domain/irepository/knowledge_base"
	"github.com/jettjia/xiaoqinglong/agent-frame/infra/pkg/idata"
	converter "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/converter/knowledge_base"
	po "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/knowledge_base"
)

var _ irepository.ISysKnowledgeBaseRepo = (*SysKnowledgeBase)(nil)

type SysKnowledgeBase struct {
	data *data.Data
}

func NewSysKnowledgeBaseImpl() *SysKnowledgeBase {
	return &SysKnowledgeBase{
		data: idata.NewDataOptionCli(),
	}
}

func (r *SysKnowledgeBase) Create(ctx context.Context, sysKnowledgeBaseEn *entity.SysKnowledgeBase) (ulid string, err error) {
	sysKnowledgeBasePo := converter.E2PSysKnowledgeBaseAdd(sysKnowledgeBaseEn)
	if err = r.data.DB(ctx).Create(&sysKnowledgeBasePo).Error; err != nil {
		return
	}

	return sysKnowledgeBasePo.Ulid, nil
}

func (r *SysKnowledgeBase) Delete(ctx context.Context, sysKnowledgeBaseEn *entity.SysKnowledgeBase) (err error) {
	sysKnowledgeBasePo := converter.E2PSysKnowledgeBaseDel(sysKnowledgeBaseEn)

	return r.data.DB(ctx).Model(&po.SysKnowledgeBase{}).Where("ulid = ? ", sysKnowledgeBaseEn.Ulid).Updates(sysKnowledgeBasePo).Error
}

func (r *SysKnowledgeBase) Update(ctx context.Context, sysKnowledgeBaseEn *entity.SysKnowledgeBase) (err error) {
	sysKnowledgeBasePo := converter.E2PSysKnowledgeBaseUpdate(sysKnowledgeBaseEn)

	return r.data.DB(ctx).Model(&po.SysKnowledgeBase{}).Where("ulid = ? ", sysKnowledgeBaseEn.Ulid).Updates(sysKnowledgeBasePo).Error
}

func (r *SysKnowledgeBase) FindById(ctx context.Context, ulid string, selectColumn ...string) (sysKnowledgeBaseEn *entity.SysKnowledgeBase, err error) {
	var sysKnowledgeBasePo po.SysKnowledgeBase
	if err = r.data.DB(ctx).Select(selectColumn).Limit(1).Find(&sysKnowledgeBasePo, "ulid = ? ", ulid).Error; err != nil {
		return
	}

	sysKnowledgeBaseEn = converter.P2ESysKnowledgeBase(&sysKnowledgeBasePo)

	return
}

func (r *SysKnowledgeBase) FindAll(ctx context.Context, queries []*builder.Query, selectArgs ...[]string) (entries []*entity.SysKnowledgeBase, err error) {
	whereStr, values, err := builder.GormBuildWhere(queries)
	if err != nil {
		return
	}

	selectField := builder.BuildSelectVariable(selectArgs...)

	sysKnowledgeBasePos := make([]*po.SysKnowledgeBase, 0)
	if err = r.data.DB(ctx).Model(&po.SysKnowledgeBase{}).Select(selectField).Where(whereStr, values...).Order("ulid desc").Find(&sysKnowledgeBasePos).Error; err != nil {
		return
	}

	entries = converter.P2ESysKnowledgeBases(sysKnowledgeBasePos)

	return
}

func (r *SysKnowledgeBase) FindPage(ctx context.Context, queries []*builder.Query, reqPage *builder.PageData, reqSort *builder.SortData, selectArgs ...[]string) (entries []*entity.SysKnowledgeBase, rspPag *builder.PageData, err error) {
	var (
		total int64
	)
	sysKnowledgeBasePos := make([]*po.SysKnowledgeBase, 0)

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

	dbQuery := r.data.DB(ctx).Model(&po.SysKnowledgeBase{}).Where(whereStr, values...)

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
		Find(&sysKnowledgeBasePos).
		Error

	if err != nil {
		return
	}

	entries = converter.P2ESysKnowledgeBases(sysKnowledgeBasePos)

	return
}
