package model

import (
	"context"
	"strings"

	"github.com/jettjia/igo-pkg/pkg/xerror"
	"github.com/jettjia/igo-pkg/pkg/xsql/builder"

	assModel "github.com/jettjia/xiaoqinglong/agent-frame/application/assembler/model"
	dtoAgent "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/agent"
	dtoModel "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/model"
	agent "github.com/jettjia/xiaoqinglong/agent-frame/application/service/agent"
	srvModel "github.com/jettjia/xiaoqinglong/agent-frame/domain/srv/model"
	"github.com/jettjia/xiaoqinglong/agent-frame/types/apierror"
)

// defaultModelConfig 默认模型配置
type defaultModelConfig struct {
	provider string
	name    string
	apiKey  string
	baseURL string
}

// buildModelConfigJson 根据模型配置生成 models JSON
func buildModelConfigJson(cfg *defaultModelConfig) string {
	return `{
					"default": {
						"provider": "` + cfg.provider + `",
						"name": "` + cfg.name + `",
						"api_key": "` + cfg.apiKey + `",
						"api_base": "` + cfg.baseURL + `"
					}
				}`
}

// replaceModelsConfigInJson 替换 configJson 中的 models 配置
func replaceModelsConfigInJson(configJson string, newModelsJSON string) string {
	startIdx := strings.Index(configJson, `"models": {`)
	if startIdx == -1 {
		return configJson
	}

	braceCount := 0
	inModels := false
	endIdx := startIdx
	for i := startIdx; i < len(configJson); i++ {
		if configJson[i] == '{' {
			braceCount++
			inModels = true
		} else if configJson[i] == '}' {
			braceCount--
			if braceCount == 0 && inModels {
				endIdx = i + 1
				break
			}
		}
	}

	modelsStart := strings.Index(configJson[startIdx:], `"models": {`) + startIdx
	return configJson[:modelsStart] + `"models": ` + newModelsJSON + configJson[endIdx:]
}

// syncBuiltInAgentsWithDefaultModel 同步内置智能体使用默认模型配置
func (s *SysModelService) syncBuiltInAgentsWithDefaultModel(ctx context.Context, modelCfg *defaultModelConfig) {
	// 查询所有内置智能体（is_system=true, created_by='system'）
	agents, err := s.agentSvc.FindSysAgentAll(ctx, &dtoAgent.FindSysAgentAllReq{})
	if err != nil {
		return
	}

	// 生成新的 models JSON 配置
	modelJSON := buildModelConfigJson(modelCfg)

	// 对每个内置智能体，更新 configJson
	for _, ag := range agents {
		if ag.CreatedBy != "system" || !ag.IsSystem {
			continue
		}
		// 重新生成 configJson，替换 models 配置块
		newConfigJson := replaceModelsConfigInJson(ag.ConfigJson, modelJSON)
		// 调用更新
		err := s.agentSvc.UpdateSysAgent(ctx, &dtoAgent.UpdateSysAgentReq{
			Ulid:       ag.Ulid,
			ConfigJson: newConfigJson,
			UpdatedBy:  "system",
		})
		if err != nil {
			continue
		}
	}
}

type SysModelService struct {
	sysModelDto *assModel.SysModelDto
	sysModelSrv *srvModel.SysModelSvc
	agentSvc    *agent.SysAgentService
}

func NewSysModelService() *SysModelService {
	return &SysModelService{
		sysModelDto: assModel.NewSysModelDto(),
		sysModelSrv: srvModel.NewSysModelSvc(),
		agentSvc:    agent.NewSysAgentService(),
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

	// 如果是默认模型，同步内置智能体
	if req.Category == "default" {
		modelCfg := &defaultModelConfig{
			provider: req.Provider,
			name:    req.Name,
			apiKey:  req.ApiKey,
			baseURL: req.BaseUrl,
		}
		go s.syncBuiltInAgentsWithDefaultModel(context.Background(), modelCfg)
	}

	return &rsp, nil
}

func (s *SysModelService) DeleteSysModel(ctx context.Context, req *dtoModel.DelSysModelReq) error {
	en := s.sysModelDto.D2EDeleteSysModel(req)

	return s.sysModelSrv.DeleteSysModel(ctx, en)
}

func (s *SysModelService) UpdateSysModel(ctx context.Context, req *dtoModel.UpdateSysModelReq) error {
	en := s.sysModelDto.D2EUpdateSysModel(req)

	err := s.sysModelSrv.UpdateSysModel(ctx, en)
	if err != nil {
		return err
	}

	// 如果是默认模型，同步内置智能体
	if req.Category == "default" {
		modelCfg := &defaultModelConfig{
			provider: req.Provider,
			name:    req.Name,
			apiKey:  req.ApiKey,
			baseURL: req.BaseUrl,
		}
		go s.syncBuiltInAgentsWithDefaultModel(context.Background(), modelCfg)
	}

	return nil
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

// FindDefaultModel 获取默认模型配置
func (s *SysModelService) FindDefaultModel(ctx context.Context) (*dtoModel.FindSysModelRsp, error) {
	queries := []*builder.Query{
		{Key: "deleted_at", Operator: builder.Operator_opEq, Value: 0},
		{Key: "category", Operator: builder.Operator_opEq, Value: "default"},
	}

	models, err := s.sysModelSrv.FindSysModelAll(ctx, queries)
	if err != nil {
		return nil, err
	}

	if len(models) == 0 {
		return nil, nil
	}

	return s.sysModelDto.E2DFindSysModelRsp(models[0]), nil
}
