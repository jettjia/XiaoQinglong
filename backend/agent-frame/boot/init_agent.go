package boot

import (
	"context"
	"log"
	"os"

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

// getRunnerEndpoint 获取 runner endpoint，支持环境变量配置
func getRunnerEndpoint() string {
	if endpoint := os.Getenv("RUNNER_ENDPOINT"); endpoint != "" {
		return endpoint
	}
	return "http://localhost:18080/run"
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
	sort        int
}

// getBuiltInAgents 获取内置智能体配置列表
func getBuiltInAgents(modelCfg *defaultModelConfig) []agentConfig {
	// 通用模型配置
	modelJSON := buildModelConfigJson(modelCfg)

	return []agentConfig{
		{
			name:        "快速问答",
			description: "通用对话助手，可以调用任意技能和工具处理用户问题，灵活应对各种需求",
			icon:        "MessageCircle",
			model:       "default",
			configJson: `{
				"endpoint": "` + getRunnerEndpoint() + `",
				"models": ` + modelJSON + `,
				"system_prompt": "你是一个智能助手，名称叫小青龙(Dragon Agent OS)。你可以根据用户的问题，灵活调用任何可用的技能（skills）和工具（tools）来解决问题。\n\n【身份定义】\n你是一个通用智能助手，擅长使用工具和技能来完成各种任务。你应该主动行动而不是仅仅描述你要做什么。\n\n【工具使用准则】\n你必须使用工具来采取行动——不要只描述你会做什么或计划做什么。当你承诺执行某个操作时，必须立即在同一个回复中调用相应的工具。不要用"我会..."来结束回合，必须立即执行。\n持续工作直到任务真正完成。不要在承诺下一步行动后停下来。如果你有可用的工具来完成的任务，就使用它们。\n\n【技能系统 - 渐进式披露】\n技能遵循渐进式披露模式：\n1. 识别技能何时适用：检查用户任务是否匹配技能的描述或触发条件\n2. 加载技能详情：使用 'load_skill' 工具获取完整指令\n3. 执行技能：使用 'run_skill' 工具执行技能\n4. 复杂任务：使用 'orchestrate_skills' 来规划和执行多步骤技能\n\n【技能自动创建】\n完成复杂任务（5+ 工具调用）、解决棘手问题、或发现非平凡工作流程时，使用 'skill_manage' 工具创建新技能来保存这个方法，以便将来复用。\n\n创建技能的时机：\n- 复杂任务成功完成（5+ 工具调用）\n- 解决了棘手的问题\n- 发现了非平凡的工作流程\n- 用户要求记住一个流程\n\n当使用技能发现其过时、不完整或错误时，立即使用 skill_manage(action='patch') 打补丁——不要等待被要求。\n\n良好的技能包括：\n- 清晰的触发条件（何时使用此技能）\n- 带精确命令的编号步骤\n- 陷阱部分（常见错误）\n- 验证步骤（如何确认成功）\n\n【输出格式要求】\n1. 直接输出 Markdown 格式，不要用代码块包裹整个回答\n2. 表格使用标准 Markdown 语法\n3. 代码块必须指定语言\n4. 列表、引用等使用标准 Markdown 语法\n\n【能力范围】\n- 知识问答和信息检索\n- 文档处理和分析\n- 代码编写和调试\n- 数据处理和可视化\n- 文件生成（PPT、Excel、Word等）\n- 翻译和语言处理\n- 复杂的多步骤任务\n\n【回答原则】\n1. 理解用户意图\n2. 决定是否需要调用技能或工具\n3. 按需调用并整合结果\n4. 提供清晰完整的回答，使用 Markdown 格式化输出",
				"options": {
					"temperature": 0.7,
					"max_tokens": 8000,
					"max_iterations": 20,
					"stream": true,
					"approval_policy": {
						"enabled": false
					},
					"retry": {
						"max_attempts": 3,
						"initial_delay_ms": 1000,
						"backoff_multiplier": 2.0,
						"max_delay_ms": 30000
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
			sort:     1,
		},
		{
			name:        "翻译",
			description: "多语言实时翻译，支持中英日韩等常用语言互译",
			icon:        "Languages",
			model:       "default",
			configJson: `{
				"endpoint": "` + getRunnerEndpoint() + `",
				"models": ` + modelJSON + `,
				"system_prompt": "你是一个专业的翻译助手。用户输入一段文字，你将其翻译成目标语言。请保持原文风格和语气。如果用户没有指定目标语言，如果是输入的是中文，就翻译成英文。如果输入的是英文，就翻译成中文。其他语言请翻译成英文。",
				"options": {
					"temperature": 0.3,
					"max_tokens": 2000,
					"max_iterations": 3,
					"stream": true,
					"approval_policy": {
						"enabled": false
					},
					"retry": {
						"max_attempts": 3,
						"initial_delay_ms": 1000,
						"backoff_multiplier": 2.0,
						"max_delay_ms": 30000
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
			sort:     2,
		},
		{
			name:        "文档问答",
			description: "基于文档内容的智能问答，可以从上传的文档中查找答案",
			icon:        "FileSearch",
			model:       "default",
			configJson: `{
				"endpoint": "` + getRunnerEndpoint() + `",
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
					},
					"retry": {
						"max_attempts": 3,
						"initial_delay_ms": 1000,
						"backoff_multiplier": 2.0,
						"max_delay_ms": 30000
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
			sort:     3,
		},
		{
			name:        "数据分析",
			description: "分析 CSV/Excel 数据文件，生成交互式 HTML 报告，支持数据可视化、统计摘要、异常检测等",
			icon:        "ChartBar",
			model:       "default",
			configJson: `{
				"endpoint": "` + getRunnerEndpoint() + `",
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
					},
					"retry": {
						"max_attempts": 3,
						"initial_delay_ms": 1000,
						"backoff_multiplier": 2.0,
						"max_delay_ms": 30000
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
			sort:     4,
		},
		{
			name:        "生成PPT",
			description: "根据用户输入的主题或上传的文件内容，自动生成专业的 PowerPoint 演示文稿，支持多页布局、数据图表和精美设计",
			icon:        "Presentation",
			model:       "default",
			configJson: `{
				"endpoint": "` + getRunnerEndpoint() + `",
				"models": ` + modelJSON + `,
				"system_prompt": "你是一个专业的 PPT 制作专家。当用户要求生成 PPT 时，请按以下步骤执行：\n\nStep 0: 首先使用 parse_file 工具读取上传的文件内容（如果有）\nStep 1: 理解用户需求，规划 PPT 结构（封面页、内容页、总结页等）\nStep 2: 使用 pptx skill 生成 PPT，按照 SKILL.md 中的指引调用工具执行脚本\nStep 3: 生成的 PPT 文件会自动保存到报告目录，返回文件路径给用户\n\nPPT 要求：\n- 封面页：标题、副标题\n- 内容页：根据主题设计 3-10 页内容\n- 总结页：核心要点回顾\n- 每页需要有视觉元素（图表、图标等），不要纯文字堆砌\n- 使用专业的配色方案",
				"skills": [
					{
						"id": "pptx",
						"name": "PPT生成",
						"description": "用于生成 PowerPoint 演示文稿，支持从主题或文件内容生成精美 PPT",
						"scope": "both",
						"trigger": "auto",
						"risk_level": "low"
					}
				],
				"options": {
					"temperature": 0.5,
					"max_tokens": 8000,
					"max_iterations": 10,
					"stream": true,
					"approval_policy": {
						"enabled": false
					},
					"retry": {
						"max_attempts": 3,
						"initial_delay_ms": 1000,
						"backoff_multiplier": 2.0,
						"max_delay_ms": 30000
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
			sort:     5,
		},
	}
}

// shouldResetAgents 检查是否需要重置智能体（环境变量 INIT_AGENT=true）
func shouldResetAgents() bool {
	return os.Getenv("INIT_AGENT") == "true"
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

	// 检查是否需要重置
	resetAgents := shouldResetAgents()
	if resetAgents {
		log.Println("[Init] INIT_AGENT=true, will reset existing agents")
	}

	for _, ag := range defaultAgents {
		// 检查是否已存在同名智能体
		existing, err := agentSvc.FindSysAgentAll(ctx, &dtoAgent.FindSysAgentAllReq{Name: ag.name})
		if err != nil {
			log.Printf("[Init] Failed to check agent %s: %v", ag.name, err)
			continue
		}
		if len(existing) > 0 {
			if resetAgents {
				// 删除已存在的智能体
				oldAgent := existing[0]
				if err := agentSvc.DeleteSysAgent(ctx, &dtoAgent.DelSysAgentReq{Ulid: oldAgent.Ulid}); err != nil {
					log.Printf("[Init] Failed to delete agent %s (ulid=%s): %v", ag.name, oldAgent.Ulid, err)
					continue
				}
				log.Printf("[Init] Deleted existing agent: %s (ulid=%s)", ag.name, oldAgent.Ulid)
			} else {
				// 已存在，跳过
				log.Printf("[Init] Agent already exists: %s", ag.name)
				continue
			}
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
			Sort:        ag.sort,
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
