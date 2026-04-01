package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/jettjia/XiaoQinglong/runner/pkg/logger"
	"github.com/jettjia/XiaoQinglong/runner/retriever"
	"github.com/jettjia/XiaoQinglong/runner/subagent"
)

func (d *Dispatcher) initModels(ctx context.Context) error {
	d.models = make(map[string]model.ToolCallingChatModel)
	d.modelsByRole = make(map[ModelRole]model.ToolCallingChatModel)

	for key, cfg := range d.request.Models {
		cm, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
			APIKey:  cfg.APIKey,
			Model:   cfg.Name,
			BaseURL: cfg.APIBase,
		})
		if err != nil {
			return fmt.Errorf("create model %s failed: %w", key, err)
		}
		d.models[key] = cm

		// 设置 by role
		switch ModelRole(key) {
		case ModelRoleDefault:
			d.defaultModel = cm
			d.defaultModelName = cfg.Name
			d.modelsByRole[ModelRoleDefault] = cm
		case ModelRoleRewrite:
			d.modelsByRole[ModelRoleRewrite] = cm
		case ModelRoleSkill:
			d.modelsByRole[ModelRoleSkill] = cm
		case ModelRoleSummarize:
			d.modelsByRole[ModelRoleSummarize] = cm
		default:
			// 如果 key 不是已知的 role，也添加到 byRole map
			d.modelsByRole[ModelRole(key)] = cm
		}
	}

	// 如果没有配置 default，使用第一个模型
	if d.defaultModel == nil && len(d.models) > 0 {
		for key, cm := range d.models {
			d.defaultModel = cm
			d.defaultModelName = d.request.Models[key].Name
			break
		}
	}

	if d.defaultModel == nil {
		return fmt.Errorf("no default model configured")
	}

	return nil
}

func (d *Dispatcher) initTools(ctx context.Context) error {
	for _, tc := range d.request.Tools {
		// 存储工具配置以便在中断时查找
		d.toolConfigs[tc.Name] = tc
		switch tc.Type {
		case "http":
			httpTool := NewHTTPTool(tc)
			// 根据 risk_level 判断是否需要包装审批
			wrapped := d.wrapToolWithApproval(httpTool, tc.Name, "http", tc.RiskLevel)
			d.tools = append(d.tools, wrapped)
		}
	}
	return nil
}

// initKnowledgeRetriever 初始化知识检索器（作为工具供 Agent 调用）
func (d *Dispatcher) initKnowledgeRetriever(ctx context.Context) error {
	// 如果没有配置知识库，则跳过
	if len(d.request.KnowledgeBases) == 0 {
		logger.Infof("[Dispatcher] initKnowledgeRetriever: no knowledge bases configured")
		return nil
	}

	// 创建检索工具
	retrievalTool := retriever.CreateRetrievalTool(d.request.KnowledgeBases)
	d.tools = append(d.tools, retrievalTool)

	logger.Infof("[Dispatcher] initKnowledgeRetriever: added retrieval tool with %d knowledge bases", len(d.request.KnowledgeBases))
	return nil
}

// initFileRetrieval 初始化文件检索工具（作为工具供 Agent 调用）
func (d *Dispatcher) initFileRetrieval(ctx context.Context) error {
	// 如果没有上传文件或 uploadsBaseDir，则跳过
	if d.uploadsBaseDir == "" {
		logger.Infof("[Dispatcher] initFileRetrieval: no uploads base dir configured")
		return nil
	}

	// 创建文件检索工具
	fileRetrievalTool := retriever.CreateFileRetrievalTool(d.uploadsBaseDir)
	d.tools = append(d.tools, fileRetrievalTool)

	logger.Infof("[Dispatcher] initFileRetrieval: added file retrieval tool")
	return nil
}

func (d *Dispatcher) initA2A(ctx context.Context) error {
	d.a2aRunners = make(map[string]*adk.Runner)

	for _, cfg := range d.request.A2A {
		client, err := NewA2AClient(ctx, cfg)
		if err != nil {
			logger.Infof("[Dispatcher] initA2A: failed to create client for %s: %v", cfg.Name, err)
			continue
		}

		runner, err := client.CreateA2ARunner(ctx, d.defaultModel)
		if err != nil {
			logger.Infof("[Dispatcher] initA2A: failed to create runner for %s: %v", cfg.Name, err)
			continue
		}

		d.a2aRunners[cfg.Name] = runner
		logger.Infof("[Dispatcher] initA2A: registered agent %s", cfg.Name)
	}

	// 如果有 A2A agents，创建 A2A tool 并添加到 tools
	if len(d.a2aRunners) > 0 {
		clients := make(map[string]*A2AClient)
		for name, runner := range d.a2aRunners {
			_ = runner // runner 已存储
			// 创建 client 引用
			for _, cfg := range d.request.A2A {
				if cfg.Name == name {
					client, _ := NewA2AClient(ctx, cfg)
					if client != nil {
						clients[name] = client
					}
				}
			}
		}
		if len(clients) > 0 {
			// 重置计数器并使用带计数限制的 A2A tool
			d.a2aCallCount = 0
			maxA2ACalls := 0
			if d.request.Options != nil {
				maxA2ACalls = d.request.Options.MaxA2ACalls
			}
			a2aTool := NewA2AToolWithCounter(clients, &d.a2aCallCount, maxA2ACalls)
			// 设置 trace context
			if d.request.Context != nil {
				traceCtx := make(map[string]string)
				if v, ok := d.request.Context["trace_id"].(string); ok {
					traceCtx["trace_id"] = v
				}
				if v, ok := d.request.Context["parent_span_id"].(string); ok {
					traceCtx["parent_span_id"] = v
				}
				if len(traceCtx) > 0 {
					a2aTool.SetTraceContext(traceCtx)
					logger.Infof("[Dispatcher] A2A trace context set: %v", traceCtx)
				}
			}

			// 获取 A2A risk level（使用配置中的 risk_level，默认 medium）
			a2aRiskLevel := "medium"
			if len(d.request.A2A) > 0 && d.request.A2A[0].RiskLevel != "" {
				a2aRiskLevel = d.request.A2A[0].RiskLevel
			}

			// 根据 risk_level 判断是否需要包装审批
			wrappedA2ATool := d.wrapToolWithApproval(a2aTool, "a2a", "a2a", a2aRiskLevel)
			d.tools = append(d.tools, wrappedA2ATool)
		}
	}

	logger.Infof("[Dispatcher] initA2A: %d agents initialized", len(d.a2aRunners))
	return nil
}

func (d *Dispatcher) initInternalAgents(ctx context.Context) error {
	d.internalAgents = make(map[string]adk.Agent)

	for _, cfg := range d.request.InternalAgents {
		agent, err := d.createInternalAgent(ctx, cfg)
		if err != nil {
			logger.Infof("[Dispatcher] initInternalAgents: failed to create agent %s: %v", cfg.Name, err)
			continue
		}

		d.internalAgents[cfg.ID] = agent
		logger.Infof("[Dispatcher] initInternalAgents: registered agent %s (%s)", cfg.Name, cfg.ID)
	}

	logger.Infof("[Dispatcher] initInternalAgents: %d agents initialized", len(d.internalAgents))
	return nil
}

// initSubAgents 初始化 Sub-Agent 管理器
func (d *Dispatcher) initSubAgents(ctx context.Context) error {
	if len(d.request.SubAgents) == 0 {
		logger.Infof("[Dispatcher] initSubAgents: no sub-agents configured")
		return nil
	}

	// 创建 Sub-Agent 管理器
	d.subAgentManager = subagent.NewSubAgentManager(d.defaultModel)

	// 注册 Sub-Agent 配置
	d.subAgentManager.RegisterConfigs(d.request.SubAgents)

	// 创建 spawn 工具（异步并行执行）
	spawnTool := subagent.NewSpawnTool(d.subAgentManager)
	d.tools = append(d.tools, spawnTool)

	// 创建 collect_task 工具（获取异步任务结果）
	collectTool := subagent.NewCollectTaskTool(d.subAgentManager)
	d.tools = append(d.tools, collectTool)

	// 创建 list_tasks 工具（列出所有任务）
	listTasksTool := subagent.NewListTasksTool(d.subAgentManager)
	d.tools = append(d.tools, listTasksTool)

	// 创建 cancel_task 工具（取消任务）
	cancelTool := subagent.NewCancelTaskTool(d.subAgentManager)
	d.tools = append(d.tools, cancelTool)

	// 创建 Delegate 工具（同步执行，保持向后兼容）
	delegateTool := subagent.NewDelegateTool(d.subAgentManager)
	d.tools = append(d.tools, delegateTool)

	logger.Infof("[Dispatcher] initSubAgents: %d sub-agents registered, spawn/collect/list/cancel/delegate tools added", len(d.request.SubAgents))
	return nil
}

// initMCPs 初始化 MCP 工具
func (d *Dispatcher) initMCPs(ctx context.Context) error {
	if len(d.request.MCPs) == 0 {
		return nil
	}

	for _, mcpCfg := range d.request.MCPs {
		logger.Infof("[Dispatcher] initMCP: name=%s, transport=%s", mcpCfg.Name, mcpCfg.Transport)

		switch mcpCfg.Transport {
		case "http":
			// HTTP 模式：通过 HTTP API 加载 tools
			if mcpCfg.Endpoint == "" {
				logger.Infof("[Dispatcher] initMCP: %s has empty endpoint, skipping", mcpCfg.Name)
				continue
			}
			loader := NewMCPToolLoader(mcpCfg.Endpoint, mcpCfg.Headers)
			tools, err := loader.LoadTools(ctx)
			if err != nil {
				logger.Infof("[Dispatcher] initMCP: failed to load tools for %s: %v", mcpCfg.Name, err)
				continue
			}
			for _, t := range tools {
				// HTTP MCP tools 需要包装审批
				if invokableTool, ok := t.(tool.InvokableTool); ok {
					wrapped := d.wrapToolWithApproval(invokableTool, mcpCfg.Name, "mcp", mcpCfg.RiskLevel)
					d.tools = append(d.tools, wrapped)
				} else {
					d.tools = append(d.tools, t)
				}
			}
			logger.Infof("[Dispatcher] initMCP: %s loaded %d tools", mcpCfg.Name, len(tools))

		case "stdio":
			// stdio 模式：启动本地进程
			if mcpCfg.Command == "" {
				logger.Infof("[Dispatcher] initMCP: %s has empty command, skipping", mcpCfg.Name)
				continue
			}
			client, err := NewMCPStdioClient(mcpCfg.Command, mcpCfg.Args, mcpCfg.Env)
			if err != nil {
				logger.Infof("[Dispatcher] initMCP: failed to create stdio client for %s: %v", mcpCfg.Name, err)
				continue
			}
			// 创建 stdio MCP tool 并添加
			mcpTool := &mcpStdioTool{
				name:   mcpCfg.Name,
				client: client,
			}
			// 根据 risk_level 判断是否需要包装审批
			wrapped := d.wrapToolWithApproval(mcpTool, mcpCfg.Name, "mcp", mcpCfg.RiskLevel)
			d.tools = append(d.tools, wrapped)
			logger.Infof("[Dispatcher] initMCP: %s (stdio) initialized", mcpCfg.Name)

		default:
			logger.Infof("[Dispatcher] initMCP: unknown transport %s for %s, skipping", mcpCfg.Transport, mcpCfg.Name)
		}
	}

	return nil
}

// initSkills 初始化 skill runner
func (d *Dispatcher) initSkills(ctx context.Context) error {
	// 构建 skillsDir
	skillsDir := "skills"
	if d.request.Context != nil {
		if dir, ok := d.request.Context["skills_dir"].(string); ok && dir != "" {
			skillsDir = dir
		}
	}

	// 创建 skill 配置管理器
	configMgr, err := NewSkillConfigManager(DefaultSkillConfigPath())
	if err != nil {
		logger.Infof("[Dispatcher] initSkills: warning - failed to load config: %v", err)
		// 配置加载失败不影响 skill 运行，使用空配置
		configMgr, _ = NewSkillConfigManager("")
	}

	// 创建 skill runner
	d.skillRunner = NewSkillRunner(
		d.request.Skills,
		skillsDir,
		d.request.Sandbox,
		d.defaultModel,
		configMgr,
	)

	// 设置 session ID（从 context 中获取）
	if sessionID, ok := d.request.Context["session_id"].(string); ok && sessionID != "" {
		d.skillRunner.CurrentSessionID = sessionID
		logger.Infof("[Dispatcher] Using session_id: %s for skill execution", sessionID)
	}

	// 创建 skill tool 并添加到 tools
	skillToolBase := d.skillRunner.BuildSkillTool()
	if skillToolBase != nil {
		// 检查是否需要包装审批
		// 构建 skill name -> risk level 的映射
		skillRiskLevels := make(map[string]string)
		for _, s := range d.request.Skills {
			if s.RiskLevel != "" {
				skillRiskLevels[s.ID] = s.RiskLevel
			}
		}

		// 如果有任何 skill 需要审批，则包装 skill tool
		needsApproval := false
		for _, riskLevel := range skillRiskLevels {
			if d.shouldWrapForApproval("skill", riskLevel) {
				needsApproval = true
				break
			}
		}

		if needsApproval {
			// 类型断言获取 InvokableTool
			if invokableTool, ok := skillToolBase.(tool.InvokableTool); ok {
				// 创建动态风险级别获取器
				getter := func(argumentsInJSON string) string {
					// 解析 argumentsInJSON 提取 skill name
					type skillInput struct {
						Name string `json:"name"`
					}
					var input skillInput
					if err := json.Unmarshal([]byte(argumentsInJSON), &input); err == nil {
						if riskLevel, ok := skillRiskLevels[input.Name]; ok {
							return riskLevel
						}
					}
					return "medium" // 默认风险级别
				}
				wrappedSkillTool := NewInvokableApprovableToolWithGetter(invokableTool, "skill", "skill", "medium", getter)
				d.tools = append(d.tools, wrappedSkillTool)
				logger.Infof("[Dispatcher] Skill tool wrapped with dynamic approval (skill count: %d)", len(skillRiskLevels))
			} else {
				d.tools = append(d.tools, skillToolBase)
			}
		} else {
			d.tools = append(d.tools, skillToolBase)
		}
	}

	// 创建 load_skill tool（低风险，通常不需要审批）
	loadSkillTool := d.skillRunner.BuildLoadSkillTool()
	if loadSkillTool != nil {
		d.tools = append(d.tools, loadSkillTool)
	}

	logger.Infof("[Dispatcher] initSkills: %d skills registered, config: %s",
		len(d.request.Skills), DefaultSkillConfigPath())

	// 创建技能规划器 (SkillPlanner)
	d.skillPlanner = NewSkillPlanner(d.request.Skills, d.skillRunner, d.defaultModel)
	logger.Infof("[Dispatcher] SkillPlanner created")

	// 创建技能编排工具 (当需要多 skill 协同时使用)
	skillOrchestratorTool := d.skillRunner.BuildSkillOrchestratorTool(d.skillPlanner)
	if skillOrchestratorTool != nil {
		d.tools = append(d.tools, skillOrchestratorTool)
		logger.Infof("[Dispatcher] SkillOrchestrator tool registered")
	}

	return nil
}
