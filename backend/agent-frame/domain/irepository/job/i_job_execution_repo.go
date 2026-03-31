package job

import (
	"context"

	"github.com/jettjia/igo-pkg/pkg/xsql/builder"

	entityJob "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/job"
)

// IJobExecutionRepo job_execution_log
//
//go:generate mockgen --source ./i_job_execution_repo.go --destination ./mock/mock_i_job_execution_repo.go --package mock
type IJobExecutionRepo interface {
	Create(ctx context.Context, jobEn *entityJob.JobExecution) (ulid string, err error)                                                                                              // 创建
	Delete(ctx context.Context, jobEn *entityJob.JobExecution) (err error)                                                                                                           // 删除
	Update(ctx context.Context, jobEn *entityJob.JobExecution) (err error)                                                                                                           // 修改
	FindById(ctx context.Context, ulid string, selectColumn ...string) (jobEn *entityJob.JobExecution, err error)                                                                     // 查看byId
	FindByQuery(ctx context.Context, queries []*builder.Query) (jobEn *entityJob.JobExecution, err error)                                                                             // 查看byQuery
	FindAll(ctx context.Context, queries []*builder.Query, selectArgs ...[]string) (entries []*entityJob.JobExecution, err error)                                                     // 所有
	FindPage(ctx context.Context, queries []*builder.Query, reqPage *builder.PageData, reqSort *builder.SortData, selectArgs ...[]string) ([]*entityJob.JobExecution, *builder.PageData, error) // 列表
	FindByAgentId(ctx context.Context, agentId string, limit int) ([]*entityJob.JobExecution, error)                                                                                  // 根据AgentId查询
	DeleteOldByAgentId(ctx context.Context, agentId string, keepCount int) error                                                                                                    // 删除旧的
	CountByAgentId(ctx context.Context, agentId string) (int, error)                                                                                                                // 统计数量
}
