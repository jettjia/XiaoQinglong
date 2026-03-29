package agent

import (
	"context"

	"github.com/jettjia/igo-pkg/pkg/xsql/builder"

	entity "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/agent"
)

// ISysAgentRepo 仓库接口
type ISysAgentRepo interface {
	Create(ctx context.Context, sysAgentEn *entity.SysAgent) (ulid string, err error)
	Delete(ctx context.Context, sysAgentEn *entity.SysAgent) (err error)
	Update(ctx context.Context, sysAgentEn *entity.SysAgent) (err error)
	FindById(ctx context.Context, ulid string, selectColumn ...string) (sysAgentEn *entity.SysAgent, err error)
	FindAll(ctx context.Context, queries []*builder.Query, selectArgs ...[]string) (entries []*entity.SysAgent, err error)
	FindPage(ctx context.Context, queries []*builder.Query, reqPage *builder.PageData, reqSort *builder.SortData, selectArgs ...[]string) (entries []*entity.SysAgent, rspPag *builder.PageData, err error)
	FindByName(ctx context.Context, name string) (sysAgentEn *entity.SysAgent, err error)
}
