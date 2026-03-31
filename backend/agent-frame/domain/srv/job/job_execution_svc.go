package job

import (
	"context"

	"github.com/jettjia/igo-pkg/pkg/xsql/builder"

	entityJob "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/job"
	repoJob "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/repo/job"
)

// JobExecutionSvc job execution service
//
//go:generate mockgen --source ./job_execution_svc.go --destination ./mock/mock_job_execution_svc.go --package mock
type JobExecutionSvc interface {
	CreateJobExecution(ctx context.Context, jobEn *entityJob.JobExecution) (ulid string, err error)                                                                                  // 创建
	DeleteJobExecution(ctx context.Context, jobEn *entityJob.JobExecution) (err error)                                                                                           // 删除
	UpdateJobExecution(ctx context.Context, jobEn *entityJob.JobExecution) (err error)                                                                                           // 修改
	FindJobExecutionById(ctx context.Context, ulid string) (jobEn *entityJob.JobExecution, err error)                                                                             // 查看byId
	FindJobExecutionByQuery(ctx context.Context, queries []*builder.Query) (jobEn *entityJob.JobExecution, err error)                                                             // 查看byQuery
	FindJobExecutionAll(ctx context.Context, queries []*builder.Query) (entries []*entityJob.JobExecution, err error)                                                             // 所有
	FindJobExecutionPage(ctx context.Context, queries []*builder.Query, reqPage *builder.PageData, reqSort *builder.SortData) ([]*entityJob.JobExecution, *builder.PageData, error) // 列表
	FindByAgentId(ctx context.Context, agentId string, limit int) ([]*entityJob.JobExecution, error)                                                                              // 根据AgentId查询
	DeleteOldByAgentId(ctx context.Context, agentId string, keepCount int) error                                                                                                // 删除旧的
	CountByAgentId(ctx context.Context, agentId string) (int, error)                                                                                                              // 统计数量
}

type JobExecution struct {
	jobExecutionRepo *repoJob.JobExecutionRepo
}

func NewJobExecutionSvc() *JobExecution {
	return &JobExecution{
		jobExecutionRepo: repoJob.NewJobExecutionImpl(),
	}
}

func (s *JobExecution) CreateJobExecution(ctx context.Context, jobEn *entityJob.JobExecution) (ulid string, err error) {
	return s.jobExecutionRepo.Create(ctx, jobEn)
}

func (s *JobExecution) DeleteJobExecution(ctx context.Context, jobEn *entityJob.JobExecution) (err error) {
	return s.jobExecutionRepo.Delete(ctx, jobEn)
}

func (s *JobExecution) UpdateJobExecution(ctx context.Context, jobEn *entityJob.JobExecution) (err error) {
	return s.jobExecutionRepo.Update(ctx, jobEn)
}

func (s *JobExecution) FindJobExecutionById(ctx context.Context, ulid string) (jobEn *entityJob.JobExecution, err error) {
	return s.jobExecutionRepo.FindById(ctx, ulid)
}

func (s *JobExecution) FindJobExecutionByQuery(ctx context.Context, queries []*builder.Query) (jobEn *entityJob.JobExecution, err error) {
	return s.jobExecutionRepo.FindByQuery(ctx, queries)
}

func (s *JobExecution) FindJobExecutionAll(ctx context.Context, queries []*builder.Query) (entries []*entityJob.JobExecution, err error) {
	return s.jobExecutionRepo.FindAll(ctx, queries)
}

func (s *JobExecution) FindJobExecutionPage(ctx context.Context, queries []*builder.Query, reqPage *builder.PageData, reqSort *builder.SortData) ([]*entityJob.JobExecution, *builder.PageData, error) {
	return s.jobExecutionRepo.FindPage(ctx, queries, reqPage, reqSort)
}

func (s *JobExecution) FindByAgentId(ctx context.Context, agentId string, limit int) ([]*entityJob.JobExecution, error) {
	return s.jobExecutionRepo.FindByAgentId(ctx, agentId, limit)
}

func (s *JobExecution) DeleteOldByAgentId(ctx context.Context, agentId string, keepCount int) error {
	return s.jobExecutionRepo.DeleteOldByAgentId(ctx, agentId, keepCount)
}

func (s *JobExecution) CountByAgentId(ctx context.Context, agentId string) (int, error) {
	return s.jobExecutionRepo.CountByAgentId(ctx, agentId)
}
