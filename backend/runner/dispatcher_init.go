package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/jettjia/XiaoQinglong/runner/cliext"
	"github.com/jettjia/XiaoQinglong/runner/cron"
	"github.com/jettjia/XiaoQinglong/runner/pkg/logger"
	"github.com/jettjia/XiaoQinglong/runner/plugins"
	"github.com/jettjia/XiaoQinglong/runner/retriever"
	"github.com/jettjia/XiaoQinglong/runner/subagent"
	"github.com/jettjia/XiaoQinglong/runner/tools"
	"github.com/jettjia/XiaoQinglong/runner/types"
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
			httpTool := plugins.NewHTTPTool(types.ToolConfig{
				Type:        tc.Type,
				Name:        tc.Name,
				Description: tc.Description,
				Endpoint:    tc.Endpoint,
				Method:      tc.Method,
				Headers:     tc.Headers,
				RiskLevel:   tc.RiskLevel,
			})
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
	kbs := make([]retriever.KnowledgeBaseConfig, len(d.request.KnowledgeBases))
	for i, kb := range d.request.KnowledgeBases {
		kbs[i] = retriever.KnowledgeBaseConfig{
			ID:           kb.ID,
			Name:         kb.Name,
			RetrievalURL: kb.RetrievalURL,
			Token:        kb.Token,
			TopK:         kb.TopK,
		}
	}
	retrievalTool := retriever.CreateRetrievalTool(kbs)
	d.tools = append(d.tools, retrievalTool)

	// 保存 knowledgeRetriever 引用，用于自动 RAG
	d.knowledgeRetriever = retriever.NewKnowledgeRetriever(kbs)

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
		a2aCfg := types.A2AAgentConfig{
			Name:      cfg.Name,
			Endpoint:  cfg.Endpoint,
			Headers:   cfg.Headers,
			RiskLevel: cfg.RiskLevel,
		}
		client, err := plugins.NewA2AClient(ctx, a2aCfg)
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
		clients := make(map[string]*plugins.A2AClient)
		for name, runner := range d.a2aRunners {
			_ = runner // runner 已存储
			// 创建 client 引用
			for _, cfg := range d.request.A2A {
				if cfg.Name == name {
					a2aCfg := types.A2AAgentConfig{
						Name:      cfg.Name,
						Endpoint:  cfg.Endpoint,
						Headers:   cfg.Headers,
						RiskLevel: cfg.RiskLevel,
					}
					client, _ := plugins.NewA2AClient(ctx, a2aCfg)
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
			a2aTool := plugins.NewA2AToolWithCounter(clients, &d.a2aCallCount, maxA2ACalls)
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
	logger.Infof("[Dispatcher] initSubAgents called: SubAgents count = %d", len(d.request.SubAgents))
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
		case "http", "sse":
			// HTTP/SSE 模式：通过 HTTP API 或 SSE 端点加载 tools
			if mcpCfg.Endpoint == "" {
				logger.Infof("[Dispatcher] initMCP: %s has empty endpoint, skipping", mcpCfg.Name)
				continue
			}
			loader := plugins.NewMCPToolLoader(mcpCfg.Endpoint, mcpCfg.Headers)
			tools, err := loader.LoadTools(ctx)
			if err != nil {
				logger.Infof("[Dispatcher] initMCP: failed to load tools for %s: %v", mcpCfg.Name, err)
				continue
			}
			for _, t := range tools {
				// HTTP/SSE MCP tools 需要包装审批
				if invokableTool, ok := t.(tool.InvokableTool); ok {
					wrapped := d.wrapToolWithApproval(invokableTool, mcpCfg.Name, "mcp", mcpCfg.RiskLevel)
					d.tools = append(d.tools, wrapped)
				} else {
					d.tools = append(d.tools, t)
				}
			}
			logger.Infof("[Dispatcher] initMCP: %s loaded %d tools (transport=%s)", mcpCfg.Name, len(tools), mcpCfg.Transport)

		case "stdio":
			// stdio 模式：启动本地进程
			if mcpCfg.Command == "" {
				logger.Infof("[Dispatcher] initMCP: %s has empty command, skipping", mcpCfg.Name)
				continue
			}
			client, err := plugins.NewMCPStdioClient(mcpCfg.Command, mcpCfg.Args, mcpCfg.Env)
			if err != nil {
				logger.Infof("[Dispatcher] initMCP: failed to create stdio client for %s: %v", mcpCfg.Name, err)
				continue
			}

			// 先获取工具列表
			tools, err := client.ListTools(ctx)
			if err != nil {
				logger.Infof("[Dispatcher] initMCP: failed to list tools for %s: %v", mcpCfg.Name, err)
				continue
			}

			// 为每个 MCP 工具创建单独的 tool
			for _, t := range tools {
				mcpTool := &mcpStdioTool{
					name:   t.Name,
					client: client,
				}
				wrapped := d.wrapToolWithApproval(mcpTool, t.Name, "mcp", mcpCfg.RiskLevel)
				d.tools = append(d.tools, wrapped)
			}
			logger.Infof("[Dispatcher] initMCP: %s (stdio) initialized with %d tools", mcpCfg.Name, len(tools))

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
	configMgr, err := plugins.NewSkillConfigManager(plugins.DefaultSkillConfigPath())
	if err != nil {
		logger.Infof("[Dispatcher] initSkills: warning - failed to load config: %v", err)
		// 配置加载失败不影响 skill 运行，使用空配置
		configMgr, _ = plugins.NewSkillConfigManager("")
	}

	// 转换 skills 和 sandbox 到 types 包类型
	var skills []types.Skill
	for _, s := range d.request.Skills {
		skills = append(skills, types.Skill{
			ID:          s.ID,
			Name:        s.Name,
			Description: s.Description,
			Instruction: s.Instruction,
			Scope:       s.Scope,
			Trigger:     s.Trigger,
			EntryScript: s.EntryScript,
			FilePath:    s.FilePath,
			Inputs:      s.Inputs,
			Outputs:     s.Outputs,
			RiskLevel:   s.RiskLevel,
		})
	}

	var sandboxCfg *types.SandboxConfig
	if d.request.Sandbox != nil {
		var limits *types.SandboxLimits
		if d.request.Sandbox.Limits != nil {
			limits = &types.SandboxLimits{
				CPU:    d.request.Sandbox.Limits.CPU,
				Memory: d.request.Sandbox.Limits.Memory,
			}
		}
		var volumes []types.VolumeMount
		for _, v := range d.request.Sandbox.Volumes {
			volumes = append(volumes, types.VolumeMount{
				HostPath:      v.HostPath,
				ContainerPath: v.ContainerPath,
				ReadOnly:      v.ReadOnly,
			})
		}
		sandboxCfg = &types.SandboxConfig{
			Enabled:   d.request.Sandbox.Enabled,
			Mode:      d.request.Sandbox.Mode,
			Image:     d.request.Sandbox.Image,
			Workdir:   d.request.Sandbox.Workdir,
			Network:   d.request.Sandbox.Network,
			TimeoutMs: d.request.Sandbox.TimeoutMs,
			Env:       d.request.Sandbox.Env,
			Limits:    limits,
			Volumes:   volumes,
		}
	}

	d.skillRunner = plugins.NewSkillRunner(
		skills,
		skillsDir,
		sandboxCfg,
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
		// 检查 skill tool 是否有可用的 skills（Info 返回 nil 表示没有 skills）
		info, _ := skillToolBase.Info(ctx)
		if info == nil {
			logger.Infof("[Dispatcher] initSkills: no skills available, skipping skill tool registration")
		} else {
			logger.Infof("[Dispatcher] initSkills: registering skill tool with %d skills", len(d.request.Skills))
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
	}

	// 创建 load_skill tool（低风险，通常不需要审批）
	loadSkillTool := d.skillRunner.BuildLoadSkillTool()
	if loadSkillTool != nil {
		// 检查是否有可用的 skills（Info 返回 nil 表示没有 skills）
		info, _ := loadSkillTool.Info(ctx)
		if info != nil {
			d.tools = append(d.tools, loadSkillTool)
		}
	}

	logger.Infof("[Dispatcher] initSkills: %d skills registered, config: %s",
		len(d.request.Skills), plugins.DefaultSkillConfigPath())

	// 创建技能规划器 (SkillPlanner)
	d.skillPlanner = plugins.NewSkillPlanner(skills, d.skillRunner, d.defaultModel)
	logger.Infof("[Dispatcher] SkillPlanner created")

	// 创建技能编排工具 (当需要多 skill 协同时使用)
	skillOrchestratorTool := d.skillRunner.BuildSkillOrchestratorTool(d.skillPlanner)
	if skillOrchestratorTool != nil {
		// 检查是否有可用的 skills（Info 返回 nil 表示没有 skills）
		info, _ := skillOrchestratorTool.Info(ctx)
		if info == nil {
			logger.Infof("[Dispatcher] initSkills: no skills available, skipping orchestrate_skills tool registration")
		} else {
			d.tools = append(d.tools, skillOrchestratorTool)
			logger.Infof("[Dispatcher] SkillOrchestrator tool registered")
		}
	}

	return nil
}

// initLoopCron 初始化 /loop 定时任务工具
func (d *Dispatcher) initLoopCron(ctx context.Context) error {
	// 获取 project dir（从 context 或默认值）
	projectDir := "."
	if d.request.Context != nil {
		if dir, ok := d.request.Context["project_dir"].(string); ok && dir != "" {
			projectDir = dir
		}
	}

	// 创建 cron tools
	cronTools := cron.CreateLoopTools(projectDir)
	for _, t := range cronTools {
		d.tools = append(d.tools, t)
	}

	logger.Infof("[Dispatcher] initLoopCron: %d cron tools registered", len(cronTools))

	// 设置任务触发处理器
	scheduler := cron.GetScheduler()
	scheduler.SetHandler(&cronTaskHandler{
		request: d.request,
	})

	// 启动调度器
	scheduler.Start()
	logger.Infof("[Dispatcher] Cron scheduler started")

	return nil
}

// initBuiltinTools 初始化内置工具（Glob, Grep, FileRead, FileEdit, FileWrite, Bash, WebFetch, WebSearch, Sleep, Task*, Todo*, PlanMode, AskUserQuestion）
func (d *Dispatcher) initBuiltinTools(ctx context.Context) error {
	// 获取工作目录
	workingDir := "."
	if d.request.Context != nil {
		if dir, ok := d.request.Context["project_dir"].(string); ok && dir != "" {
			workingDir = dir
		}
	}

	// 创建内置工具
	builtinTools := []interface {
		Info(ctx context.Context) (*schema.ToolInfo, error)
		InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error)
	}{
		tools.NewGlobTool(workingDir),
		tools.NewGrepTool(workingDir),
		tools.NewFileReadTool(workingDir),
		tools.NewFileEditTool(workingDir),
		tools.NewFileWriteTool(workingDir),
		tools.NewBashTool(workingDir),
		tools.NewSleepTool(),
		tools.NewWebFetchTool(),
		tools.NewWebSearchTool(),
		tools.NewTaskCreateTool(workingDir),
		tools.NewTaskGetTool(workingDir),
		tools.NewTaskListTool(workingDir),
		tools.NewTaskUpdateTool(workingDir),
		tools.NewTodoWriteTool(workingDir),
		tools.NewEnterPlanModeTool(),
		tools.NewExitPlanModeTool(),
		tools.NewAskUserQuestionTool(),
	}

	// 注册工具，使用默认风险级别（低风险工具默认不需要审批）
	for _, t := range builtinTools {
		info, err := t.Info(ctx)
		if err != nil {
			logger.Warnf("[Dispatcher] initBuiltinTools: failed to get tool info: %v", err)
			continue
		}

		// 包装工具并添加到列表
		wrapped := d.wrapToolWithApproval(t, info.Name, "builtin", "low")
		d.tools = append(d.tools, wrapped)
	}

	logger.Infof("[Dispatcher] initBuiltinTools: registered %d builtin tools", len(builtinTools))
	return nil
}

// initCLIs 初始化 CLI 扩展工具（如飞书 CLI）
func (d *Dispatcher) initCLIs(ctx context.Context) error {
	if len(d.request.CLIs) == 0 {
		logger.Infof("[Dispatcher] initCLIs: no CLI configs provided")
		return nil
	}

	// 确定 CLI 配置目录
	cliBaseDir := "/var/run/cliext"
	if d.request.Context != nil {
		if dir, ok := d.request.Context["cli_config_dir"].(string); ok && dir != "" {
			cliBaseDir = dir
		}
	}

	// 创建 CLI 扩展管理器
	ext := cliext.NewCLIExtension(cliBaseDir)

	// 转换并注册每个 CLI
	for _, cliReq := range d.request.CLIs {
		cfg := cliext.CLIConfig{
			Name:      cliReq.Name,
			Command:   cliReq.Command,
			ConfigDir: cliReq.ConfigDir,
			SkillsDir: cliReq.SkillsDir,
			RiskLevel: cliReq.RiskLevel,
			AuthType:  cliReq.AuthType,
		}

		if err := ext.Register(cfg); err != nil {
			logger.Infof("[Dispatcher] initCLIs: failed to register %s: %v", cliReq.Name, err)
			continue
		}

		// 创建 CLI 工具
		cliTool := cliext.NewCLITool(ext, cliReq.Name)

		// 根据 risk_level 判断是否需要包装审批
		riskLevel := cliReq.RiskLevel
		if riskLevel == "" {
			riskLevel = "medium"
		}

		wrapped := d.wrapToolWithApproval(cliTool, "cli_"+cliReq.Name, "cli", riskLevel)
		d.tools = append(d.tools, wrapped)
		logger.Infof("[Dispatcher] initCLIs: registered CLI tool: %s", cliReq.Name)
	}

	// 添加 auth 工具
	authTool := cliext.NewCLIAuthTool(ext)
	d.tools = append(d.tools, authTool)
	logger.Infof("[Dispatcher] initCLIs: registered cli_auth tool")

	// 保存 CLI 扩展引用
	d.cliExt = ext

	logger.Infof("[Dispatcher] initCLIs: %d CLI tools registered", len(d.request.CLIs))
	return nil
}

// cronTaskHandler 处理定时任务触发
type cronTaskHandler struct {
	request *types.RunRequest
}

// OnTaskFired 当任务触发时执行
func (h *cronTaskHandler) OnTaskFired(taskID string, prompt string) {
	logger.Infof("[CronTaskHandler] Task %s fired with prompt: %s", taskID, prompt)

	// 启动新 goroutine 执行定时任务
	go func() {
		// 创建新的 RunRequest，复制原始配置但使用新的 prompt
		req := &types.RunRequest{
			Prompt:         prompt,
			Models:         h.request.Models,
			Messages:       []types.Message{}, // 定时任务从新 prompt 开始
			Context:        h.request.Context,
			KnowledgeBases: h.request.KnowledgeBases,
			Skills:         h.request.Skills,
			MCPs:           h.request.MCPs,
			A2A:            h.request.A2A,
			Tools:          h.request.Tools,
			InternalAgents: h.request.InternalAgents,
			SubAgents:      h.request.SubAgents,
			Options:        h.request.Options,
			Sandbox:        h.request.Sandbox,
			Files:          h.request.Files,
		}

		runner := NewRunner(req)
		result, err := runner.Run(context.Background())
		if err != nil {
			logger.Errorf("[CronTaskHandler] Task %s execution failed: %v", taskID, err)
			return
		}

		logger.Infof("[CronTaskHandler] Task %s completed, finish_reason: %s", taskID, result.FinishReason)
	}()
}

// OnTaskError 当任务执行出错时记录日志
func (h *cronTaskHandler) OnTaskError(taskID string, err error) {
	logger.Errorf("[CronTaskHandler] Task %s error: %v", taskID, err)
}
