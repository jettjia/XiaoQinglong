package agent

import (
	"context"

	"github.com/jettjia/igo-pkg/pkg/xerror"
	"github.com/jettjia/igo-pkg/pkg/xsql/builder"

	ass "github.com/jettjia/xiaoqinglong/agent-frame/application/assembler/agent"
	dto "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/agent"
	srv "github.com/jettjia/xiaoqinglong/agent-frame/domain/srv/agent"
	"github.com/jettjia/xiaoqinglong/agent-frame/types/apierror"

	"github.com/jettjia/xiaoqinglong/agent-frame/api/job"
)

type SysAgentService struct {
	sysAgentDto *ass.SysAgentAssembler
	sysAgentSrv *srv.SysAgentSvc
}

func NewSysAgentService() *SysAgentService {
	return &SysAgentService{
		sysAgentDto: ass.NewSysAgentAssembler(),
		sysAgentSrv: srv.NewSysAgentSvc(),
	}
}

func (s *SysAgentService) CreateSysAgent(ctx context.Context, req *dto.CreateSysAgentReq) (*dto.CreateSysAgentRsp, error) {
	var rsp dto.CreateSysAgentRsp
	en := s.sysAgentDto.D2ECreateSysAgent(req)

	ulid, err := s.sysAgentSrv.Create(ctx, en)
	if err != nil {
		return nil, err
	}
	rsp.Ulid = ulid

	// 如果是周期任务，注册到 JobManager
	if req.IsPeriodic && req.CronRule != "" {
		jm := job.GetJobManager()
		if jm != nil {
			_ = jm.AddCronJob(ulid, req.Name, req.CronRule, req.Config)
		}
	}

	return &rsp, nil
}

func (s *SysAgentService) DeleteSysAgent(ctx context.Context, req *dto.DelSysAgentReq) error {
	// 检查是否为系统内置Agent
	existing, err := s.sysAgentSrv.FindById(ctx, req.Ulid)
	if err != nil {
		return err
	}
	if existing == nil || existing.DeletedAt != 0 {
		return xerror.NewErrorOpt(apierror.NotFoundErr, xerror.WithCause("agent not found"))
	}
	if existing.IsSystem {
		return xerror.NewErrorOpt(apierror.ForbiddenErr, xerror.WithCause("系统内置Agent不能删除"))
	}

	en := s.sysAgentDto.D2EDeleteSysAgent(req)

	err = s.sysAgentSrv.Delete(ctx, en)
	if err != nil {
		return err
	}

	// 从 JobManager 中移除 cron job
	jm := job.GetJobManager()
	if jm != nil {
		_ = jm.RemoveCronJob(req.Ulid)
	}

	return nil
}

func (s *SysAgentService) UpdateSysAgent(ctx context.Context, req *dto.UpdateSysAgentReq) error {
	en := s.sysAgentDto.D2EUpdateSysAgent(req)
	err := s.sysAgentSrv.Update(ctx, en)
	if err != nil {
		return err
	}

	// 更新 JobManager 中的 cron job
	jm := job.GetJobManager()
	if jm != nil {
		if req.IsPeriodic != nil && *req.IsPeriodic && req.CronRule != "" {
			// 需要重新注册
			_ = jm.UpdateCronJob(req.Ulid, req.Name, req.CronRule, req.Config)
		} else {
			// 需要删除
			_ = jm.RemoveCronJob(req.Ulid)
		}
	}

	return nil
}

// UpdateSysAgentEnabled 修改启用状态
func (s *SysAgentService) UpdateSysAgentEnabled(ctx context.Context, req *dto.UpdateSysAgentEnabledReq) error {
	return s.sysAgentSrv.UpdateEnabled(ctx, req.Ulid, req.Enabled)
}

func (s *SysAgentService) FindSysAgentById(ctx context.Context, req *dto.FindSysAgentByIdReq) (*dto.FindSysAgentRsp, error) {
	en, err := s.sysAgentSrv.FindById(ctx, req.Ulid)
	if err != nil {
		return nil, err
	}

	// 过滤已删除的记录
	if en == nil || en.DeletedAt != 0 {
		return nil, xerror.NewErrorOpt(apierror.NotFoundErr, xerror.WithCause("agent not found or deleted"))
	}

	dtoRsp := s.sysAgentDto.E2DFindSysAgentRsp(en)

	return dtoRsp, nil
}

func (s *SysAgentService) FindSysAgentAll(ctx context.Context, req *dto.FindSysAgentAllReq) ([]*dto.FindSysAgentRsp, error) {
	queries := []*builder.Query{
		{Key: "deleted_at", Operator: builder.Operator_opEq, Value: 0},
	}

	if req.Name != "" {
		queries = append(queries, &builder.Query{Key: "name", Operator: builder.Operator_opLike, Value: req.Name})
	}

	ens, err := s.sysAgentSrv.FindAll(ctx, queries)
	if err != nil {
		return nil, err
	}

	dtos := s.sysAgentDto.E2DGetSysAgents(ens)

	return dtos, nil
}

func (s *SysAgentService) FindSysAgentPage(ctx context.Context, req *dto.FindSysAgentPageReq) (*dto.FindSysAgentPageRsp, error) {
	var rsp dto.FindSysAgentPageRsp
	ens, pageData, err := s.sysAgentSrv.FindPage(ctx, req.Query, req.PageData, req.SortData)
	if err != nil {
		return nil, err
	}

	entries := s.sysAgentDto.E2DGetSysAgents(ens)
	rsp.Entries = entries
	rsp.PageData = pageData

	return &rsp, nil
}

// UploadSysAgent 上传并导入Agent JSON配置
func (s *SysAgentService) UploadSysAgent(ctx context.Context, req *dto.UploadSysAgentReq) (*dto.CreateSysAgentRsp, error) {
	// 检查同名Agent是否已存在
	existing, err := s.sysAgentSrv.FindByName(ctx, req.Name)
	if err != nil {
		return nil, err
	}

	if existing != nil && existing.DeletedAt == 0 {
		return nil, xerror.NewErrorOpt(apierror.ConflictErr, xerror.WithCause("Agent name already exists"))
	}

	// 创建数据库记录
	createReq := &dto.CreateSysAgentReq{
		Name:        req.Name,
		Description: req.Description,
		Icon:        req.Icon,
		Model:       req.Model,
		Config:      req.Config,
		Enabled:     req.Enabled,
		IsSystem:    false,
	}

	return s.CreateSysAgent(ctx, createReq)
}
