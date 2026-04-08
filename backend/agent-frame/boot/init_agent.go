package boot

import (
	"context"
	"log"

	dtoAgent "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/agent"
	dtoModel "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/model"
	"github.com/jettjia/xiaoqinglong/agent-frame/application/service/agent"
	modelSvc "github.com/jettjia/xiaoqinglong/agent-frame/application/service/model"
)

// defaultModelConfig 默认模型配置（从 sys_model 表读取）
type defaultModelConfig struct {
	provider string
	name     string
	apiKey   string
	baseURL  string
}

// getDefaultModelConfig 从 sys_model 表获取默认模型配置
func getDefaultModelConfig(ctx context.Context) *defaultModelConfig {
	cfg := &defaultModelConfig{
		provider: "default",
		name:     "${OPENAI_MODEL}",
		apiKey:   "${OPENAI_API_KEY}",
		baseURL:  "${OPENAI_BASE_URL}",
	}

	// 尝试从 sys_model 表读取 category=default 的模型
	modelService := modelSvc.NewSysModelService()
	models, err := modelService.FindSysModelAll(ctx, &dtoModel.FindSysModelAllReq{})
	if err != nil {
		log.Printf("[Init] Failed to query sys_model: %v, using default env values", err)
		return cfg
	}

	// 查找 category 为 "default" 的模型
	for _, m := range models {
		if m.Category == "default" || m.Category == "" {
			// 找到默认模型，使用数据库中的配置
			cfg.provider = m.Provider
			cfg.name = m.Name
			cfg.apiKey = m.ApiKey
			cfg.baseURL = m.BaseUrl
			log.Printf("[Init] Found default model from sys_model: provider=%s, name=%s, baseURL=%s",
				cfg.provider, cfg.name, cfg.baseURL)
			return cfg
		}
	}

	log.Printf("[Init] No default model found in sys_model, using default env values")
	return cfg
}

// buildModelConfigJson 根据模型配置生成 models JSON
func buildModelConfigJson(modelCfg *defaultModelConfig) string {
	return `{
					"default": {
						"provider": "` + modelCfg.provider + `",
						"name": "` + modelCfg.name + `",
						"api_key": "` + modelCfg.apiKey + `",
						"api_base": "` + modelCfg.baseURL + `"
					}
				}`
}

// agentConfig 定义智能体配置结构
type agentConfig struct {
	name        string
	description string
	icon        string
	model       string
	configJson  string
	isSystem    bool
}

// getBuiltInAgents 获取内置智能体配置列表
func getBuiltInAgents(modelCfg *defaultModelConfig) []agentConfig {
	// 通用模型配置
	modelJSON := buildModelConfigJson(modelCfg)

	return []agentConfig{
		{
			name:        "翻译",
			description: "多语言实时翻译，支持中英日韩等常用语言互译",
			icon:        "Languages",
			model:       "default",
			configJson: `{
				"endpoint": "http://localhost:18080/run",
				"models": ` + modelJSON + `,
				"system_prompt": "你是一个专业的翻译助手。用户输入一段文字，你将其翻译成目标语言。请保持原文风格和语气。如果用户没有指定目标语言，如果是输入的是中文，就翻译成英文。如果输入的是英文，就翻译成中文。其他语言请翻译成英文。",
				"options": {
					"temperature": 0.3,
					"max_tokens": 2000,
					"max_iterations": 3,
					"stream": true,
					"approval_policy": {
						"enabled": false
					}
				},
				"context_window": {
					"max_rounds": 10,
					"max_tokens": 32000,
					"strategy": "sliding_window"
				},
				"long_term_memory": {
					"enabled": true,
					"max_count": 5
				}
			}`,
			isSystem: true,
		},
		{
			name:        "文档问答",
			description: "基于文档内容的智能问答，可以从上传的文档中查找答案",
			icon:        "FileSearch",
			model:       "default",
			configJson: `{
				"endpoint": "http://localhost:18080/run",
				"models": ` + modelJSON + `,
				"system_prompt": "你是一个专业的文档问答助手。根据用户提供的文档内容，准确回答用户的问题。如果文档中没有相关信息，请明确告知。",
				"options": {
					"temperature": 0.2,
					"max_tokens": 4000,
					"max_iterations": 5,
					"stream": true,
					"approval_policy": {
						"enabled": false
					},
					"routing": {
						"default_model": "default",
						"rewrite_prompt": "请优化以下用户Query，使其更加清晰、准确，便于理解。只返回优化后的Query，不要其他内容。",
						"summarize_prompt": "请总结以下内容，提取关键信息，保持简洁。只返回总结内容，不要其他内容。"
					}
				},
				"context_window": {
					"max_rounds": 20,
					"max_tokens": 64000,
					"strategy": "sliding_window"
				},
				"long_term_memory": {
					"enabled": true,
					"max_count": 10
				}
			}`,
			isSystem: true,
		},
		{
			name:        "数据分析",
			description: "分析 CSV/Excel 数据文件，生成交互式 HTML 报告，支持数据可视化、统计摘要、异常检测等",
			icon:        "ChartBar",
			model:       "default",
			configJson: `{
				"endpoint": "http://localhost:18080/run",
				"models": ` + modelJSON + `,
				"system_prompt": "你是一个数据分析专家。当用户上传 CSV/Excel 文件并要求分析时，请按以下步骤执行：\n\nStep 0: 首先使用 parse_file 工具读取上传的 CSV/Excel 文件，获取文件内容\nStep 1: 使用 csv-data-analysis skill 分析数据文件，执行 csv_analyzer.py 获取统计摘要和图表数据\nStep 2: 根据分析结果生成业务洞察，并返回完整的 HTML 报告\n\n报告应包含：数据概览、分布分析、相关性分析、异常检测、结论与建议。",
				"skills": [
					{
						"id": "csv-data-analysis",
						"name": "CSV数据分析",
						"description": "用于分析 CSV 或 Excel 文件，理解数据模式，生成统计摘要和数据可视化",
						"scope": "both",
						"trigger": "auto",
						"risk_level": "low"
					}
				],
				"options": {
					"temperature": 0.3,
					"max_tokens": 8000,
					"max_iterations": 10,
					"stream": true,
					"approval_policy": {
						"enabled": false
					}
				},
				"sandbox": {
					"enabled": true,
					"mode": "docker",
					"image": "sandbox-code-interpreter:v1.0.3",
					"workdir": "/workspace",
					"timeout_ms": 120000,
					"network": "bridge",
					"env": {
						"PATH": "/usr/local/bin:/usr/bin:/bin"
					},
					"limits": {
						"cpu": "0.5",
						"memory": "512m"
					}
				},
				"context_window": {
					"max_rounds": 10,
					"max_tokens": 32000,
					"strategy": "sliding_window"
				},
				"long_term_memory": {
					"enabled": false
				}
			}`,
			isSystem: true,
		},
	}
}

// initDefaultAgents 初始化默认智能体
func initDefaultAgents() error {
	log.Println("[Init] Initializing default agents")

	agentSvc := agent.NewSysAgentService()
	ctx := context.Background()

	// 获取默认模型配置（优先从 sys_model 表读取）
	modelCfg := getDefaultModelConfig(ctx)

	// 获取内置智能体配置
	defaultAgents := getBuiltInAgents(modelCfg)

	for _, ag := range defaultAgents {
		// 检查是否已存在同名智能体
		existing, err := agentSvc.FindSysAgentAll(ctx, &dtoAgent.FindSysAgentAllReq{Name: ag.name})
		if err != nil {
			log.Printf("[Init] Failed to check agent %s: %v", ag.name, err)
			continue
		}
		if len(existing) > 0 {
			// 已存在，跳过
			log.Printf("[Init] Agent already exists: %s", ag.name)
			continue
		}

		// 创建智能体
		createReq := &dtoAgent.CreateSysAgentReq{
			Name:        ag.name,
			Description: ag.description,
			Icon:        ag.icon,
			Model:       ag.model,
			ConfigJson:  ag.configJson,
			Enabled:     true,
			IsSystem:    ag.isSystem,
			CreatedBy:   "system",
		}

		_, err = agentSvc.CreateSysAgent(ctx, createReq)
		if err != nil {
			log.Printf("[Init] Failed to create agent %s: %v", ag.name, err)
			return err
		}
		log.Printf("[Init] Created agent: %s", ag.name)
	}

	return nil
}
