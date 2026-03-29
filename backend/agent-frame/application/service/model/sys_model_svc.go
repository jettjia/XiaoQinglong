package model

import (
	"context"

	"github.com/jettjia/igo-pkg/pkg/xerror"
	"github.com/jettjia/igo-pkg/pkg/xsql/builder"

	assModel "github.com/jettjia/xiaoqinglong/agent-frame/application/assembler/model"
	dtoModel "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/model"
	srvModel "github.com/jettjia/xiaoqinglong/agent-frame/domain/srv/model"
	"github.com/jettjia/xiaoqinglong/agent-frame/types/apierror"
)

type SysModelService struct {
	sysModelDto *assModel.SysModelDto
	sysModelSrv *srvModel.SysModelSvc
}

func NewSysModelService() *SysModelService {
	return &SysModelService{
		sysModelDto: assModel.NewSysModelDto(),
		sysModelSrv: srvModel.NewSysModelSvc(),
	}
}

func (s *SysModelService) CreateSysModel(ctx context.Context, req *dtoModel.CreateSysModelReq) (*dtoModel.CreateSysModelRsp, error) {
	var rsp dtoModel.CreateSysModelRsp
	en := s.sysModelDto.D2ECreateSysModel(req)

	ulid, err := s.sysModelSrv.CreateSysModel(ctx, en)
	if err != nil {
		return nil, err
	}
	rsp.Ulid = ulid

	return &rsp, nil
}

func (s *SysModelService) DeleteSysModel(ctx context.Context, req *dtoModel.DelSysModelReq) error {
	en := s.sysModelDto.D2EDeleteSysModel(req)

	return s.sysModelSrv.DeleteSysModel(ctx, en)
}

func (s *SysModelService) UpdateSysModel(ctx context.Context, req *dtoModel.UpdateSysModelReq) error {
	en := s.sysModelDto.D2EUpdateSysModel(req)

	return s.sysModelSrv.UpdateSysModel(ctx, en)
}

func (s *SysModelService) FindSysModelById(ctx context.Context, req *dtoModel.FindSysModelByIdReq) (*dtoModel.FindSysModelRsp, error) {
	en, err := s.sysModelSrv.FindSysModelById(ctx, req.Ulid)
	if err != nil {
		return nil, err
	}

	// 过滤已删除的记录
	if en == nil || en.DeletedAt != 0 {
		return nil, xerror.NewErrorOpt(apierror.NotFoundErr, xerror.WithCause("model not found or deleted"))
	}

	dto := s.sysModelDto.E2DFindSysModelRsp(en)

	return dto, nil
}

func (s *SysModelService) FindSysModelAll(ctx context.Context, req *dtoModel.FindSysModelAllReq) ([]*dtoModel.FindSysModelRsp, error) {
	queries := []*builder.Query{
		{Key: "deleted_at", Operator: builder.Operator_opEq, Value: 0},
	}
	if req.ModelType != "" {
		queries = append(queries, &builder.Query{Key: "model_type", Operator: builder.Operator_opEq, Value: req.ModelType})
	}

	ens, err := s.sysModelSrv.FindSysModelAll(ctx, queries)
	if err != nil {
		return nil, err
	}

	dtos := s.sysModelDto.E2DGetSysModels(ens)

	return dtos, nil
}

func (s *SysModelService) FindSysModelPage(ctx context.Context, req *dtoModel.FindSysModelPageReq) (*dtoModel.FindSysModelPageRsp, error) {
	var rsp dtoModel.FindSysModelPageRsp
	ens, pageData, err := s.sysModelSrv.FindSysModelPage(ctx, req.Query, req.PageData, req.SortData)
	if err != nil {
		return nil, err
	}

	entries := s.sysModelDto.E2DGetSysModels(ens)
	rsp.Entries = entries
	rsp.PageData = pageData

	return &rsp, nil
}
