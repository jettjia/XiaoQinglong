package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino-ext/components/model/openai"
)

// ========== Dispatch Result ==========

type DispatchResult struct {
	Content          string
	ToolCalls        []ToolCall
	A2AResults       []A2AResult
	FinishReason     string
	A2UIMessages     []json.RawMessage
	TokensUsed       int
	Metadata         *ResultMetadata
	PendingApprovals []PendingApproval
	CheckPointID     string
}

type ResultMetadata struct {
	Model           string
	LatencyMs       int64
	TotalLatencyMs  int64
	PromptTokens    int
	CompletionTokens int
	ToolCallsCount  int
	A2ACallsCount   int
	SkillCallsCount int
	Iterations      int
	ToolCallsDetail []ToolCallMetadata
	Error           string
}

type ToolCallMetadata struct {
	Tool      string
	Input     any
	Output    any
	LatencyMs int64
	Success   bool
	Error     string
}

// ========== Dispatcher ==========

type Dispatcher struct {
	request *RunRequest

	// 组件
	defaultModel     model.ToolCallingChatModel
	defaultModelName string
	models          map[string]model.ToolCallingChatModel
	modelsByRole    map[ModelRole]model.ToolCallingChatModel
	tools          []tool.BaseTool
	toolConfigs    map[string]ToolConfig // tool name -> config for interrupt handling
	a2aRunners     map[string]*adk.Runner
	internalAgents map[string]adk.Agent
	skillRunner    *SkillRunner
	skillPlanner   *SkillPlanner // LLM 驱动的技能规划器
	a2aCallCount   int
}

func NewDispatcher(req *RunRequest) *Dispatcher {
	return &Dispatcher{
		request:     req,
		toolConfigs: make(map[string]ToolConfig),
	}
}

func (d *Dispatcher) Run(ctx context.Context) (*DispatchResult, error) {
	// 1. 初始化模型
	if err := d.initModels(ctx); err != nil {
		return nil, fmt.Errorf("init models failed: %w", err)
	}

	// 2. 初始化工具 (HTTP tools)
	if err := d.initTools(ctx); err != nil {
		return nil, fmt.Errorf("init tools failed: %w", err)
	}

	// 3. 初始化 A2A agents
	if err := d.initA2A(ctx); err != nil {
		return nil, fmt.Errorf("init a2a failed: %w", err)
	}

	// 4. 初始化内部 agents
	if err := d.initInternalAgents(ctx); err != nil {
		return nil, fmt.Errorf("init internal agents failed: %w", err)
	}

	// 5. 初始化 MCP
	if err := d.initMCPs(ctx); err != nil {
		log.Printf("[Dispatcher] Warning: init mcps failed: %v", err)
	}

	// 6. 初始化 skill runner
	if err := d.initSkills(ctx); err != nil {
		return nil, fmt.Errorf("init skills failed: %w", err)
	}

	// 6. 构建系统 prompt
	systemPrompt := d.buildSystemPrompt()

	// 7. 构建消息（如果配置了rewrite模型，则对用户query进行改写）
	messages, err := d.buildMessagesWithRewrite(ctx, systemPrompt)
	if err != nil {
		return nil, fmt.Errorf("build messages failed: %w", err)
	}

	// 8. 创建 Agent 并运行
	return d.runAgent(ctx, messages)
}

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

// shouldWrapForApproval 判断工具是否需要包装审批
func (d *Dispatcher) shouldWrapForApproval(toolName, riskLevel string) bool {
	if d.request.Options == nil || d.request.Options.ApprovalPolicy == nil {
		return false
	}

	policy := d.request.Options.ApprovalPolicy
	if !policy.Enabled {
		return false
	}

	// 检查是否在白名单中
	for _, name := range policy.AutoApprove {
		if name == toolName {
			return false
		}
	}

	// 检查 risk_level 是否达到阈值
	return ShouldApprove(riskLevel, policy.RiskThreshold)
}

// wrapToolWithApproval 如果需要审批，包装工具
func (d *Dispatcher) wrapToolWithApproval(t tool.InvokableTool, toolName, toolType, riskLevel string) tool.BaseTool {
	if !d.shouldWrapForApproval(toolName, riskLevel) {
		return t
	}

	log.Printf("[Dispatcher] Wrapping tool %s (%s) with approval, risk_level=%s", toolName, toolType, riskLevel)
	return NewInvokableApprovableTool(t, toolName, toolType, riskLevel)
}

// buildToolRiskLevels 构建工具名称到风险级别的映射
func (d *Dispatcher) buildToolRiskLevels() map[string]string {
	riskLevels := make(map[string]string)

	log.Printf("[Dispatcher] buildToolRiskLevels: d.request.Tools has %d tools", len(d.request.Tools))
	for _, tc := range d.request.Tools {
		log.Printf("[Dispatcher]   tool: name=%s, type=%s, risk_level=%s", tc.Name, tc.Type, tc.RiskLevel)
		riskLevels[tc.Name] = tc.RiskLevel
	}

	log.Printf("[Dispatcher] buildToolRiskLevels: d.request.A2A has %d agents", len(d.request.A2A))
	// A2A agents
	for _, cfg := range d.request.A2A {
		log.Printf("[Dispatcher]   a2a: name=%s, risk_level=%s", cfg.Name, cfg.RiskLevel)
		riskLevels["a2a"] = cfg.RiskLevel
	}

	log.Printf("[Dispatcher] buildToolRiskLevels: d.request.MCPs has %d configs", len(d.request.MCPs))
	// MCP tools (using the mcp name)
	for _, cfg := range d.request.MCPs {
		log.Printf("[Dispatcher]   mcp: name=%s, risk_level=%s", cfg.Name, cfg.RiskLevel)
		riskLevels[cfg.Name] = cfg.RiskLevel
	}

	// A2A agents
	for _, cfg := range d.request.A2A {
		riskLevels["a2a"] = cfg.RiskLevel
	}

	// MCP tools (using the mcp name)
	for _, cfg := range d.request.MCPs {
		riskLevels[cfg.Name] = cfg.RiskLevel
	}

	return riskLevels
}

// buildApprovalToolMiddleware 构建审批中间件
func (d *Dispatcher) buildApprovalToolMiddleware() compose.InvokableToolMiddleware {
	// 设置审批阈值
	if d.request.Options != nil && d.request.Options.ApprovalPolicy != nil {
		SetApprovalThreshold(d.request.Options.ApprovalPolicy.RiskThreshold)
	}

	riskLevels := d.buildToolRiskLevels()
	log.Printf("[Dispatcher] Building approval middleware with %d tools, threshold=%s", len(riskLevels), approvalThreshold)

	return newApprovalToolMiddleware(riskLevels).Wrap
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

func (d *Dispatcher) initA2A(ctx context.Context) error {
	d.a2aRunners = make(map[string]*adk.Runner)

	for _, cfg := range d.request.A2A {
		client, err := NewA2AClient(ctx, cfg)
		if err != nil {
			log.Printf("[Dispatcher] initA2A: failed to create client for %s: %v", cfg.Name, err)
			continue
		}

		runner, err := client.CreateA2ARunner(ctx, d.defaultModel)
		if err != nil {
			log.Printf("[Dispatcher] initA2A: failed to create runner for %s: %v", cfg.Name, err)
			continue
		}

		d.a2aRunners[cfg.Name] = runner
		log.Printf("[Dispatcher] initA2A: registered agent %s", cfg.Name)
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
					log.Printf("[Dispatcher] A2A trace context set: %v", traceCtx)
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

	log.Printf("[Dispatcher] initA2A: %d agents initialized", len(d.a2aRunners))
	return nil
}

func (d *Dispatcher) initInternalAgents(ctx context.Context) error {
	d.internalAgents = make(map[string]adk.Agent)

	for _, cfg := range d.request.InternalAgents {
		agent, err := d.createInternalAgent(ctx, cfg)
		if err != nil {
			log.Printf("[Dispatcher] initInternalAgents: failed to create agent %s: %v", cfg.Name, err)
			continue
		}

		d.internalAgents[cfg.ID] = agent
		log.Printf("[Dispatcher] initInternalAgents: registered agent %s (%s)", cfg.Name, cfg.ID)
	}

	log.Printf("[Dispatcher] initInternalAgents: %d agents initialized", len(d.internalAgents))
	return nil
}

// initMCPs 初始化 MCP 工具
func (d *Dispatcher) initMCPs(ctx context.Context) error {
	if len(d.request.MCPs) == 0 {
		return nil
	}

	for _, mcpCfg := range d.request.MCPs {
		log.Printf("[Dispatcher] initMCP: name=%s, transport=%s", mcpCfg.Name, mcpCfg.Transport)

		switch mcpCfg.Transport {
		case "http":
			// HTTP 模式：通过 HTTP API 加载 tools
			if mcpCfg.Endpoint == "" {
				log.Printf("[Dispatcher] initMCP: %s has empty endpoint, skipping", mcpCfg.Name)
				continue
			}
			loader := NewMCPToolLoader(mcpCfg.Endpoint, mcpCfg.Headers)
			tools, err := loader.LoadTools(ctx)
			if err != nil {
				log.Printf("[Dispatcher] initMCP: failed to load tools for %s: %v", mcpCfg.Name, err)
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
			log.Printf("[Dispatcher] initMCP: %s loaded %d tools", mcpCfg.Name, len(tools))

		case "stdio":
			// stdio 模式：启动本地进程
			if mcpCfg.Command == "" {
				log.Printf("[Dispatcher] initMCP: %s has empty command, skipping", mcpCfg.Name)
				continue
			}
			client, err := NewMCPStdioClient(mcpCfg.Command, mcpCfg.Args, mcpCfg.Env)
			if err != nil {
				log.Printf("[Dispatcher] initMCP: failed to create stdio client for %s: %v", mcpCfg.Name, err)
				continue
			}
			// 创建 stdio MCP tool 并添加
			mcpTool := &mcpStdioTool{
				name:    mcpCfg.Name,
				client:  client,
			}
			// 根据 risk_level 判断是否需要包装审批
			wrapped := d.wrapToolWithApproval(mcpTool, mcpCfg.Name, "mcp", mcpCfg.RiskLevel)
			d.tools = append(d.tools, wrapped)
			log.Printf("[Dispatcher] initMCP: %s (stdio) initialized", mcpCfg.Name)

		default:
			log.Printf("[Dispatcher] initMCP: unknown transport %s for %s, skipping", mcpCfg.Transport, mcpCfg.Name)
		}
	}

	return nil
}

// mcpStdioTool stdio 模式的 MCP tool
type mcpStdioTool struct {
	name   string
	client *MCPStdioClient
}

func (t *mcpStdioTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        t.name,
		Desc:        fmt.Sprintf("MCP tool via stdio: %s", t.name),
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"args": {Type: schema.Object, Desc: "Tool arguments", Required: false},
		}),
	}, nil
}

func (t *mcpStdioTool) InvokableRun(ctx context.Context, argumentsInJSON string, opt ...tool.Option) (string, error) {
	var args map[string]any
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	name, _ := args["name"].(string)
	delete(args, "name")

	return t.client.CallTool(ctx, name, args)
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
		log.Printf("[Dispatcher] initSkills: warning - failed to load config: %v", err)
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
		log.Printf("[Dispatcher] Using session_id: %s for skill execution", sessionID)
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
				log.Printf("[Dispatcher] Skill tool wrapped with dynamic approval (skill count: %d)", len(skillRiskLevels))
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

	log.Printf("[Dispatcher] initSkills: %d skills registered, config: %s",
		len(d.request.Skills), DefaultSkillConfigPath())

	// 创建技能规划器 (SkillPlanner)
	d.skillPlanner = NewSkillPlanner(d.request.Skills, d.skillRunner, d.defaultModel)
	log.Printf("[Dispatcher] SkillPlanner created")

	// 创建技能编排工具 (当需要多 skill 协同时使用)
	skillOrchestratorTool := d.skillRunner.BuildSkillOrchestratorTool(d.skillPlanner)
	if skillOrchestratorTool != nil {
		d.tools = append(d.tools, skillOrchestratorTool)
		log.Printf("[Dispatcher] SkillOrchestrator tool registered")
	}

	return nil
}

// createInternalAgent 创建内部 agent
func (d *Dispatcher) createInternalAgent(ctx context.Context, cfg InternalAgentConfig) (adk.Agent, error) {
	var chatModel model.ToolCallingChatModel

	// 使用指定的模型配置
	if cfg.Model.Name != "" {
		if cm, ok := d.models[cfg.Model.Name]; ok {
			chatModel = cm
		} else {
			// 创建新的模型实例
			openaiCfg := &openai.ChatModelConfig{
				APIKey:  cfg.Model.APIKey,
				Model:   cfg.Model.Name,
				BaseURL: cfg.Model.APIBase,
			}
			cm, err := openai.NewChatModel(ctx, openaiCfg)
			if err != nil {
				return nil, fmt.Errorf("create model failed: %w", err)
			}
			chatModel = cm
		}
	} else if d.defaultModel != nil {
		chatModel = d.defaultModel
	} else {
		return nil, fmt.Errorf("no model available for internal agent %s", cfg.Name)
	}

	prompt := cfg.Prompt
	if prompt == "" {
		prompt = fmt.Sprintf("你是一个内部 agent，名称为 %s。", cfg.Name)
	}

	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        cfg.Name,
		Description: fmt.Sprintf("内部 Agent: %s", cfg.Name),
		Instruction: prompt,
		Model:       chatModel,
	})
	if err != nil {
		return nil, fmt.Errorf("create agent failed: %w", err)
	}

	return agent, nil
}

func (d *Dispatcher) buildSystemPrompt() string {
	prompt := d.request.Prompt

	// 添加知识库上下文
	if len(d.request.Knowledge) > 0 {
		prompt += "\n\n## 知识库信息\n"
		for _, kb := range d.request.Knowledge {
			prompt += fmt.Sprintf("\n### %s (相关性: %.2f)\n%s\n", kb.Name, kb.Score, kb.Content)
			if kb.Metadata != nil {
				for k, v := range kb.Metadata {
					prompt += fmt.Sprintf("%s: %v\n", k, v)
				}
			}
		}
	}

	// 添加 skills 上下文
	if len(d.request.Skills) > 0 {
		prompt += "\n\n## 可用技能\n"
		for _, skill := range d.request.Skills {
			prompt += fmt.Sprintf("\n### %s\n%s\n", skill.Name, skill.Description)
			prompt += fmt.Sprintf("指令: %s\n", skill.Instruction)
		}
	}

	// 添加 context 变量
	if len(d.request.Context) > 0 {
		prompt += "\n\n## 上下文信息\n"
		for k, v := range d.request.Context {
			prompt += fmt.Sprintf("- %s: %v\n", k, v)
		}
	}

	// 添加 A2A agents 信息
	if len(d.request.A2A) > 0 {
		prompt += "\n\n## 可用外部 Agent\n"
		for _, a2a := range d.request.A2A {
			prompt += fmt.Sprintf("- %s: %s\n", a2a.Name, a2a.Endpoint)
		}
	}

	// 添加内部 agents 信息
	if len(d.request.InternalAgents) > 0 {
		prompt += "\n\n## 可用内部 Agent\n"
		for _, ia := range d.request.InternalAgents {
			prompt += fmt.Sprintf("- %s (%s): %s\n", ia.Name, ia.ID, ia.Prompt)
		}
	}

	return prompt
}

// buildMessagesWithRewrite builds messages and optionally rewrites the last user query
func (d *Dispatcher) buildMessagesWithRewrite(ctx context.Context, systemPrompt string) ([]adk.Message, error) {
	messages := d.buildMessages(systemPrompt)

	// 检查是否需要rewrite：如果有rewrite模型且最后一条是user message
	routingCfg := d.getRoutingConfig()
	if routingCfg != nil && d.modelsByRole[ModelRoleRewrite] != nil {
		// 找到最后一条user message
		for i := len(messages) - 1; i >= 0; i-- {
			if messages[i].Role == "user" {
				// 调用rewrite模型改写query
				rewritten, err := d.rewriteQuery(ctx, messages[i].Content)
				if err != nil {
					log.Printf("[Dispatcher] rewriteQuery failed: %v, using original", err)
					break
				}
				log.Printf("[Dispatcher] query rewritten: %s -> %s", messages[i].Content, rewritten)
				messages[i].Content = rewritten
				break
			}
			// 遇到其他类型的message就停止查找
			if messages[i].Role == "system" || messages[i].Role == "assistant" {
				break
			}
		}
	}

	return messages, nil
}

func (d *Dispatcher) buildMessages(systemPrompt string) []adk.Message {
	var messages []adk.Message

	// 添加 system message
	if d.request.Prompt != "" {
		messages = append(messages, schema.SystemMessage(systemPrompt))
	}

	// 添加对话历史
	for _, msg := range d.request.Messages {
		switch msg.Role {
		case "user":
			messages = append(messages, schema.UserMessage(msg.Content))
		case "assistant":
			messages = append(messages, schema.AssistantMessage(msg.Content, nil))
		case "system":
			messages = append(messages, schema.SystemMessage(msg.Content))
		}
	}

	return messages
}

// getModel returns the model for the given role, falls back to default if not found
func (d *Dispatcher) getModel(role ModelRole) model.ToolCallingChatModel {
	if m, ok := d.modelsByRole[role]; ok {
		return m
	}
	return d.defaultModel
}

// rewriteQuery uses the rewrite model to enhance/improve the user query
func (d *Dispatcher) rewriteQuery(ctx context.Context, query string) (string, error) {
	rewriteModel := d.getModel(ModelRoleRewrite)
	if rewriteModel == nil {
		return query, nil // no rewrite model, return original
	}

	routingCfg := d.getRoutingConfig()
	prompt := routingCfg.RewritePrompt
	if prompt == "" {
		prompt = "请优化以下用户Query，使其更加清晰、准确，便于理解和执行。只返回优化后的Query，不要其他内容。"
	}

	messages := []adk.Message{
		schema.SystemMessage(prompt),
		schema.UserMessage(query),
	}

	resp, err := rewriteModel.Generate(ctx, messages)
	if err != nil {
		return query, fmt.Errorf("rewrite failed: %w", err)
	}

	return resp.Content, nil
}

// summarizeContent uses the summarize model to summarize content
func (d *Dispatcher) summarizeContent(ctx context.Context, content string) (string, error) {
	summarizeModel := d.getModel(ModelRoleSummarize)
	if summarizeModel == nil {
		return content, nil // no summarize model, return original
	}

	routingCfg := d.getRoutingConfig()
	prompt := routingCfg.SummarizePrompt
	if prompt == "" {
		prompt = "请总结以下内容，提取关键信息，保持简洁。只返回总结内容，不要其他内容。"
	}

	messages := []adk.Message{
		schema.SystemMessage(prompt),
		schema.UserMessage(content),
	}

	resp, err := summarizeModel.Generate(ctx, messages)
	if err != nil {
		return content, fmt.Errorf("summarize failed: %w", err)
	}

	return resp.Content, nil
}

// getRoutingConfig returns the routing configuration
func (d *Dispatcher) getRoutingConfig() *RoutingConfig {
	if d.request.Options != nil && d.request.Options.Routing != nil {
		return d.request.Options.Routing
	}
	return nil
}

// buildModelRetryConfig converts RetryConfig to adk.ModelRetryConfig
func (d *Dispatcher) buildModelRetryConfig() *adk.ModelRetryConfig {
	if d.request.Options == nil || d.request.Options.Retry == nil {
		return nil
	}
	retry := d.request.Options.Retry
	if retry.MaxAttempts <= 0 {
		return nil
	}

	return &adk.ModelRetryConfig{
		MaxRetries: retry.MaxAttempts,
		IsRetryAble: func(ctx context.Context, err error) bool {
			if len(retry.RetryableErrors) == 0 {
				return true // 默认全部可重试
			}
			errStr := err.Error()
			for _, e := range retry.RetryableErrors {
				if strings.Contains(errStr, e) {
					return true
				}
			}
			return false
		},
		BackoffFunc: func(ctx context.Context, attempt int) time.Duration {
			delay := float64(retry.InitialDelayMs) * math.Pow(retry.BackoffMultiplier, float64(attempt-1))
			if delay > float64(retry.MaxDelayMs) {
				delay = float64(retry.MaxDelayMs)
			}
			return time.Duration(delay) * time.Millisecond
		},
	}
}

func (d *Dispatcher) runAgent(ctx context.Context, messages []adk.Message) (*DispatchResult, error) {
	startTime := time.Now()

	// 计算最大迭代次数
	maxIterations := 10
	if d.request.Options != nil && d.request.Options.MaxIterations > 0 {
		maxIterations = d.request.Options.MaxIterations
	}

	// MaxTotalTokens 限制
	maxTotalTokens := 0
	if d.request.Options != nil && d.request.Options.MaxTotalTokens > 0 {
		maxTotalTokens = d.request.Options.MaxTotalTokens
	}

	// Token 使用量追踪
	var totalPromptTokens int
	var totalCompletionTokens int

	// 构建 Agent 配置
	agentConfig := &adk.ChatModelAgentConfig{
		Name:           "main_agent",
		Description:    "Main agent with skill, A2A, MCP and tool support",
		Instruction:    d.buildSystemPrompt(),
		Model:          d.defaultModel,
		MaxIterations:  maxIterations,
		ModelRetryConfig: d.buildModelRetryConfig(),
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: d.tools,
				// 不再使用 middleware，直接使用 InvokableApprovableTool wrapper 进行审批
			},
		},
	}

	// 创建 Agent
	mainAgent, err := adk.NewChatModelAgent(ctx, agentConfig)
	if err != nil {
		return nil, fmt.Errorf("create agent failed: %w", err)
	}

	// 创建 Runner
	// 使用全局的 checkpoint store 管理器
	checkpointID := ""
	if d.request.Options != nil && d.request.Options.CheckPointID != "" {
		checkpointID = d.request.Options.CheckPointID
	}
	// 如果没有指定 checkpointID，自动生成一个
	if checkpointID == "" {
		checkpointID = uuid.New().String()
		log.Printf("[Dispatcher] Auto-generated checkpointID=%s", checkpointID)
	}
	var checkpointStore compose.CheckPointStore
	if checkpointID != "" {
		// 尝试获取已有的 checkpoint store
		checkpointStore = GetCheckPointStore(checkpointID)
		if checkpointStore == nil {
			// 创建基于文件的 checkpoint store 并存储（持久化）
			checkpointStore = NewFileCheckPointStore("/tmp/runner_checkpoints")
			SetCheckPointStore(checkpointID, checkpointStore)
			log.Printf("[Dispatcher] Created FileCheckPointStore for checkpointID=%s", checkpointID)
		}
	} else {
		// 如果没有指定 checkpoint ID，使用临时的 store
		checkpointStore = NewInMemoryCheckPointStore()
	}

	runnerConfig := adk.RunnerConfig{
		Agent:           mainAgent,
		CheckPointStore: checkpointStore,
	}

	if d.request.Options != nil && d.request.Options.Stream {
		runnerConfig.EnableStreaming = true
	}

	runner := adk.NewRunner(ctx, runnerConfig)

	// 运行 Agent
	var finalContent string
	var toolCalls []ToolCall
	var finishReason string
	var toolCallsDetail []ToolCallMetadata
	var pendingApprovals []PendingApproval
	var interrupted bool

	// 工具调用计数
	toolCallCount := 0
	maxToolCalls := 0
	if d.request.Options != nil && d.request.Options.MaxToolCalls > 0 {
		maxToolCalls = d.request.Options.MaxToolCalls
	}

	events := runner.Run(ctx, messages, adk.WithCheckPointID(checkpointID))
	for {
		event, ok := events.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			log.Printf("[Dispatcher] Agent error: %v (type: %T)", event.Err, event.Err)
			// 检查是否是 ApprovalInterruptError
			if ae, ok := event.Err.(*ApprovalInterruptError); ok {
				log.Printf("[Dispatcher] Caught ApprovalInterruptError for tool=%s", ae.ToolName)
				interrupted = true
				// 如果有 checkpointID，存储 runner 以便后续 resume
				if checkpointID != "" {
					SetRunner(checkpointID, &adkRunner{
						runner:   runner,
						Messages: messages,
					})
					log.Printf("[Dispatcher] Stored runner for checkpointID=%s", checkpointID)
				}
				pendingApprovals = append(pendingApprovals, PendingApproval{
					ToolName:      ae.ToolName,
					ToolType:      ae.ApprovalInfo.ToolType,
					ArgumentsJSON: ae.ApprovalInfo.ArgumentsInJSON,
					RiskLevel:     ae.RiskLevel,
					Description:   ae.ApprovalInfo.Description,
					InterruptID:   ae.ApprovalInfo.ToolName, // Use tool name as interrupt ID for now
				})
				finishReason = "interrupted"
				// 返回中断结果而不是错误
				return &DispatchResult{
					Content:          "",
					ToolCalls:        toolCalls,
					FinishReason:     "interrupted",
					A2UIMessages:     nil,
					PendingApprovals: pendingApprovals,
					CheckPointID:     checkpointID,
					Metadata: &ResultMetadata{
						Model:             d.defaultModelName,
						TotalLatencyMs:    time.Since(startTime).Milliseconds(),
						ToolCallsCount:    toolCallCount,
						Iterations:        toolCallCount,
						PromptTokens:      totalPromptTokens,
						CompletionTokens:  totalCompletionTokens,
						ToolCallsDetail:   toolCallsDetail,
					},
				}, nil
			}
			// 使用 errors.Is 检查被包装的错误
			var approvalErr *ApprovalInterruptError
			if errors.As(event.Err, &approvalErr) {
				log.Printf("[Dispatcher] Caught wrapped ApprovalInterruptError for tool=%s", approvalErr.ToolName)
				interrupted = true
				pendingApprovals = append(pendingApprovals, PendingApproval{
					ToolName:      approvalErr.ToolName,
					ToolType:      approvalErr.ApprovalInfo.ToolType,
					ArgumentsJSON: approvalErr.ApprovalInfo.ArgumentsInJSON,
					RiskLevel:     approvalErr.RiskLevel,
					Description:   approvalErr.ApprovalInfo.Description,
				})
				finishReason = "interrupted"
				return &DispatchResult{
					Content:          "",
					ToolCalls:        toolCalls,
					FinishReason:     "interrupted",
					A2UIMessages:     nil,
					PendingApprovals: pendingApprovals,
					CheckPointID:     checkpointID,
					Metadata: &ResultMetadata{
						Model:             d.defaultModelName,
						TotalLatencyMs:    time.Since(startTime).Milliseconds(),
						ToolCallsCount:    toolCallCount,
						Iterations:        toolCallCount,
						PromptTokens:      totalPromptTokens,
						CompletionTokens:  totalCompletionTokens,
						ToolCallsDetail:   toolCallsDetail,
					},
				}, nil
			}
			return nil, fmt.Errorf("agent error: %w", event.Err)
		}

		// 首先检查是否是中断事件（不管 event.Output 是否存在）
		if event.Action != nil && event.Action.Interrupted != nil {
			log.Printf("[Dispatcher] >>>>>>> Interrupt detected in event loop!")
			interrupted = true
			// 从已收集的 toolCalls 中获取最近一次工具调用的信息
			// 因为中断是在工具执行过程中发生的
			var lastToolName, lastArgsJSON string
			if len(toolCalls) > 0 {
				lastTool := toolCalls[len(toolCalls)-1]
				lastToolName = lastTool.Tool
				if inputStr, ok := lastTool.Input.(string); ok {
					lastArgsJSON = inputStr
				} else if inputMap, ok := lastTool.Input.(map[string]any); ok {
					if jsonBytes, err := json.Marshal(inputMap); err == nil {
						lastArgsJSON = string(jsonBytes)
					}
				}
				log.Printf("[Dispatcher]   Last tool call: %s, args: %s", lastToolName, lastArgsJSON)
			}
			// 获取中断上下文信息
			for _, ic := range event.Action.Interrupted.InterruptContexts {
				log.Printf("[Dispatcher]   interrupt context: ID=%s", ic.ID)
				// 查找工具配置以获取 ToolType 和 RiskLevel
				var toolType, riskLevel string
				if tc, ok := d.toolConfigs[lastToolName]; ok {
					toolType = tc.Type
					riskLevel = tc.RiskLevel
				}
				log.Printf("[Dispatcher]   tool config: type=%s, risk=%s", toolType, riskLevel)
				pa := PendingApproval{
					InterruptID:   ic.ID,
					ToolName:      lastToolName,
					ToolType:      toolType,
					RiskLevel:     riskLevel,
					ArgumentsJSON: lastArgsJSON,
				}
				pendingApprovals = append(pendingApprovals, pa)
			}
			log.Printf("[Dispatcher] Captured %d pending approvals", len(pendingApprovals))
			// 如果有 checkpointID，存储 runner 以便后续 resume
			if checkpointID != "" {
				SetRunner(checkpointID, &adkRunner{
					runner:   runner,
					Messages: messages,
				})
				log.Printf("[Dispatcher] Stored runner for checkpointID=%s", checkpointID)
			}
			// 中断事件后的 tool calls 应该被忽略
			// 因为这些工具实际上没有执行成功（返回了空结果）
			// 找到最后一个成功的 tool call，截断后续的
			break
		}

		// Skip if event.Output is nil (no message output available)
		if event.Output == nil {
			continue
		}

		// 处理消息输出
		if msg, err := event.Output.MessageOutput.GetMessage(); err == nil {
			finalContent = msg.Content

			// 累计 token 使用量
			if msg.ResponseMeta != nil && msg.ResponseMeta.Usage != nil {
				totalPromptTokens += msg.ResponseMeta.Usage.PromptTokens
				totalCompletionTokens += msg.ResponseMeta.Usage.CompletionTokens
			}

			// 检查 maxTotalTokens 限制
			if maxTotalTokens > 0 && (totalPromptTokens+totalCompletionTokens) > maxTotalTokens {
				return &DispatchResult{
					Content:      finalContent,
					ToolCalls:    toolCalls,
					FinishReason: "max_total_tokens_exceeded",
					Metadata: &ResultMetadata{
						Model:             d.defaultModelName,
						TotalLatencyMs:    time.Since(startTime).Milliseconds(),
						ToolCallsCount:    toolCallCount,
						Iterations:        toolCallCount,
						PromptTokens:      totalPromptTokens,
						CompletionTokens:  totalCompletionTokens,
						ToolCallsDetail:   toolCallsDetail,
					},
				}, nil
			}

			for _, tc := range msg.ToolCalls {
				toolCallCount++
				if maxToolCalls > 0 && toolCallCount > maxToolCalls {
					return &DispatchResult{
						Content:      finalContent,
						ToolCalls:    toolCalls,
						FinishReason: "max_tool_calls_exceeded",
						Metadata: &ResultMetadata{
							Model:          d.defaultModelName,
							TotalLatencyMs: time.Since(startTime).Milliseconds(),
							ToolCallsCount: toolCallCount,
							Iterations:     toolCallCount,
							ToolCallsDetail: toolCallsDetail,
						},
					}, nil
				}

				tcStart := time.Now()
				toolCalls = append(toolCalls, ToolCall{
					Tool:   tc.Function.Name,
					Input:  tc.Function.Arguments,
					Output: nil,
				})
				toolCallsDetail = append(toolCallsDetail, ToolCallMetadata{
					Tool:      tc.Function.Name,
					Input:     tc.Function.Arguments,
					LatencyMs: 0, // 将在工具执行后更新
					Success:   true,
				})
				_ = tcStart // 用于后续精确计时
			}
			if len(msg.ToolCalls) > 0 {
				finishReason = "tool"
			} else {
				finishReason = "stop"
			}
		}

		// 处理流式输出
		if event.Output.MessageOutput != nil {
			if stream := event.Output.MessageOutput.MessageStream; stream != nil {
			for {
				chunk, err := stream.Recv()
				if err != nil {
					// 流关闭或错误时退出
					if err.Error() == "EOF" || strings.Contains(err.Error(), "closed") {
						break
					}
					return nil, fmt.Errorf("stream error: %w", err)
				}
				finalContent += chunk.Content
				for _, tc := range chunk.ToolCalls {
					toolCallCount++
					if maxToolCalls > 0 && toolCallCount > maxToolCalls {
						return &DispatchResult{
							Content:      finalContent,
							ToolCalls:    toolCalls,
							FinishReason: "max_tool_calls_exceeded",
							Metadata: &ResultMetadata{
								Model:          d.defaultModelName,
								TotalLatencyMs: time.Since(startTime).Milliseconds(),
								ToolCallsCount: toolCallCount,
								Iterations:     toolCallCount,
								ToolCallsDetail: toolCallsDetail,
							},
						}, nil
					}
					toolCalls = append(toolCalls, ToolCall{
						Tool:   tc.Function.Name,
						Input:  tc.Function.Arguments,
						Output: nil,
					})
					toolCallsDetail = append(toolCallsDetail, ToolCallMetadata{
						Tool:      tc.Function.Name,
						Input:     tc.Function.Arguments,
						LatencyMs: 0,
						Success:   true,
					})
				}
				if len(chunk.ToolCalls) > 0 {
					finishReason = "tool"
				}
			}
			}
		}
	}

	// 如果被中断，覆盖任何之前的 finishReason
	if interrupted {
		finishReason = "interrupted"
	} else if finishReason == "" {
		finishReason = "stop"
	}

	// 应用响应格式配置
	formattedContent, a2uiMsgs := d.formatResponse(finalContent)

	return &DispatchResult{
		Content:          formattedContent,
		ToolCalls:        toolCalls,
		FinishReason:     finishReason,
		A2UIMessages:     a2uiMsgs,
		PendingApprovals: pendingApprovals,
		CheckPointID:     checkpointID,
		Metadata: &ResultMetadata{
			Model:             d.defaultModelName,
			TotalLatencyMs:    time.Since(startTime).Milliseconds(),
			ToolCallsCount:    toolCallCount,
			Iterations:        toolCallCount,
			PromptTokens:      totalPromptTokens,
			CompletionTokens:  totalCompletionTokens,
			ToolCallsDetail:   toolCallsDetail,
		},
	}, nil
}

// RunStream 流式运行 Agent，返回事件通道
func (d *Dispatcher) RunStream(ctx context.Context) (<-chan StreamEvent, error) {
	eventsChan := make(chan StreamEvent, 100)

	go func() {
		defer close(eventsChan)

		// 1. 初始化模型
		if err := d.initModels(ctx); err != nil {
			eventsChan <- StreamEvent{Type: "error", Data: map[string]any{"error": fmt.Sprintf("init models failed: %v", err)}}
			return
		}

		// 2. 初始化工具
		if err := d.initTools(ctx); err != nil {
			eventsChan <- StreamEvent{Type: "error", Data: map[string]any{"error": fmt.Sprintf("init tools failed: %v", err)}}
			return
		}

		// 3. 初始化 A2A
		if err := d.initA2A(ctx); err != nil {
			eventsChan <- StreamEvent{Type: "error", Data: map[string]any{"error": fmt.Sprintf("init a2a failed: %v", err)}}
			return
		}

		// 4. 初始化内部 agents
		if err := d.initInternalAgents(ctx); err != nil {
			eventsChan <- StreamEvent{Type: "error", Data: map[string]any{"error": fmt.Sprintf("init internal agents failed: %v", err)}}
			return
		}

		// 5. 初始化 MCP
		if err := d.initMCPs(ctx); err != nil {
			eventsChan <- StreamEvent{Type: "error", Data: map[string]any{"error": fmt.Sprintf("init mcps failed: %v", err)}}
			return
		}

		// 6. 初始化 skills
		if err := d.initSkills(ctx); err != nil {
			eventsChan <- StreamEvent{Type: "error", Data: map[string]any{"error": fmt.Sprintf("init skills failed: %v", err)}}
			return
		}

		// 7. 构建消息
		systemPrompt := d.buildSystemPrompt()
		messages, err := d.buildMessagesWithRewrite(ctx, systemPrompt)
		if err != nil {
			eventsChan <- StreamEvent{Type: "error", Data: map[string]any{"error": fmt.Sprintf("build messages failed: %v", err)}}
			return
		}

		// 8. 构建 Agent 配置
		maxIterations := 10
		if d.request.Options != nil && d.request.Options.MaxIterations > 0 {
			maxIterations = d.request.Options.MaxIterations
		}

		agentConfig := &adk.ChatModelAgentConfig{
			Name:           "main_agent",
			Description:    "Main agent with skill, A2A, MCP and tool support",
			Instruction:    d.buildSystemPrompt(),
			Model:          d.defaultModel,
			MaxIterations:  maxIterations,
			ModelRetryConfig: d.buildModelRetryConfig(),
			ToolsConfig: adk.ToolsConfig{
				ToolsNodeConfig: compose.ToolsNodeConfig{
					Tools: d.tools,
				},
			},
		}

		mainAgent, err := adk.NewChatModelAgent(ctx, agentConfig)
		if err != nil {
			eventsChan <- StreamEvent{Type: "error", Data: map[string]any{"error": fmt.Sprintf("create agent failed: %v", err)}}
			return
		}

		// 9. 创建 Runner
		checkpointID := uuid.New().String()
		runnerConfig := adk.RunnerConfig{
			Agent:           mainAgent,
			CheckPointStore: NewInMemoryCheckPointStore(),
			EnableStreaming: true,
		}
		runner := adk.NewRunner(ctx, runnerConfig)

		// 10. 运行 Agent 并发送流式事件
		events := runner.Run(ctx, messages, adk.WithCheckPointID(checkpointID))

		eventsChan <- StreamEvent{Type: "meta", Data: map[string]any{"checkpoint_id": checkpointID}}

		var out strings.Builder
		var totalPromptTokens int
		var totalCompletionTokens int
		var toolCallsCount int
		for {
			event, ok := events.Next()
			if !ok {
				break
			}

			if event.Err != nil {
				eventsChan <- StreamEvent{Type: "error", Data: map[string]any{"error": event.Err.Error()}}
				break
			}

			// 处理输出消息
			if event.Output != nil && event.Output.MessageOutput != nil {
				mo := event.Output.MessageOutput

				// 累计 token 使用量
				if mo.Message != nil && mo.Message.ResponseMeta != nil && mo.Message.ResponseMeta.Usage != nil {
					totalPromptTokens += mo.Message.ResponseMeta.Usage.PromptTokens
					totalCompletionTokens += mo.Message.ResponseMeta.Usage.CompletionTokens
				}

				// 处理 tool calls
				if mo.Message != nil && len(mo.Message.ToolCalls) > 0 {
					toolCallsCount += len(mo.Message.ToolCalls)
					for _, tc := range mo.Message.ToolCalls {
						eventsChan <- StreamEvent{
							Type: "tool_call",
							Data: map[string]any{
								"agent":    event.AgentName,
								"tool":     tc.Function.Name,
								"arguments": tc.Function.Arguments,
							},
						}
					}
				}

				// 处理 assistant 消息内容
				if mo.Message != nil && mo.Message.Role == schema.Assistant && len(mo.Message.ToolCalls) == 0 {
					content := mo.Message.Content
					out.WriteString(content)
					eventsChan <- StreamEvent{Type: "delta", Data: map[string]any{"text": content}}
				}

				// 处理 tool 返回
				if mo.Message != nil && mo.Message.Role == schema.Tool {
					content := mo.Message.Content
					if strings.TrimSpace(content) == "" {
						content = "(无输出)"
					}
					eventsChan <- StreamEvent{
						Type: "tool",
						Data: map[string]any{
							"agent":      event.AgentName,
							"tool":       mo.Message.ToolName,
							"tool_call_id": mo.Message.ToolCallID,
							"output":    content,
						},
					}
				}

				// 处理流式 chunk
				if mo.MessageStream != nil {
					for {
						chunk, err := mo.MessageStream.Recv()
						if errors.Is(err, io.EOF) {
							break
						}
						if err != nil {
							eventsChan <- StreamEvent{Type: "error", Data: map[string]any{"error": err.Error()}}
							break
						}
						if chunk != nil {
							if chunk.Role == schema.Assistant && len(chunk.ToolCalls) == 0 && strings.TrimSpace(chunk.Content) != "" {
								out.WriteString(chunk.Content)
								eventsChan <- StreamEvent{Type: "delta", Data: map[string]any{"text": chunk.Content}}
							}
							if chunk.Role == schema.Tool {
								content := chunk.Content
								if strings.TrimSpace(content) == "" {
									content = "(无输出)"
								}
								eventsChan <- StreamEvent{
									Type: "tool",
									Data: map[string]any{
										"agent":      event.AgentName,
										"tool":       chunk.ToolName,
										"tool_call_id": chunk.ToolCallID,
										"output":    content,
									},
								}
							}
						}
					}
				}
			}

			// 处理中断事件
			if event.Action != nil && event.Action.Interrupted != nil {
				eventsChan <- StreamEvent{
					Type: "interrupted",
					Data: map[string]any{
						"checkpoint_id": checkpointID,
						"data":          event.Action.Interrupted.Data,
					},
				}
			}
		}

		eventsChan <- StreamEvent{Type: "done", Data: map[string]any{
			"content":          out.String(),
			"prompt_tokens":    totalPromptTokens,
			"completion_tokens": totalCompletionTokens,
			"total_tokens":      totalPromptTokens + totalCompletionTokens,
			"tool_calls_count":  toolCallsCount,
		}}
	}()

	return eventsChan, nil
}

// StreamEvent 流式事件
type StreamEvent struct {
	Type string
	Data map[string]any
}

// formatResponse 根据 response_schema 配置格式化响应
func (d *Dispatcher) formatResponse(content string) (string, []json.RawMessage) {
	if d.request.Options == nil || d.request.Options.ResponseSchema == nil {
		return content, nil
	}

	rs := d.request.Options.ResponseSchema
	log.Printf("[Dispatcher] formatResponse: type=%s", rs.Type)

	switch rs.Type {
	case "a2ui":
		// 使用 schema 构建 A2UI 格式
		msgs := d.buildA2UIMessagesFromSchema(content, rs.Schema)
		log.Printf("[Dispatcher] formatResponse: built %d a2ui messages", len(msgs))
		return "", msgs

	case "markdown", "text":
		// 直接返回 markdown 或文本内容
		return content, nil

	case "json":
		// 尝试解析 content 为 JSON 并美化输出
		var data any
		if err := json.Unmarshal([]byte(content), &data); err == nil {
			if prettyJSON, err := json.MarshalIndent(data, "", "  "); err == nil {
				return string(prettyJSON), nil
			}
		}
		return content, nil

	case "image", "audio", "video":
		// 多媒体格式 - 从 content 中解析 URL 或 base64
		// content 应该是 JSON 格式: {"url": "..."} 或 {"base64": "..."}
		var data map[string]any
		if err := json.Unmarshal([]byte(content), &data); err == nil {
			// 返回原始 JSON 作为 content
			if jsonStr, err := json.Marshal(data); err == nil {
				return string(jsonStr), nil
			}
		}
		return content, nil

	case "multipart":
		// 多格式混合 - content 应该是 JSON 数组格式
		return content, nil

	default:
		// 未知格式，返回原始内容
		log.Printf("[Dispatcher] formatResponse: unknown type %s, returning raw content", rs.Type)
		return content, nil
	}
}

// buildA2UIMessagesFromSchema 根据 response_schema 构建 A2UI 格式消息
func (d *Dispatcher) buildA2UIMessagesFromSchema(content string, schema map[string]any) []json.RawMessage {
	msgs := []json.RawMessage{}

	// 解析 schema 获取 properties
	properties, _ := schema["properties"].(map[string]any)

	// 创建默认 surface
	surfaceID := "default_surface"

	createSurface, _ := json.Marshal(map[string]any{
		"createSurface": map[string]any{
			"surfaceId":  surfaceID,
			"catalogId": "standard",
		},
	})
	msgs = append(msgs, createSurface)

	// 构建组件列表
	var components []map[string]any

	// 处理 content 字段 - 如果 schema 中定义了 content 字段，就使用它
	if properties != nil {
		if _, ok := properties["content"]; ok {
			// content 字段存在于 schema 中，添加文本组件
			components = append(components, map[string]any{
				"id":        "content",
				"component": "Text",
				"text":      map[string]any{"text": content},
			})
		}

		// 处理 action 字段
		if actionProp, ok := properties["action"].(map[string]any); ok {
			if actionType, _ := actionProp["type"].(string); actionType != "" {
				components = append(components, map[string]any{
					"id":         "action",
					"component":  "Action",
					"actionType": actionType,
				})
			}
		}

		// 处理 card 字段
		if cardProp, ok := properties["card"].(map[string]any); ok {
			if cardSchema, ok := cardProp["properties"].(map[string]any); ok {
				card := map[string]any{
					"id":        "card",
					"component": "Card",
				}
				if title, _ := cardSchema["title"].(string); title != "" {
					card["title"] = title
				}
				components = append(components, card)
			}
		}
	}

	// 如果没有从 schema 解析到组件，使用默认文本组件
	if len(components) == 0 {
		components = []map[string]any{
			{
				"id":        "text_content",
				"component": "Text",
				"text":      map[string]any{"text": content},
			},
		}
	}

	updateComponents, _ := json.Marshal(map[string]any{
		"updateComponents": map[string]any{
			"surfaceId":  surfaceID,
			"components": components,
		},
	})
	msgs = append(msgs, updateComponents)

	return msgs
}