package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/deep"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/jettjia/XiaoQinglong/runner/cliext"
	"github.com/jettjia/XiaoQinglong/runner/cron"
	"github.com/jettjia/XiaoQinglong/runner/llm"
	_ "github.com/jettjia/XiaoQinglong/runner/llm/adapters"
	"github.com/jettjia/XiaoQinglong/runner/memory"
	"github.com/jettjia/XiaoQinglong/runner/pkg/logger"
	"github.com/jettjia/XiaoQinglong/runner/pkg/xqldir"
	"github.com/jettjia/XiaoQinglong/runner/plugins"
	"github.com/jettjia/XiaoQinglong/runner/retriever"
	"github.com/jettjia/XiaoQinglong/runner/subagent"
	"github.com/jettjia/XiaoQinglong/runner/tools"
	"github.com/jettjia/XiaoQinglong/runner/types"
)

func (d *Dispatcher) initModels(ctx context.Context) error {
	logger.Infof("[Dispatcher] initModels: starting")
	d.models = make(map[string]model.ToolCallingChatModel)
	d.modelsByRole = make(map[ModelRole]model.ToolCallingChatModel)

	for key, cfg := range d.request.Models {
		// 确定 provider，默认 openai
		provider := cfg.Provider
		if provider == "" {
			provider = "openai"
			logger.Warnf("[Dispatcher] initModels: model %s has no provider, defaulting to openai", key)
		}

		// 获取 factory
		factory, err := llm.GetFactory(provider)
		if err != nil {
			return fmt.Errorf("get model factory for provider %s failed: %w", provider, err)
		}

		// 转换为 llm.ModelConfig
		llmCfg := &llm.ModelConfig{
			Name:        cfg.Name,
			APIKey:      cfg.APIKey,
			APIBase:     cfg.APIBase,
			Temperature: cfg.Temperature,
			MaxTokens:   cfg.MaxTokens,
			TopP:        cfg.TopP,
			ExtraFields: cfg.ExtraFields,
		}

		logger.Infof("[Dispatcher] initModels: key=%s, provider=%s, model=%s, baseURL=%s",
			key, provider, cfg.Name, cfg.APIBase)

		cm, err := factory.CreateChatModel(ctx, llmCfg)
		if err != nil {
			return fmt.Errorf("create model %s (provider=%s) failed: %w", key, provider, err)
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

// initMemStore 初始化记忆存储（加载冻结快照）
func (d *Dispatcher) initMemStore(ctx context.Context) {
	if d.memStore == nil {
		d.memStore = memory.NewMemStore()
	}

	// 从 context 中获取 session_id、user_id、agent_id
	sessionID := ""
	userID := ""
	agentID := ""

	if v, ok := d.request.Context["session_id"].(string); ok {
		sessionID = v
	}
	if v, ok := d.request.Context["user_id"].(string); ok {
		userID = v
	}
	if v, ok := d.request.Context["agent_id"].(string); ok {
		agentID = v
	}

	// 初始化各层级记忆
	if err := d.memStore.InitializeAll(ctx, sessionID, userID, agentID); err != nil {
		logger.Warnf("[Dispatcher] initMemStore failed: %v", err)
	}
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
			wrapped := WrapToolWithApproval(httpTool, tc.Name, "http", tc.RiskLevel, d.request.Options.ApprovalPolicy)
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
			wrappedA2ATool := WrapToolWithApproval(a2aTool, "a2a", "a2a", a2aRiskLevel, d.request.Options.ApprovalPolicy)
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

	// 如果有 sub_agents，初始化 deep agent（opt-in 模式）
	if len(d.request.SubAgents) > 0 {
		if err := d.initDeepAgent(ctx); err != nil {
			logger.Infof("[Dispatcher] initSubAgents: warning - initDeepAgent failed: %v", err)
			// 不失败，保留原有的 spawn/collect/delegate 工具方式
		}
	}

	return nil
}

// initDeepAgent 初始化 deep agent（仅当配置了 sub_agents 时启用）
func (d *Dispatcher) initDeepAgent(ctx context.Context) error {
	logger.Infof("[Dispatcher] initDeepAgent: starting, sub_agents count = %d", len(d.request.SubAgents))

	// 创建 SubAgentAdapter 列表
	var subAgents []adk.Agent
	for _, cfg := range d.request.SubAgents {
		// 创建内部的 adk.Agent
		innerAgent, err := d.createSubAgent(ctx, &cfg)
		if err != nil {
			return fmt.Errorf("createSubAgent %s failed: %w", cfg.Name, err)
		}
		adapter := subagent.NewSubAgentAdapter(&cfg, innerAgent)
		subAgents = append(subAgents, adapter)
		logger.Infof("[Dispatcher] initDeepAgent: created adapter for sub-agent %s", cfg.Name)
	}

	// 构建 deep agent 的 tools（Coordinator 自己的工具）
	// 注意：deep agent 不需要 spawn/collect/delegate 工具，这些是给普通 main_agent 用的
	var coordinatorTools []tool.BaseTool
	// 其他工具已经添加到 d.tools，这里只过滤掉 sub-agent 相关的工具
	for _, t := range d.tools {
		info, _ := t.Info(ctx)
		if info == nil {
			continue
		}
		// 排除 sub-agent 管理工具
		switch info.Name {
		case "spawn_task", "collect_task", "list_tasks", "cancel_task", "delegate_to_agent":
			continue
		}
		coordinatorTools = append(coordinatorTools, t)
	}

	// 计算 max iterations
	maxIterations := 10
	if d.request.Options != nil && d.request.Options.MaxIterations > 0 {
		maxIterations = d.request.Options.MaxIterations
	}

	// 构建 Instruction
	instruction := d.buildSystemPrompt()

	// 创建 deep agent
	deepCfg := &deep.Config{
		Name:        "deep_coordinator",
		Description: "Deep agent coordinator with sub-agent support",
		ChatModel:   d.defaultModel,
		SubAgents:   subAgents,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: coordinatorTools,
			},
		},
		MaxIteration: maxIterations,
		Instruction:  instruction,
	}

	var err error
	d.deepAgent, err = deep.New(ctx, deepCfg)
	if err != nil {
		return fmt.Errorf("deep.New failed: %w", err)
	}

	logger.Infof("[Dispatcher] initDeepAgent: deep agent created with %d sub-agents", len(subAgents))
	return nil
}

// createSubAgent 为 deep agent 创建 sub-agent
func (d *Dispatcher) createSubAgent(ctx context.Context, cfg *subagent.SubAgentConfig) (adk.Agent, error) {
	// 确定使用的模型
	model := d.defaultModel
	if cfg.Model != nil && cfg.Model.Name != "" {
		if cm, ok := d.models[cfg.Model.Name]; ok {
			model = cm
		} else if cm, ok := d.modelsByRole[ModelRole(cfg.Model.Name)]; ok {
			model = cm
		}
		// 如果都没找到，继续使用 defaultModel
	}

	// 收集该 sub-agent 的工具（根据配置的工具名称）
	var tools []tool.BaseTool
	for _, toolName := range cfg.Tools {
		for _, t := range d.tools {
			info, _ := t.Info(ctx)
			if info != nil && info.Name == toolName {
				tools = append(tools, t)
				break
			}
		}
	}

	// 构建 instruction
	instruction := cfg.Prompt
	if cfg.Description != "" {
		instruction = cfg.Description + "\n\n" + instruction
	}

	// 创建 agent
	agentCfg := &adk.ChatModelAgentConfig{
		Name:          cfg.Name,
		Description:   cfg.Description,
		Instruction:   instruction,
		Model:         model,
		MaxIterations: cfg.MaxIterations,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: tools,
			},
		},
	}

	agent, err := adk.NewChatModelAgent(ctx, agentCfg)
	if err != nil {
		return nil, fmt.Errorf("adk.NewChatModelAgent failed: %w", err)
	}

	return agent, nil
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
					wrapped := WrapToolWithApproval(invokableTool, mcpCfg.Name, "mcp", mcpCfg.RiskLevel, d.request.Options.ApprovalPolicy)
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
				wrapped := WrapToolWithApproval(mcpTool, t.Name, "mcp", mcpCfg.RiskLevel, d.request.Options.ApprovalPolicy)
				d.tools = append(d.tools, wrapped)
			}
			logger.Infof("[Dispatcher] initMCP: %s (stdio) initialized with %d tools", mcpCfg.Name, len(tools))

		default:
			logger.Infof("[Dispatcher] initMCP: unknown transport %s for %s, skipping", mcpCfg.Transport, mcpCfg.Name)
		}
	}

	return nil
}

// initSkills 初始化 skill middleware
// 使用 eino 官方 skill middleware，从 skillsDir 加载 SKILL.md
func (d *Dispatcher) initSkills(ctx context.Context) error {
	// 构建 skillsDir
	// 优先级: Context.skills_dir > 环境变量 SKILLS_DIR > xqldir.GetSkillsDir()
	skillsDir := xqldir.GetSkillsDir()
	if d.request.Context != nil {
		if dir, ok := d.request.Context["skills_dir"].(string); ok && dir != "" {
			skillsDir = dir
		}
	}
	// 如果环境变量指定，覆盖默认
	if envDir := os.Getenv("SKILLS_DIR"); envDir != "" {
		skillsDir = envDir
	}
	// 如果是相对路径，转换为绝对路径
	if !filepath.IsAbs(skillsDir) {
		// 相对于 base dir 解析
		skillsDir = filepath.Join(xqldir.GetBaseDir(), skillsDir)
	}
	logger.Infof("[Dispatcher] initSkills: using skills_dir: %s", skillsDir)

	// 创建 eino skill middleware（从 skillsDir 加载 SKILL.md）
	// 这让 agent 可以通过 skill("xxx") 获取 skill 的完整内容
	skillMw, err := plugins.NewSkillMiddleware(ctx, skillsDir)
	if err != nil {
		logger.Infof("[Dispatcher] initSkills: warning - failed to create skill middleware: %v", err)
	} else {
		d.skillMiddleware = skillMw
		logger.Infof("[Dispatcher] initSkills: eino skill middleware created for dir: %s", skillsDir)
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

	// 获取临时目录用于存储过大的工具结果
	tempDir := os.TempDir()

	// 从注册中心获取所有已注册的工具
	registeredTools := tools.GlobalRegistry.List()
	logger.Infof("[Dispatcher] initBuiltinTools: found %d registered tools in registry", len(registeredTools))

	// 注册工具，使用注册中心中的默认风险级别
	for _, toolName := range registeredTools {
		// 从注册中心创建工具实例
		t, err := tools.GlobalRegistry.CreateTool(toolName, workingDir)
		if err != nil {
			logger.Warnf("[Dispatcher] initBuiltinTools: failed to create tool %s: %v", toolName, err)
			continue
		}

		info, err := t.Info(ctx)
		if err != nil {
			logger.Warnf("[Dispatcher] initBuiltinTools: failed to get tool info for %s: %v", toolName, err)
			continue
		}

		// 从注册中心获取默认风险级别
		riskLevel := tools.GlobalRegistry.GetDefaultRisk(toolName)

		// 从注册中心获取该工具的最大结果限制
		maxChars := tools.GlobalRegistry.GetMaxResultChars(toolName)
		if maxChars > 0 {
			// 创建结果限制器
			limiter := tools.NewResultLimiter(tempDir, maxChars)
			// 包装工具以限制结果大小
			t = tools.WrapToolWithLimiter(t, limiter)
			logger.Infof("[Dispatcher] initBuiltinTools: tool %s has result limit %d chars", toolName, maxChars)
		}

		// 包装工具并添加到列表
		wrapped := WrapToolWithApproval(t, info.Name, "builtin", riskLevel, d.request.Options.ApprovalPolicy)
		d.tools = append(d.tools, wrapped)
	}

	logger.Infof("[Dispatcher] initBuiltinTools: registered %d builtin tools from registry", len(registeredTools))
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

		wrapped := WrapToolWithApproval(cliTool, "cli_"+cliReq.Name, "cli", riskLevel, d.request.Options.ApprovalPolicy)
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
