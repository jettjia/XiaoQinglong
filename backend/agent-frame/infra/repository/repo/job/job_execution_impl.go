package job

import (
	"context"

	"github.com/jettjia/igo-pkg/pkg/data"
	"github.com/jettjia/igo-pkg/pkg/xsql/builder"

	entityJob "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/job"
	irepositoryJob "github.com/jettjia/xiaoqinglong/agent-frame/domain/irepository/job"
	"github.com/jettjia/xiaoqinglong/agent-frame/infra/pkg/idata"
	converterJob "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/converter/job"
	poJob "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/job"
)

var _ irepositoryJob.IJobExecutionRepo = (*JobExecutionRepo)(nil)

type JobExecutionRepo struct {
	data *data.Data
}

func NewJobExecutionImpl() *JobExecutionRepo {
	return &JobExecutionRepo{
		data: idata.NewDataOptionCli(),
	}
}

func (r *JobExecutionRepo) Create(ctx context.Context, jobEn *entityJob.JobExecution) (ulid string, err error) {
	jobPo := converterJob.E2PJobExecutionAdd(jobEn)
	if err = r.data.DB(ctx).Create(&jobPo).Error; err != nil {
		return
	}

	return jobPo.Ulid, nil
}

func (r *JobExecutionRepo) Delete(ctx context.Context, jobEn *entityJob.JobExecution) (err error) {
	jobPo := converterJob.E2PJobExecutionDel(jobEn)

	return r.data.DB(ctx).Model(&poJob.JobExecutionPO{}).Where("ulid = ? ", jobEn.Ulid).Updates(jobPo).Error
}

func (r *JobExecutionRepo) Update(ctx context.Context, jobEn *entityJob.JobExecution) (err error) {
	jobPo := converterJob.E2PJobExecutionUpdate(jobEn)

	return r.data.DB(ctx).Model(&poJob.JobExecutionPO{}).Where("ulid = ? ", jobEn.Ulid).Updates(jobPo).Error
}

func (r *JobExecutionRepo) FindById(ctx context.Context, ulid string, selectColumn ...string) (jobEn *entityJob.JobExecution, err error) {
	var jobPo poJob.JobExecutionPO
	if err = r.data.DB(ctx).Select(selectColumn).Limit(1).Find(&jobPo, "ulid = ? AND deleted_at = 0", ulid).Error; err != nil {
		return
	}

	jobEn = converterJob.P2EJobExecution(&jobPo)

	return
}

func (r *JobExecutionRepo) FindByQuery(ctx context.Context, queries []*builder.Query) (jobEn *entityJob.JobExecution, err error) {
	whereStr, values, err := builder.GormBuildWhere(queries)
	if err != nil {
		return
	}

	var jobPo poJob.JobExecutionPO
	if err = r.data.DB(ctx).Model(&poJob.JobExecutionPO{}).Limit(1).Where(whereStr+" AND deleted_at = 0", values...).Find(&jobPo).Error; err != nil {
		return
	}

	jobEn = converterJob.P2EJobExecution(&jobPo)

	return
}

func (r *JobExecutionRepo) FindAll(ctx context.Context, queries []*builder.Query, selectArgs ...[]string) (entries []*entityJob.JobExecution, err error) {
	whereStr, values, err := builder.GormBuildWhere(queries)
	if err != nil {
		return
	}

	selectField := builder.BuildSelectVariable(selectArgs...)

	jobPos := make([]*poJob.JobExecutionPO, 0)
	if err = r.data.DB(ctx).Model(&poJob.JobExecutionPO{}).Select(selectField).Where(whereStr+" AND deleted_at = 0", values...).Order("ulid desc").Find(&jobPos).Error; err != nil {
		return
	}

	entries = converterJob.P2EJobExecutions(jobPos)

	return
}

func (r *JobExecutionRepo) FindPage(ctx context.Context, queries []*builder.Query, reqPage *builder.PageData, reqSort *builder.SortData, selectArgs ...[]string) (entries []*entityJob.JobExecution, rspPag *builder.PageData, err error) {
	var (
		total int64
	)
	jobPos := make([]*poJob.JobExecutionPO, 0)

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

	dbQuery := r.data.DB(ctx).Model(&poJob.JobExecutionPO{}).Where(whereStr+" AND deleted_at = 0", values...)

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
		Find(&jobPos).
		Error

	if err != nil {
		return
	}

	entries = converterJob.P2EJobExecutions(jobPos)

	return
}

func (r *JobExecutionRepo) FindByAgentId(ctx context.Context, agentId string, limit int) ([]*entityJob.JobExecution, error) {
	jobPos := make([]*poJob.JobExecutionPO, 0)
	err := r.data.DB(ctx).Model(&poJob.JobExecutionPO{}).
		Where("agent_id = ? AND deleted_at = 0", agentId).
		Order("trigger_time DESC").
		Limit(limit).
		Find(&jobPos).Error

	if err != nil {
		return nil, err
	}

	return converterJob.P2EJobExecutions(jobPos), nil
}

func (r *JobExecutionRepo) DeleteOldByAgentId(ctx context.Context, agentId string, keepCount int) error {
	// 删除超过 keepCount 的最旧记录
	sql := `DELETE FROM job_execution_log WHERE agent_id = ? AND deleted_at = 0
			AND ulid NOT IN (
				SELECT ulid FROM (
					SELECT ulid FROM job_execution_log
					WHERE agent_id = ? AND deleted_at = 0
					ORDER BY trigger_time DESC LIMIT ?
				) t
			)`
	err := r.data.DB(ctx).Exec(sql, agentId, agentId, keepCount).Error
	return err
}

func (r *JobExecutionRepo) CountByAgentId(ctx context.Context, agentId string) (int, error) {
	var count int64
	err := r.data.DB(ctx).Model(&poJob.JobExecutionPO{}).
		Where("agent_id = ? AND deleted_at = 0", agentId).
		Count(&count).Error
	return int(count), err
}
