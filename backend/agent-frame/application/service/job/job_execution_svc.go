package job

import (
	"context"

	assJob "github.com/jettjia/xiaoqinglong/agent-frame/application/assembler/job"
	dtoJob "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/job"
	srvJob "github.com/jettjia/xiaoqinglong/agent-frame/domain/srv/job"
)

type JobExecutionService struct {
	jobExecutionDto *assJob.JobExecutionDto
	jobExecutionSvc *srvJob.JobExecution
}

func NewJobExecutionService() *JobExecutionService {
	return &JobExecutionService{
		jobExecutionDto: assJob.NewJobExecutionDto(),
		jobExecutionSvc: srvJob.NewJobExecutionSvc(),
	}
}

func (s *JobExecutionService) FindJobExecutionById(ctx context.Context, req *dtoJob.FindJobExecutionByIdReq) (*dtoJob.FindJobExecutionRsp, error) {
	en, err := s.jobExecutionSvc.FindJobExecutionById(ctx, req.Ulid)
	if err != nil {
		return nil, err
	}

	dto := s.jobExecutionDto.E2DFindJobExecutionRsp(en)

	return dto, nil
}

func (s *JobExecutionService) FindJobExecutionByAgentId(ctx context.Context, req *dtoJob.FindJobExecutionByAgentIdReq) (*dtoJob.FindJobExecutionListRsp, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}

	ens, err := s.jobExecutionSvc.FindByAgentId(ctx, req.AgentId, limit)
	if err != nil {
		return nil, err
	}

	entries := s.jobExecutionDto.E2DGetJobExecutions(ens)

	return &dtoJob.FindJobExecutionListRsp{Entries: entries}, nil
}

func (s *JobExecutionService) FindJobExecutionPage(ctx context.Context, req *dtoJob.FindJobExecutionPageReq) (*dtoJob.FindJobExecutionPageRsp, error) {
	var rsp dtoJob.FindJobExecutionPageRsp

	ens, pageData, err := s.jobExecutionSvc.FindJobExecutionPage(ctx, req.Query, req.PageData, req.SortData)
	if err != nil {
		return nil, err
	}

	entries := s.jobExecutionDto.E2DGetJobExecutions(ens)
	rsp.Entries = entries
	rsp.PageData = pageData

	return &rsp, nil
}
