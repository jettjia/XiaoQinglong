package channel

import (
	"context"

	"github.com/jettjia/igo-pkg/pkg/xsql/builder"

	ass "github.com/jettjia/xiaoqinglong/agent-frame/application/assembler/channel"
	dto "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/channel"
	srv "github.com/jettjia/xiaoqinglong/agent-frame/domain/srv/channel"
)

type SysChannelService struct {
	sysChannelDto *ass.SysChannelAssembler
	sysChannelSrv *srv.SysChannelSvc
}

func NewSysChannelService() *SysChannelService {
	return &SysChannelService{
		sysChannelDto: ass.NewSysChannelAssembler(),
		sysChannelSrv: srv.NewSysChannelSvc(),
	}
}

func (s *SysChannelService) CreateSysChannel(ctx context.Context, req *dto.CreateSysChannelReq) (*dto.CreateSysChannelRsp, error) {
	var rsp dto.CreateSysChannelRsp
	en := s.sysChannelDto.D2ECreateSysChannel(req)

	ulid, err := s.sysChannelSrv.Create(ctx, en)
	if err != nil {
		return nil, err
	}
	rsp.Ulid = ulid

	return &rsp, nil
}

func (s *SysChannelService) DeleteSysChannel(ctx context.Context, req *dto.DelSysChannelReq) error {
	en := s.sysChannelDto.D2EDeleteSysChannel(req)

	return s.sysChannelSrv.Delete(ctx, en)
}

func (s *SysChannelService) UpdateSysChannel(ctx context.Context, req *dto.UpdateSysChannelReq) error {
	en := s.sysChannelDto.D2EUpdateSysChannel(req)

	return s.sysChannelSrv.Update(ctx, en)
}

func (s *SysChannelService) FindSysChannelById(ctx context.Context, req *dto.FindSysChannelByIdReq) (*dto.FindSysChannelRsp, error) {
	en, err := s.sysChannelSrv.FindById(ctx, req.Ulid)
	if err != nil {
		return nil, err
	}

	dtoRsp := s.sysChannelDto.E2DFindSysChannelRsp(en)

	return dtoRsp, nil
}

func (s *SysChannelService) FindSysChannelAll(ctx context.Context, req *dto.FindSysChannelAllReq) ([]*dto.FindSysChannelRsp, error) {
	queries := []*builder.Query{
		{Key: "deleted_at", Operator: builder.Operator_opEq, Value: 0},
	}

	if req.Name != "" {
		queries = append(queries, &builder.Query{Key: "name", Operator: builder.Operator_opLike, Value: req.Name})
	}

	ens, err := s.sysChannelSrv.FindAll(ctx, queries)
	if err != nil {
		return nil, err
	}

	dtos := s.sysChannelDto.E2DGetSysChannels(ens)

	return dtos, nil
}

func (s *SysChannelService) FindSysChannelPage(ctx context.Context, req *dto.FindSysChannelPageReq) (*dto.FindSysChannelPageRsp, error) {
	var rsp dto.FindSysChannelPageRsp
	ens, pageData, err := s.sysChannelSrv.FindPage(ctx, req.Query, req.PageData, req.SortData)
	if err != nil {
		return nil, err
	}

	entries := s.sysChannelDto.E2DGetSysChannels(ens)
	rsp.Entries = entries
	rsp.PageData = pageData

	return &rsp, nil
}