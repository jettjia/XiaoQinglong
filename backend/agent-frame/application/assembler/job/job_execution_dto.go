package job

import (
	"github.com/jinzhu/copier"

	dtoJob "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/job"
	entityJob "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/job"
)

// JobExecutionDto dto转换器
type JobExecutionDto struct {
}

// NewJobExecutionDto NewJobExecutionDto
func NewJobExecutionDto() *JobExecutionDto {
	return &JobExecutionDto{}
}

//////////////////////////////////////////////////////////////////
// entity to dto

// E2DFindJobExecutionRsp entity转换成dto
func (a *JobExecutionDto) E2DFindJobExecutionRsp(en *entityJob.JobExecution) *dtoJob.FindJobExecutionRsp {
	var rspDto dtoJob.FindJobExecutionRsp

	if err := copier.Copy(&rspDto, &en); err != nil {
		panic(any(err))
	}

	return &rspDto
}

// E2DGetJobExecutions entity列表转换成dto列表
func (a *JobExecutionDto) E2DGetJobExecutions(ens []*entityJob.JobExecution) []*dtoJob.FindJobExecutionRsp {
	if len(ens) == 0 {
		return []*dtoJob.FindJobExecutionRsp{}
	}

	var jobExecutionsRsp []*dtoJob.FindJobExecutionRsp
	for _, v := range ens {
		cfg := a.E2DFindJobExecutionRsp(v)
		jobExecutionsRsp = append(jobExecutionsRsp, cfg)
	}

	return jobExecutionsRsp
}
