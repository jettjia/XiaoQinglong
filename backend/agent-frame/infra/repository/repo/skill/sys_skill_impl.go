package skill

import (
	"context"

	"github.com/jettjia/igo-pkg/pkg/data"
	"github.com/jettjia/igo-pkg/pkg/xsql/builder"

	entity "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/skill"
	irepository "github.com/jettjia/xiaoqinglong/agent-frame/domain/irepository/skill"
	"github.com/jettjia/xiaoqinglong/agent-frame/infra/pkg/idata"
	converter "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/converter/skill"
	po "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/skill"
)

var _ irepository.ISysSkillRepo = (*SysSkill)(nil)

type SysSkill struct {
	data *data.Data
}

func NewSysSkillImpl() *SysSkill {
	return &SysSkill{
		data: idata.NewDataOptionCli(),
	}
}

func (r *SysSkill) Create(ctx context.Context, sysSkillEn *entity.SysSkill) (ulid string, err error) {
	sysSkillPo := converter.E2PSysSkillAdd(sysSkillEn)
	if err = r.data.DB(ctx).Create(&sysSkillPo).Error; err != nil {
		return
	}

	return sysSkillPo.Ulid, nil
}

func (r *SysSkill) Delete(ctx context.Context, sysSkillEn *entity.SysSkill) (err error) {
	sysSkillPo := converter.E2PSysSkillDel(sysSkillEn)

	return r.data.DB(ctx).Model(&po.SysSkill{}).Where("ulid = ? ", sysSkillEn.Ulid).Updates(sysSkillPo).Error
}

func (r *SysSkill) Update(ctx context.Context, sysSkillEn *entity.SysSkill) (err error) {
	sysSkillPo := converter.E2PSysSkillUpdate(sysSkillEn)

	return r.data.DB(ctx).Model(&po.SysSkill{}).Where("ulid = ? ", sysSkillEn.Ulid).Updates(sysSkillPo).Error
}

func (r *SysSkill) FindById(ctx context.Context, ulid string, selectColumn ...string) (sysSkillEn *entity.SysSkill, err error) {
	var sysSkillPo po.SysSkill
	if err = r.data.DB(ctx).Select(selectColumn).Limit(1).Find(&sysSkillPo, "ulid = ? ", ulid).Error; err != nil {
		return
	}

	sysSkillEn = converter.P2ESysSkill(&sysSkillPo)

	return
}

func (r *SysSkill) FindAll(ctx context.Context, queries []*builder.Query, selectArgs ...[]string) (entries []*entity.SysSkill, err error) {
	whereStr, values, err := builder.GormBuildWhere(queries)
	if err != nil {
		return
	}

	selectField := builder.BuildSelectVariable(selectArgs...)

	sysSkillPos := make([]*po.SysSkill, 0)
	if err = r.data.DB(ctx).Model(&po.SysSkill{}).Select(selectField).Where(whereStr, values...).Order("ulid desc").Find(&sysSkillPos).Error; err != nil {
		return
	}

	entries = converter.P2ESysSkills(sysSkillPos)

	return
}

func (r *SysSkill) FindPage(ctx context.Context, queries []*builder.Query, reqPage *builder.PageData, reqSort *builder.SortData, selectArgs ...[]string) (entries []*entity.SysSkill, rspPag *builder.PageData, err error) {
	var (
		total int64
	)
	sysSkillPos := make([]*po.SysSkill, 0)

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

	dbQuery := r.data.DB(ctx).Model(&po.SysSkill{}).Where(whereStr, values...)

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
		Find(&sysSkillPos).
		Error

	if err != nil {
		return
	}

	entries = converter.P2ESysSkills(sysSkillPos)

	return
}

func (r *SysSkill) FindByName(ctx context.Context, name string) (sysSkillEn *entity.SysSkill, err error) {
	var sysSkillPo po.SysSkill
	if err = r.data.DB(ctx).Limit(1).Find(&sysSkillPo, "name = ? ", name).Error; err != nil {
		return
	}

	sysSkillEn = converter.P2ESysSkill(&sysSkillPo)

	return
}
