package channel

import (
	"context"

	"github.com/jettjia/igo-pkg/pkg/data"
	"github.com/jettjia/igo-pkg/pkg/xsql/builder"
	"gorm.io/gorm"

	entity "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/channel"
	irepository "github.com/jettjia/xiaoqinglong/agent-frame/domain/irepository/channel"
	"github.com/jettjia/xiaoqinglong/agent-frame/infra/pkg/idata"
	converter "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/converter/channel"
	po "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/channel"
)

var _ irepository.ISysChannelRepo = (*SysChannel)(nil)

type SysChannel struct {
	data *data.Data
}

func NewSysChannelImpl() *SysChannel {
	return &SysChannel{
		data: idata.NewDataOptionCli(),
	}
}

func (r *SysChannel) Create(ctx context.Context, sysChannelEn *entity.SysChannel) (ulid string, err error) {
	sysChannelPo := converter.E2PSysChannelAdd(sysChannelEn)
	if err = r.data.DB(ctx).Create(&sysChannelPo).Error; err != nil {
		return
	}

	return sysChannelPo.Ulid, nil
}

func (r *SysChannel) Delete(ctx context.Context, sysChannelEn *entity.SysChannel) (err error) {
	sysChannelPo := converter.E2PSysChannelDel(sysChannelEn)

	return r.data.DB(ctx).Model(&po.SysChannel{}).Where("ulid = ? ", sysChannelEn.Ulid).Updates(sysChannelPo).Error
}

func (r *SysChannel) Update(ctx context.Context, sysChannelEn *entity.SysChannel) (err error) {
	sysChannelPo := converter.E2PSysChannelUpdate(sysChannelEn)

	return r.data.DB(ctx).Model(&po.SysChannel{}).Where("ulid = ? ", sysChannelEn.Ulid).Updates(sysChannelPo).Error
}

func (r *SysChannel) FindById(ctx context.Context, ulid string, selectColumn ...string) (sysChannelEn *entity.SysChannel, err error) {
	var sysChannelPo po.SysChannel
	if err = r.data.DB(ctx).Select(selectColumn).Limit(1).Find(&sysChannelPo, "ulid = ? ", ulid).Error; err != nil {
		return
	}

	sysChannelEn = converter.P2ESysChannel(&sysChannelPo)

	return
}

func (r *SysChannel) FindAll(ctx context.Context, queries []*builder.Query, selectArgs ...[]string) (entries []*entity.SysChannel, err error) {
	whereStr, values, err := builder.GormBuildWhere(queries)
	if err != nil {
		return
	}

	selectField := builder.BuildSelectVariable(selectArgs...)

	sysChannelPos := make([]*po.SysChannel, 0)
	if err = r.data.DB(ctx).Model(&po.SysChannel{}).Select(selectField).Where(whereStr, values...).Order("sort asc").Find(&sysChannelPos).Error; err != nil {
		return
	}

	entries = converter.P2ESysChannels(sysChannelPos)

	return
}

func (r *SysChannel) FindPage(ctx context.Context, queries []*builder.Query, reqPage *builder.PageData, reqSort *builder.SortData, selectArgs ...[]string) (entries []*entity.SysChannel, rspPag *builder.PageData, err error) {
	var (
		total int64
	)
	sysChannelPos := make([]*po.SysChannel, 0)

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

	dbQuery := r.data.DB(ctx).Model(&po.SysChannel{}).Where(whereStr, values...)

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
		Find(&sysChannelPos).
		Error

	if err != nil {
		return
	}

	entries = converter.P2ESysChannels(sysChannelPos)

	return
}

func (r *SysChannel) FindByCode(ctx context.Context, code string) (sysChannelEn *entity.SysChannel, err error) {
	var sysChannelPo po.SysChannel
	if err = r.data.DB(ctx).Limit(1).Find(&sysChannelPo, "code = ? AND deleted_at = 0", code).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return
	}

	sysChannelEn = converter.P2ESysChannel(&sysChannelPo)
	// 如果返回的是空记录（表为空或未找到），返回 nil
	if sysChannelEn.Code == "" {
		return nil, nil
	}

	return
}
