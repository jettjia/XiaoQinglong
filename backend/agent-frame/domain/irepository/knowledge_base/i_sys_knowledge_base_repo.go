package knowledge_base

import (
	"context"

	"github.com/jettjia/igo-pkg/pkg/xsql/builder"

	entity "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/knowledge_base"
)

// ISysKnowledgeBaseRepo 仓库接口
type ISysKnowledgeBaseRepo interface {
	Create(ctx context.Context, sysKnowledgeBaseEn *entity.SysKnowledgeBase) (ulid string, err error)                                       // 创建
	Delete(ctx context.Context, sysKnowledgeBaseEn *entity.SysKnowledgeBase) (err error)                                                 // 删除
	Update(ctx context.Context, sysKnowledgeBaseEn *entity.SysKnowledgeBase) (err error)                                                  // 修改
	FindById(ctx context.Context, ulid string, selectColumn ...string) (sysKnowledgeBaseEn *entity.SysKnowledgeBase, err error)         // 查看byId
	FindAll(ctx context.Context, queries []*builder.Query, selectArgs ...[]string) (entries []*entity.SysKnowledgeBase, err error)       // 所有
	FindPage(ctx context.Context, queries []*builder.Query, reqPage *builder.PageData, reqSort *builder.SortData, selectArgs ...[]string) (entries []*entity.SysKnowledgeBase, rspPag *builder.PageData, err error) // 列表
}
