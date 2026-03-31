package boot

import (
	"context"
	"log"

	dtoAgent "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/agent"
	"github.com/jettjia/xiaoqinglong/agent-frame/application/service/agent"
)

// initDefaultAgents 初始化默认智能体
func initDefaultAgents() error {
	log.Println("[Init] Initializing default agents")

	agentSvc := agent.NewSysAgentService()
	ctx := context.Background()

	// 默认智能体列表
	defaultAgents := []struct {
		name        string
		description string
		icon        string
		model       string
		configJson  string
		isSystem    bool
	}{
		{
			name:        "翻译",
			description: "多语言实时翻译，支持中英日韩等常用语言互译",
			icon:        "Languages",
			model:       "default",
			configJson: `{
				"endpoint": "http://localhost:18080/run",
				"models": {
					"default": {
						"provider": "default",
						"name": "${OPENAI_MODEL}",
						"api_key": "${OPENAI_API_KEY}",
						"api_base": "${OPENAI_BASE_URL}"
					}
				},
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
				"models": {
					"default": {
						"provider": "default",
						"name": "${OPENAI_MODEL}",
						"api_key": "${OPENAI_API_KEY}",
						"api_base": "${OPENAI_BASE_URL}"
					}
				},
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
	}

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
