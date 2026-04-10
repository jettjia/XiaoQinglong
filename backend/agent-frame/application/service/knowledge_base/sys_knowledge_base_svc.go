package knowledge_base

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/jettjia/igo-pkg/pkg/xerror"
	"github.com/jettjia/igo-pkg/pkg/xsql/builder"

	ass "github.com/jettjia/xiaoqinglong/agent-frame/application/assembler/knowledge_base"
	dto "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/knowledge_base"
	srv "github.com/jettjia/xiaoqinglong/agent-frame/domain/srv/knowledge_base"
	"github.com/jettjia/xiaoqinglong/agent-frame/types/apierror"
)

type SysKnowledgeBaseService struct {
	sysKnowledgeBaseDto *ass.SysKnowledgeBaseDto
	sysKnowledgeBaseSrv *srv.SysKnowledgeBaseSvc
}

func NewSysKnowledgeBaseService() *SysKnowledgeBaseService {
	return &SysKnowledgeBaseService{
		sysKnowledgeBaseDto: ass.NewSysKnowledgeBaseDto(),
		sysKnowledgeBaseSrv: srv.NewSysKnowledgeBaseSvc(),
	}
}

func (s *SysKnowledgeBaseService) CreateSysKnowledgeBase(ctx context.Context, req *dto.CreateSysKnowledgeBaseReq) (*dto.CreateSysKnowledgeBaseRsp, error) {
	var rsp dto.CreateSysKnowledgeBaseRsp
	en := s.sysKnowledgeBaseDto.D2ECreateSysKnowledgeBase(req)

	ulid, err := s.sysKnowledgeBaseSrv.Create(ctx, en)
	if err != nil {
		return nil, err
	}
	rsp.Ulid = ulid

	return &rsp, nil
}

func (s *SysKnowledgeBaseService) DeleteSysKnowledgeBase(ctx context.Context, req *dto.DelSysKnowledgeBaseReq) error {
	en := s.sysKnowledgeBaseDto.D2EDeleteSysKnowledgeBase(req)

	return s.sysKnowledgeBaseSrv.Delete(ctx, en)
}

func (s *SysKnowledgeBaseService) UpdateSysKnowledgeBase(ctx context.Context, req *dto.UpdateSysKnowledgeBaseReq) error {
	en := s.sysKnowledgeBaseDto.D2EUpdateSysKnowledgeBase(req)

	return s.sysKnowledgeBaseSrv.Update(ctx, en)
}

func (s *SysKnowledgeBaseService) FindSysKnowledgeBaseById(ctx context.Context, req *dto.FindSysKnowledgeBaseByIdReq) (*dto.FindSysKnowledgeBaseRsp, error) {
	en, err := s.sysKnowledgeBaseSrv.FindById(ctx, req.Ulid)
	if err != nil {
		return nil, err
	}

	// 过滤已删除的记录
	if en == nil || en.DeletedAt != 0 {
		return nil, xerror.NewErrorOpt(apierror.NotFoundErr, xerror.WithCause("knowledge base not found or deleted"))
	}

	dtoRsp := s.sysKnowledgeBaseDto.E2DFindSysKnowledgeBaseRsp(en)

	return dtoRsp, nil
}

func (s *SysKnowledgeBaseService) FindSysKnowledgeBaseAll(ctx context.Context, req *dto.FindSysKnowledgeBaseAllReq) ([]*dto.FindSysKnowledgeBaseRsp, error) {
	queries := []*builder.Query{
		{Key: "deleted_at", Operator: builder.Operator_opEq, Value: 0},
	}

	ens, err := s.sysKnowledgeBaseSrv.FindAll(ctx, queries)
	if err != nil {
		return nil, err
	}

	dtos := s.sysKnowledgeBaseDto.E2DGetSysKnowledgeBases(ens)

	return dtos, nil
}

func (s *SysKnowledgeBaseService) FindSysKnowledgeBasePage(ctx context.Context, req *dto.FindSysKnowledgeBasePageReq) (*dto.FindSysKnowledgeBasePageRsp, error) {
	var rsp dto.FindSysKnowledgeBasePageRsp
	ens, pageData, err := s.sysKnowledgeBaseSrv.FindPage(ctx, req.Query, req.PageData, req.SortData)
	if err != nil {
		return nil, err
	}

	entries := s.sysKnowledgeBaseDto.E2DGetSysKnowledgeBases(ens)
	rsp.Entries = entries
	rsp.PageData = pageData

	return &rsp, nil
}

// RecallTest 执行召回测试
func (s *SysKnowledgeBaseService) RecallTest(ctx context.Context, ulid string, req *dto.RecallTestReq) ([]*dto.RecallTestRsp, error) {
	// 获取知识库配置
	kb, err := s.sysKnowledgeBaseSrv.FindById(ctx, ulid)
	if err != nil {
		return nil, err
	}

	if kb.RetrievalUrl == "" {
		return nil, fmt.Errorf("retrieval url is empty")
	}

	// 构建请求
	recallReq := map[string]interface{}{
		"query": req.Query,
		"top_k": req.TopK,
	}
	reqBody, err := json.Marshal(recallReq)
	if err != nil {
		return nil, err
	}

	// 创建HTTP请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", kb.RetrievalUrl, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// 设置Token认证
	if kb.Token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+kb.Token)
	}

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 解析响应
	var results []*dto.RecallTestRsp
	if err := json.Unmarshal(body, &results); err != nil {
		return nil, err
	}

	return results, nil
}
