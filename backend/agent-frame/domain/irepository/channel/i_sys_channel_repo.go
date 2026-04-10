package channel

import (
	"context"

	"github.com/jettjia/igo-pkg/pkg/xsql/builder"

	entity "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/channel"
)

// ISysChannelRepo 仓库接口
type ISysChannelRepo interface {
	Create(ctx context.Context, sysChannelEn *entity.SysChannel) (ulid string, err error)
	Delete(ctx context.Context, sysChannelEn *entity.SysChannel) (err error)
	Update(ctx context.Context, sysChannelEn *entity.SysChannel) (err error)
	FindById(ctx context.Context, ulid string, selectColumn ...string) (sysChannelEn *entity.SysChannel, err error)
	FindAll(ctx context.Context, queries []*builder.Query, selectArgs ...[]string) (entries []*entity.SysChannel, err error)
	FindPage(ctx context.Context, queries []*builder.Query, reqPage *builder.PageData, reqSort *builder.SortData, selectArgs ...[]string) (entries []*entity.SysChannel, rspPag *builder.PageData, err error)
	FindByCode(ctx context.Context, code string) (sysChannelEn *entity.SysChannel, err error)
}