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
	"sync"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/jettjia/XiaoQinglong/runner/contextcompressor"
	"github.com/jettjia/XiaoQinglong/runner/contextcompressor/compactors"
	"github.com/jettjia/XiaoQinglong/runner/pkg/logger"
	"github.com/jettjia/XiaoQinglong/runner/plugins"
	"github.com/jettjia/XiaoQinglong/runner/prompt"
	"github.com/jettjia/XiaoQinglong/runner/retriever"
	"github.com/jettjia/XiaoQinglong/runner/subagent"
	"github.com/jettjia/XiaoQinglong/runner/types"
)

// contextKey 用于在 context 中传递数据
type contextKey string

// ========== Type Aliases & Constants ==========

type ToolCall = types.ToolCall

const (
	ModelRoleDefault   ModelRole = "default"
	ModelRoleRewrite   ModelRole = "rewrite"
	ModelRoleSkill     ModelRole = "skill"
	ModelRoleSummarize ModelRole = "summarize"
)

type ModelRole = types.ModelRole

// ========== Global Checkpoint Store Manager ==========

var (
	checkpointStores = make(map[string]compose.CheckPointStore)
	checkpointMu     sync.RWMutex
	runners          = make(map[string]*adkRunner)
	runnersMu        sync.RWMutex
)

type adkRunner struct {
	runner   *adk.Runner
	Messages []adk.Message
}

func GetCheckPointStore(id string) compose.CheckPointStore {
	checkpointMu.RLock()
	defer checkpointMu.RUnlock()
	return checkpointStores[id]
}

func SetCheckPointStore(id string, store compose.CheckPointStore) {
	checkpointMu.Lock()
	defer checkpointMu.Unlock()
	checkpointStores[id] = store
}

func GetRunner(id string) *adkRunner {
	runnersMu.RLock()
	defer runnersMu.RUnlock()
	return runners[id]
}

func SetRunner(id string, r *adkRunner) {
	runnersMu.Lock()
	defer runnersMu.Unlock()
	runners[id] = r
}

// ========== Runner ==========

type Runner struct {
	request    *types.RunRequest
	dispatcher *Dispatcher
}

func NewRunner(req *types.RunRequest) *Runner {
	return &Runner{
		request:    req,
		dispatcher: NewDispatcher(req),
	}
}

func (r *Runner) Run(ctx context.Context) (*DispatchResult, error) {
	return r.dispatcher.Run(ctx)
}

func (r *Runner) RunStream(ctx context.Context) (<-chan StreamEvent, error) {
	return r.dispatcher.RunStream(ctx)
}

func (r *Runner) Resume(ctx context.Context, req *types.ResumeRequest) (*types.ResumeResponse, error) {
	// 获取存储的 runner
	adkRunner := GetRunner(req.CheckPointID)
	if adkRunner == nil {
		return &types.ResumeResponse{
			Success: false,
			Error:   "checkpoint not found or expired",
		}, nil
	}

	// 构建 approval results
	approvals := make(map[string]any)
	for _, approval := range req.Approvals {
		result := &types.ApprovalResult{
			Approved:         approval.Approved,
			DisapproveReason: approval.DisapproveReason,
		}
		approvals[approval.InterruptID] = result
	}

	// 恢复执行
	events, err := adkRunner.runner.ResumeWithParams(ctx, req.CheckPointID, &adk.ResumeParams{
		Targets: approvals,
	})
	if err != nil {
		return &types.ResumeResponse{
			Success: false,
			Error:   fmt.Sprintf("resume failed: %v", err),
		}, nil
	}

	// 处理事件
	var finalContent string
	var toolCalls []types.ToolCall
	var finishReason string
	var toolCallsDetail []types.ToolCallMetadata
	toolCallCount := 0

	for {
		event, ok := events.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			return &types.ResumeResponse{
				Success: false,
				Error:   fmt.Sprintf("resume error: %v", event.Err),
			}, nil
		}

		if event.Output != nil {
			if msg, err := event.Output.MessageOutput.GetMessage(); err == nil {
				finalContent = msg.Content

				for _, tc := range msg.ToolCalls {
					toolCallCount++
					tcMeta := types.ToolCallMetadata{
						Tool:  tc.Function.Name,
						Input: tc.Function.Arguments,
					}
					toolCallsDetail = append(toolCallsDetail, tcMeta)

					toolCalls = append(toolCalls, types.ToolCall{
						Tool:   tc.Function.Name,
						Input:  tc.Function.Arguments,
						Output: nil,
					})
				}

				if len(msg.ToolCalls) == 0 && finishReason == "" {
					finishReason = "completed"
				}
			}
		}
	}

	metadata := types.ResponseMetadata{
		Model:          r.getDefaultModelName(),
		ToolCallsCount: toolCallCount,
		Iterations:     toolCallCount,
	}

	return &types.ResumeResponse{
		Success:      true,
		FinishReason: finishReason,
		Content:      finalContent,
		ToolCalls:    toolCalls,
		Metadata:     metadata,
	}, nil
}

func (r *Runner) getDefaultModelName() string {
	if r.request.Models == nil {
		return ""
	}
	cfg, ok := r.request.Models["default"]
	if !ok {
		return ""
	}
	return cfg.Name
}

const eventsChanKey contextKey = "eventsChan"
const toolArgsMapKey contextKey = "toolArgsMap"

// toolArgsMap 用于保存 tool_call_id -> arguments 的映射
type toolArgsMapType map[string]string

// withEventsChan 将 eventsChan 添加到 context 中
func withEventsChan(ctx context.Context, ch chan StreamEvent) context.Context {
	return context.WithValue(ctx, eventsChanKey, ch)
}

// getEventsChan 从 context 中获取 eventsChan
func getEventsChan(ctx context.Context) chan StreamEvent {
	ch, _ := ctx.Value(eventsChanKey).(chan StreamEvent)
	return ch
}

// withToolArgsMap 将 toolArgsMap 添加到 context 中
func withToolArgsMap(ctx context.Context, m toolArgsMapType) context.Context {
	return context.WithValue(ctx, toolArgsMapKey, m)
}

// getToolArgsMap 从 context 中获取 toolArgsMap
func getToolArgsMap(ctx context.Context) toolArgsMapType {
	m, _ := ctx.Value(toolArgsMapKey).(toolArgsMapType)
	return m
}

// ========== Dispatch Result ==========

type DispatchResult struct {
	Content          string
	ToolCalls        []ToolCall
	A2AResults       []types.A2AResult
	FinishReason     string
	A2UIMessages     []json.RawMessage
	TokensUsed       int
	Metadata         *ResultMetadata
	PendingApprovals []types.PendingApproval
	CheckPointID     string
	Memories         []types.MemoryEntry // 提取的记忆，供 agent-frame 保存
}

type ResultMetadata struct {
	Model            string
	LatencyMs        int64
	TotalLatencyMs   int64
	PromptTokens     int
	CompletionTokens int
	ToolCallsCount   int
	A2ACallsCount    int
	SkillCallsCount  int
	Iterations       int
	ToolCallsDetail  []types.ToolCallMetadata
	Error            string
}

// toolCallEventsMiddleware 工具调用中间件，用于发送 tool_call 事件
func toolCallEventsMiddleware() *compose.ToolMiddleware {
	return &compose.ToolMiddleware{
		Invokable: func(next compose.InvokableToolEndpoint) compose.InvokableToolEndpoint {
			return func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
				logger.Infof("[ToolMiddleware] tool=%s, callID=%s, arguments=%s", input.Name, input.CallID, input.Arguments)

				// 保存 callID -> arguments 到 map
				toolArgsMap := getToolArgsMap(ctx)
				if toolArgsMap != nil && input.CallID != "" {
					toolArgsMap[input.CallID] = input.Arguments
					logger.Infof("[ToolMiddleware] Saved arguments for callID=%s", input.CallID)
				}

				// 发送 tool_call 事件
				eventsChan := getEventsChan(ctx)
				if eventsChan != nil {
					logger.Infof("[ToolMiddleware] Sending tool_call event for tool=%s", input.Name)
					eventsChan <- StreamEvent{
						Type: "tool_call",
						Data: map[string]any{
							"agent":        "main_agent",
							"tool":         input.Name,
							"tool_call_id": input.CallID,
							"arguments":    input.Arguments,
						},
					}
					logger.Infof("[ToolMiddleware] tool_call event sent")
				} else {
					logger.Warnf("[ToolMiddleware] eventsChan is nil!")
				}

				// 执行实际的工具调用
				return next(ctx, input)
			}
		},
	}
}

// ========== Dispatcher ==========

// CompactionRequest 压缩请求
type CompactionRequest struct {
	Messages []contextcompressor.Message
}

// CompactionResponse 压缩响应
type CompactionResponse struct {
	Result *contextcompressor.CompactionResult
	Error  error
}

type Dispatcher struct {
	request *types.RunRequest

	// 组件
	defaultModel     model.ToolCallingChatModel
	defaultModelName string
	models           map[string]model.ToolCallingChatModel
	modelsByRole     map[ModelRole]model.ToolCallingChatModel
	tools            []tool.BaseTool
	toolConfigs      map[string]types.ToolConfig // tool name -> config for interrupt handling
	a2aRunners       map[string]*adk.Runner
	internalAgents   map[string]adk.Agent
	skillRunner      *plugins.SkillRunner
	skillPlanner     *plugins.SkillPlanner     // LLM 驱动的技能规划器
	subAgentManager  *subagent.SubAgentManager // Sub-Agent 管理器
	a2aCallCount     int
	cliExt           interface{}               // CLI 扩展（cliext.CLIExtension）

	// 文件上传相关
	uploadsBaseDir string // uploads 目录的宿主机路径

	// 上下文压缩器
	compactService  *contextcompressor.IntegrationService
	compactChan     chan CompactionRequest  // 发送压缩请求
	compactDoneChan chan CompactionResponse // 接收压缩结果
	pendingCompact  *CompactionResponse     // 待应用的压缩结果
	compactMu       sync.Mutex              // 保护 pendingCompact

	// 知识检索器
	knowledgeRetriever *retriever.KnowledgeRetriever
}

func NewDispatcher(req *types.RunRequest) *Dispatcher {
	d := &Dispatcher{
		request:     req,
		toolConfigs: make(map[string]types.ToolConfig),
	}
	// 初始化压缩通道
	d.compactChan = make(chan CompactionRequest, 1)
	d.compactDoneChan = make(chan CompactionResponse, 1)
	return d
}

func (d *Dispatcher) Run(ctx context.Context) (*DispatchResult, error) {
	// 调试
	fmt.Println(">>>>>> [Dispatcher] Run: STARTING NOW fmt.Println")
	log.Println(">>>>>> [Dispatcher] Run: STARTING NOW log.Println")
	logger.Infof(">>>>>> [Dispatcher] Run: STARTING NOW logger.Infof")
	// 0. 设置 uploadsBaseDir
	d.setUploadsBaseDir()

	// 1. 初始化模型（必须串行，模型是其他组件的依赖）
	logger.Infof("[Dispatcher] Run: calling initModels")
	if err := d.initModels(ctx); err != nil {
		return nil, fmt.Errorf("init models failed: %w", err)
	}
	logger.Infof("[Dispatcher] Run: initModels completed, calling initParallel")

	// 2. 并行初始化其他组件（在 initModels 之后）
	logger.Infof("[Dispatcher] Run: about to call initParallel")
	if fatal, _ := d.initParallel(ctx); fatal != nil {
		return nil, fatal
	}
	logger.Infof("[Dispatcher] Run: initParallel completed")

	// 3. 构建系统 prompt
	systemPrompt := d.buildSystemPrompt()

	// 4. 构建消息（如果配置了rewrite模型，则对用户query进行改写）
	messages, err := d.buildMessagesWithRewrite(ctx, systemPrompt)
	if err != nil {
		return nil, fmt.Errorf("build messages failed: %w", err)
	}

	// 5. 创建 Agent 并运行
	return d.runAgent(ctx, messages)
}

// setUploadsBaseDir 设置 uploads 目录的宿主机路径
// 从 context 或 sandbox.volumes 中获取
func (d *Dispatcher) setUploadsBaseDir() {
	logger.Infof("[Dispatcher] setUploadsBaseDir: context=%v", d.request.Context)
	// 优先从 context 中获取
	if dir, ok := d.request.Context["uploads_dir"].(string); ok && dir != "" {
		logger.Infof("[Dispatcher] setUploadsBaseDir: set from context to %s", dir)
		d.uploadsBaseDir = dir
		return
	}

	// 从 sandbox.volumes 中查找 /mnt/uploads 的挂载路径
	if d.request.Sandbox != nil && len(d.request.Sandbox.Volumes) > 0 {
		for _, vol := range d.request.Sandbox.Volumes {
			if vol.ContainerPath == "/mnt/uploads" || vol.ContainerPath == "/mnt/uploads/" {
				d.uploadsBaseDir = vol.HostPath
				return
			}
		}
	}

	// 默认路径
	d.uploadsBaseDir = "./data/uploads"
}

// initCompactService 初始化上下文压缩服务
func (d *Dispatcher) initCompactService(ctx context.Context) {
	// 检查是否启用上下文压缩
	// 可以通过 request.Options 中的某个字段控制，这里默认启用
	if d.defaultModel == nil {
		logger.Infof("[Dispatcher] initCompactService: no default model, skipping")
		return
	}

	// 创建 ChatModel 代理
	chatModelProxy := &contextcompressor.ChatModelProxy{
		GenerateFunc: func(ctx context.Context, messages []compactors.Message) (string, error) {
			// 转换消息格式
			var chatMessages []*schema.Message
			for _, m := range messages {
				switch m.Type {
				case "user":
					chatMessages = append(chatMessages, schema.UserMessage(m.GetLastText()))
				case "assistant":
					chatMessages = append(chatMessages, schema.AssistantMessage(m.GetLastText(), nil))
				case "system":
					chatMessages = append(chatMessages, schema.SystemMessage(m.GetLastText()))
				}
			}

			// 调用模型
			resp, err := d.defaultModel.Generate(ctx, chatMessages)
			if err != nil {
				return "", err
			}
			return resp.Content, nil
		},
	}

	// 创建 tokenizer
	tokenizer := contextcompressor.NewDefaultTokenizerImpl(4.0)

	// 创建压缩器
	opts := []contextcompressor.Option{}
	// 如果用户配置了 max_total_tokens，使用它作为压缩阈值
	if d.request.Options != nil && d.request.Options.MaxTotalTokens > 0 {
		opts = append(opts, contextcompressor.WithCustomThreshold(d.request.Options.MaxTotalTokens))
		logger.Infof("[Dispatcher] initCompactService: using custom threshold=%d from max_total_tokens", d.request.Options.MaxTotalTokens)
	}
	compactor := contextcompressor.NewCompactor(chatModelProxy, tokenizer, opts...)

	// 创建集成服务
	d.compactService = contextcompressor.NewIntegrationService(compactor)

	logger.Infof("[Dispatcher] initCompactService: enabled with model=%s", d.defaultModelName)
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

	logger.Infof("[Dispatcher] Wrapping tool %s (%s) with approval, risk_level=%s", toolName, toolType, riskLevel)
	return NewInvokableApprovableTool(t, toolName, toolType, riskLevel)
}

// buildToolRiskLevels 构建工具名称到风险级别的映射
func (d *Dispatcher) buildToolRiskLevels() map[string]string {
	riskLevels := make(map[string]string)

	logger.Infof("[Dispatcher] buildToolRiskLevels: d.request.Tools has %d tools", len(d.request.Tools))
	for _, tc := range d.request.Tools {
		logger.Infof("[Dispatcher]   tool: name=%s, type=%s, risk_level=%s", tc.Name, tc.Type, tc.RiskLevel)
		riskLevels[tc.Name] = tc.RiskLevel
	}

	logger.Infof("[Dispatcher] buildToolRiskLevels: d.request.A2A has %d agents", len(d.request.A2A))
	// A2A agents
	for _, cfg := range d.request.A2A {
		logger.Infof("[Dispatcher]   a2a: name=%s, risk_level=%s", cfg.Name, cfg.RiskLevel)
		riskLevels["a2a"] = cfg.RiskLevel
	}

	logger.Infof("[Dispatcher] buildToolRiskLevels: d.request.MCPs has %d configs", len(d.request.MCPs))
	// MCP tools (using the mcp name)
	for _, cfg := range d.request.MCPs {
		logger.Infof("[Dispatcher]   mcp: name=%s, risk_level=%s", cfg.Name, cfg.RiskLevel)
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
	logger.Infof("[Dispatcher] Building approval middleware with %d tools, threshold=%s", len(riskLevels), approvalThreshold)

	return newApprovalToolMiddleware(riskLevels).Wrap
}

// mcpStdioTool stdio 模式的 MCP tool
type mcpStdioTool struct {
	name   string
	client *plugins.MCPStdioClient
}

func (t *mcpStdioTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: t.name,
		Desc: fmt.Sprintf("MCP tool via stdio: %s", t.name),
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"args": {Type: schema.Object, Desc: "Tool arguments", Required: false},
		}),
	}, nil
}

func (t *mcpStdioTool) InvokableRun(ctx context.Context, argumentsInJSON string, opt ...tool.Option) (string, error) {
	logger.Infof("[MCP Stdio] InvokableRun: tool=%s, argumentsInJSON=%s", t.name, argumentsInJSON)

	var args map[string]any
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	logger.Infof("[MCP Stdio] CallTool: tool=%s, args=%+v", t.name, args)

	// 工具名在 t.name，参数在 args 中
	return t.client.CallTool(ctx, t.name, args)
}

// createInternalAgent 创建内部 agent
func (d *Dispatcher) createInternalAgent(ctx context.Context, cfg types.InternalAgentConfig) (adk.Agent, error) {
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
	// 收集所有启用的工具名称
	var enabledTools []string
	for _, t := range d.tools {
		info, _ := t.Info(context.Background())
		if info != nil {
			enabledTools = append(enabledTools, info.Name)
		}
	}

	// 使用结构化 Prompt 构建器
	basePrompt := prompt.BuildDefaultPrompt(d.request, enabledTools)

	// 尝试加载记忆 section（从 Context 中获取 memory service）
	memorySection := d.loadMemorySection()
	if memorySection != "" {
		return basePrompt + "\n\n" + memorySection
	}

	return basePrompt
}

// loadMemorySection 从 Context 中获取 memory service 并加载记忆 section
func (d *Dispatcher) loadMemorySection() string {
	// 从 context 中获取 memory service
	// Context 中存储的是 agent-frame 的 memory service
	memorySvcInterface := d.request.Context["memory_svc"]
	if memorySvcInterface == nil {
		return ""
	}

	// 获取 agent_id 和 user_id（这些也应该在 context 中）
	agentId := ""
	userId := ""
	if v, ok := d.request.Context["agent_id"].(string); ok {
		agentId = v
	}
	if v, ok := d.request.Context["user_id"].(string); ok {
		userId = v
	}
	if agentId == "" || userId == "" {
		return ""
	}

	// 动态调用 memory service（避免直接依赖 agent-frame）
	// 这里使用反射来避免循环依赖
	return d.extractMemorySectionFromService(memorySvcInterface, agentId, userId)
}

// extractMemorySectionFromService 从 memory service 提取记忆 section
func (d *Dispatcher) extractMemorySectionFromService(svc interface{}, agentId, userId string) string {
	// 使用 type assertion 来调用 memory service 的方法
	// 这避免了 runner 对 agent-frame 的直接 import

	// 获取 GetMemoryIndex 方法
	type memoryIndexGetter interface {
		GetMemoryIndex(ctx context.Context, agentId, userId string) (interface{}, error)
	}

	// 获取 FindByAgentAndUser 方法
	type memoryFinder interface {
		FindByAgentAndUser(ctx context.Context, agentId, userId string) (interface{}, error)
	}

	// 尝试获取 memory index
	if getter, ok := svc.(memoryIndexGetter); ok {
		indices, err := getter.GetMemoryIndex(context.Background(), agentId, userId)
		if err != nil || indices == nil {
			return ""
		}

		// 将 indices 转换为 []string（hook lines）
		indexLines := d.convertMemoryIndexToLines(indices)
		return prompt.GetMemorySection(indexLines)
	}

	return ""
}

// convertMemoryIndexToLines 将 memory index 转换为 hook line 列表
func (d *Dispatcher) convertMemoryIndexToLines(indices interface{}) []string {
	// indices 是 []interface{} 或具体的 slice 类型
	// 反射提取每个元素的 hook_line 字段

	lines := make([]string, 0, 200)

	// 假设 indices 是 []map[string]interface{} 或类似结构
	// 由于是动态调用，我们做简单的类型检查

	switch v := indices.(type) {
	case []interface{}:
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				if hookLine, ok := m["hook_line"].(string); ok {
					lines = append(lines, hookLine)
				}
			}
		}
	case []map[string]interface{}:
		for _, m := range v {
			if hookLine, ok := m["hook_line"].(string); ok {
				lines = append(lines, hookLine)
			}
		}
	}

	return lines
}

// buildMessagesWithRewrite builds messages and optionally rewrites the last user query
func (d *Dispatcher) buildMessagesWithRewrite(ctx context.Context, systemPrompt string) ([]adk.Message, error) {
	messages := d.buildMessages(systemPrompt)

	// 提取用户 query（用于知识召回）
	userQuery := d.extractUserQuery(messages)
	logger.Infof("[Dispatcher] buildMessagesWithRewrite: knowledgeRetriever=%v, userQuery=%s", d.knowledgeRetriever != nil, userQuery)

	// 执行知识库召回（如果有配置）
	kbSection := ""
	if d.knowledgeRetriever != nil && userQuery != "" {
		kbSection = d.retrieveKnowledge(ctx, userQuery)
		if kbSection != "" {
			systemPrompt = systemPrompt + "\n\n" + kbSection
			// 更新 system message
			for i := range messages {
				if messages[i].Role == "system" {
					messages[i].Content = systemPrompt
					break
				}
			}
			logger.Infof("[Dispatcher] knowledge retrieved: %d chars added to system prompt", len(kbSection))
		} else {
			logger.Infof("[Dispatcher] knowledge retrieved: no docs found")
		}
	}

	// 检查是否需要rewrite：如果有rewrite模型且最后一条是user message
	routingCfg := d.getRoutingConfig()
	if routingCfg != nil && d.modelsByRole[ModelRoleRewrite] != nil {
		// 找到最后一条user message
		for i := len(messages) - 1; i >= 0; i-- {
			if messages[i].Role == "user" {
				// 调用rewrite模型改写query
				rewritten, err := d.rewriteQuery(ctx, messages[i].Content)
				if err != nil {
					logger.Infof("[Dispatcher] rewriteQuery failed: %v, using original", err)
					break
				}
				logger.Infof("[Dispatcher] query rewritten: %s -> %s", messages[i].Content, rewritten)
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

// extractUserQuery 提取最后一条用户消息作为 query
func (d *Dispatcher) extractUserQuery(messages []adk.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
		if messages[i].Role == "system" || messages[i].Role == "assistant" {
			break
		}
	}
	return ""
}

// retrieveKnowledge 从知识库检索相关内容
func (d *Dispatcher) retrieveKnowledge(ctx context.Context, query string) string {
	logger.Infof("[Dispatcher] retrieveKnowledge: calling retrieve for query=%s", query)
	docs, err := d.knowledgeRetriever.Retrieve(ctx, query)
	if err != nil {
		logger.Warnf("[Dispatcher] retrieveKnowledge failed: %v", err)
		return ""
	}

	logger.Infof("[Dispatcher] retrieveKnowledge: got %d docs", len(docs))
	if len(docs) == 0 {
		return ""
	}

	// 格式化成 section
	var lines []string
	lines = append(lines, "# Knowledge Base")
	lines = append(lines, "Use the following information from knowledge base to answer questions:")
	lines = append(lines, "")
	for i, doc := range docs {
		lines = append(lines, fmt.Sprintf("## [%d] %s", i+1, doc.MetaData["title"]))
		lines = append(lines, fmt.Sprintf("Source: %s (score: %.2f)", doc.MetaData["kb_name"], doc.MetaData["score"]))
		lines = append(lines, "")
		lines = append(lines, doc.Content)
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func (d *Dispatcher) buildMessages(systemPrompt string) []adk.Message {
	var messages []adk.Message

	// 添加 system message - systemPrompt 包含 knowledge、skills 等信息，所以只要有内容就添加
	if systemPrompt != "" {
		messages = append(messages, schema.SystemMessage(systemPrompt))
		logger.Infof("[Dispatcher] buildMessages: added systemPrompt, length=%d", len(systemPrompt))
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

	logger.Infof("[Dispatcher] buildMessages: total messages=%d", len(messages))

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
func (d *Dispatcher) getRoutingConfig() *types.RoutingConfig {
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

// compactionWorker 异步压缩 worker
func (d *Dispatcher) compactionWorker(ctx context.Context) {
	logger.Infof("[Dispatcher] compactionWorker: started")
	for {
		select {
		case <-ctx.Done():
			logger.Infof("[Dispatcher] compactionWorker: context cancelled, exiting")
			return
		case req, ok := <-d.compactChan:
			if !ok {
				logger.Infof("[Dispatcher] compactionWorker: channel closed, exiting")
				return
			}
			logger.Infof("[Dispatcher] compactionWorker: received compaction request, messages=%d", len(req.Messages))

			// 执行压缩
			result, err := d.compactService.Compact(ctx, req.Messages)

			// 发送结果
			resp := CompactionResponse{Result: result, Error: err}
			select {
			case d.compactDoneChan <- resp:
				logger.Infof("[Dispatcher] compactionWorker: sent result, success=%v", err == nil)
			case <-ctx.Done():
				logger.Infof("[Dispatcher] compactionWorker: context cancelled while sending result")
				return
			}
		}
	}
}

// checkAndTriggerCompaction 检查并触发压缩
func (d *Dispatcher) checkAndTriggerCompaction(messages []adk.Message) {
	if d.compactService == nil || !d.compactService.IsEnabled() {
		return
	}

	// 转换消息格式
	ccMessages := d.convertToCCMessages(messages)

	// 检查是否需要压缩
	if !d.compactService.ShouldCompact(ccMessages) {
		return
	}

	// 触发异步压缩
	logger.Infof("[Dispatcher] checkAndTriggerCompaction: triggering async compaction, messages=%d", len(ccMessages))
	select {
	case d.compactChan <- CompactionRequest{Messages: ccMessages}:
		logger.Infof("[Dispatcher] checkAndTriggerCompaction: sent compaction request")
	default:
		logger.Infof("[Dispatcher] checkAndTriggerCompaction: channel full, skipping")
	}
}

// applyCompactionResultNonBlocking 非阻塞应用压缩结果
// checkpointID 用于日志记录
func (d *Dispatcher) applyCompactionResultNonBlocking(messages *[]adk.Message, checkpointID string) {
	select {
	case resp, ok := <-d.compactDoneChan:
		if !ok {
			return
		}
		if resp.Error != nil {
			logger.Infof("[Dispatcher] applyCompactionResultNonBlocking: compaction failed: %v", resp.Error)
			return
		}

		// 压缩成功，转换为 adk.Message
		compactedCC := contextcompressor.BuildPostCompactMessages(resp.Result)
		compactedAdk := d.convertFromCCMessages(compactedCC)

		logger.Infof("[Dispatcher] applyCompactionResultNonBlocking: applied compaction [checkpointID=%s], original=%d, compacted=%d",
			checkpointID, len(*messages), len(compactedAdk))

		// 替换消息
		*messages = compactedAdk

	default:
		// 没有结果可读
	}
}

// convertToCCMessages 将 adk.Message 转换为 contextcompressor.Message
func (d *Dispatcher) convertToCCMessages(messages []adk.Message) []contextcompressor.Message {
	result := make([]contextcompressor.Message, 0, len(messages))
	for _, m := range messages {
		// adk.Message = *schema.Message
		ccMsg := contextcompressor.Message{
			Role: string(m.Role),
		}
		switch m.Role {
		case schema.User:
			ccMsg.Type = contextcompressor.MessageTypeUser
			ccMsg.Content = []contextcompressor.ContentBlock{{Type: "text", Text: m.Content}}
		case schema.Assistant:
			ccMsg.Type = contextcompressor.MessageTypeAssistant
			ccMsg.Content = []contextcompressor.ContentBlock{{Type: "text", Text: m.Content}}
		case schema.System:
			ccMsg.Type = contextcompressor.MessageTypeSystem
			ccMsg.Content = []contextcompressor.ContentBlock{{Type: "text", Text: m.Content}}
		}
		result = append(result, ccMsg)
	}
	return result
}

// convertFromCCMessages 将 contextcompressor.Message 转换回 adk.Message
func (d *Dispatcher) convertFromCCMessages(messages []contextcompressor.Message) []adk.Message {
	result := make([]adk.Message, 0, len(messages))
	for _, m := range messages {
		text := ""
		for _, block := range m.Content {
			if block.Type == "text" {
				text += block.Text
			}
		}
		switch m.Type {
		case contextcompressor.MessageTypeUser:
			result = append(result, schema.UserMessage(text))
		case contextcompressor.MessageTypeAssistant:
			result = append(result, schema.AssistantMessage(text, nil))
		case contextcompressor.MessageTypeSystem:
			result = append(result, schema.SystemMessage(text))
		}
	}
	return result
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
		Name:             "main_agent",
		Description:      "Main agent with skill, A2A, MCP and tool support",
		Instruction:      d.buildSystemPrompt(),
		Model:            d.defaultModel,
		MaxIterations:    maxIterations,
		ModelRetryConfig: d.buildModelRetryConfig(),
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools:               d.tools,
				ToolCallMiddlewares: []compose.ToolMiddleware{*toolCallEventsMiddleware()},
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
		logger.Infof("[Dispatcher] Auto-generated checkpointID=%s", checkpointID)
	}
	var checkpointStore compose.CheckPointStore
	if checkpointID != "" {
		// 尝试获取已有的 checkpoint store
		checkpointStore = GetCheckPointStore(checkpointID)
		if checkpointStore == nil {
			// 创建基于文件的 checkpoint store 并存储（持久化）
			checkpointStore = NewFileCheckPointStore("/tmp/runner_checkpoints")
			SetCheckPointStore(checkpointID, checkpointStore)
			logger.Infof("[Dispatcher] Created FileCheckPointStore for checkpointID=%s", checkpointID)
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
	var toolCallsDetail []types.ToolCallMetadata
	var pendingApprovals []types.PendingApproval
	var interrupted bool

	// 工具调用计数
	toolCallCount := 0
	maxToolCalls := 0
	if d.request.Options != nil && d.request.Options.MaxToolCalls > 0 {
		maxToolCalls = d.request.Options.MaxToolCalls
	}

	events := runner.Run(ctx, messages, adk.WithCheckPointID(checkpointID))

	// 启动异步压缩 worker
	if d.compactService != nil && d.compactService.IsEnabled() {
		go d.compactionWorker(ctx)
		// 检查初始是否需要压缩
		d.checkAndTriggerCompaction(messages)
	}

	for {
		// 非阻塞检查压缩结果
		d.applyCompactionResultNonBlocking(&messages, checkpointID)

		event, ok := events.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			logger.Infof("[Dispatcher] Agent error: %v (type: %T)", event.Err, event.Err)
			// 检查是否是 ApprovalInterruptError
			if ae, ok := event.Err.(*ApprovalInterruptError); ok {
				logger.Infof("[Dispatcher] Caught ApprovalInterruptError for tool=%s", ae.ToolName)
				interrupted = true
				// 如果有 checkpointID，存储 runner 以便后续 resume
				if checkpointID != "" {
					SetRunner(checkpointID, &adkRunner{
						runner:   runner,
						Messages: messages,
					})
					logger.Infof("[Dispatcher] Stored runner for checkpointID=%s", checkpointID)
				}
				pendingApprovals = append(pendingApprovals, types.PendingApproval{
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
						Model:            d.defaultModelName,
						TotalLatencyMs:   time.Since(startTime).Milliseconds(),
						ToolCallsCount:   toolCallCount,
						Iterations:       toolCallCount,
						PromptTokens:     totalPromptTokens,
						CompletionTokens: totalCompletionTokens,
						ToolCallsDetail:  toolCallsDetail,
					},
				}, nil
			}
			// 使用 errors.Is 检查被包装的错误
			var approvalErr *ApprovalInterruptError
			if errors.As(event.Err, &approvalErr) {
				logger.Infof("[Dispatcher] Caught wrapped ApprovalInterruptError for tool=%s", approvalErr.ToolName)
				interrupted = true
				pendingApprovals = append(pendingApprovals, types.PendingApproval{
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
						Model:            d.defaultModelName,
						TotalLatencyMs:   time.Since(startTime).Milliseconds(),
						ToolCallsCount:   toolCallCount,
						Iterations:       toolCallCount,
						PromptTokens:     totalPromptTokens,
						CompletionTokens: totalCompletionTokens,
						ToolCallsDetail:  toolCallsDetail,
					},
				}, nil
			}
			return nil, fmt.Errorf("agent error: %w", event.Err)
		}

		// 首先检查是否是中断事件（不管 event.Output 是否存在）
		if event.Action != nil && event.Action.Interrupted != nil {
			logger.Infof("[Dispatcher] >>>>>>> Interrupt detected in event loop!")
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
				logger.Infof("[Dispatcher]   Last tool call: %s, args: %s", lastToolName, lastArgsJSON)
			}
			// 获取中断上下文信息
			for _, ic := range event.Action.Interrupted.InterruptContexts {
				logger.Infof("[Dispatcher]   interrupt context: ID=%s", ic.ID)
				// 查找工具配置以获取 ToolType 和 RiskLevel
				var toolType, riskLevel string
				if tc, ok := d.toolConfigs[lastToolName]; ok {
					toolType = tc.Type
					riskLevel = tc.RiskLevel
				}
				logger.Infof("[Dispatcher]   tool config: type=%s, risk=%s", toolType, riskLevel)
				pa := types.PendingApproval{
					InterruptID:   ic.ID,
					ToolName:      lastToolName,
					ToolType:      toolType,
					RiskLevel:     riskLevel,
					ArgumentsJSON: lastArgsJSON,
				}
				pendingApprovals = append(pendingApprovals, pa)
			}
			logger.Infof("[Dispatcher] Captured %d pending approvals", len(pendingApprovals))
			// 如果有 checkpointID，存储 runner 以便后续 resume
			if checkpointID != "" {
				SetRunner(checkpointID, &adkRunner{
					runner:   runner,
					Messages: messages,
				})
				logger.Infof("[Dispatcher] Stored runner for checkpointID=%s", checkpointID)
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
						Model:            d.defaultModelName,
						TotalLatencyMs:   time.Since(startTime).Milliseconds(),
						ToolCallsCount:   toolCallCount,
						Iterations:       toolCallCount,
						PromptTokens:     totalPromptTokens,
						CompletionTokens: totalCompletionTokens,
						ToolCallsDetail:  toolCallsDetail,
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
							Model:           d.defaultModelName,
							TotalLatencyMs:  time.Since(startTime).Milliseconds(),
							ToolCallsCount:  toolCallCount,
							Iterations:      toolCallCount,
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
				toolCallsDetail = append(toolCallsDetail, types.ToolCallMetadata{
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
									Model:           d.defaultModelName,
									TotalLatencyMs:  time.Since(startTime).Milliseconds(),
									ToolCallsCount:  toolCallCount,
									Iterations:      toolCallCount,
									ToolCallsDetail: toolCallsDetail,
								},
							}, nil
						}
						toolCalls = append(toolCalls, ToolCall{
							Tool:   tc.Function.Name,
							Input:  tc.Function.Arguments,
							Output: nil,
						})
						toolCallsDetail = append(toolCallsDetail, types.ToolCallMetadata{
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
			Model:            d.defaultModelName,
			TotalLatencyMs:   time.Since(startTime).Milliseconds(),
			ToolCallsCount:   toolCallCount,
			Iterations:       toolCallCount,
			PromptTokens:     totalPromptTokens,
			CompletionTokens: totalCompletionTokens,
			ToolCallsDetail:  toolCallsDetail,
		},
	}, nil
}

// RunStream 流式运行 Agent，返回事件通道
func (d *Dispatcher) RunStream(ctx context.Context) (<-chan StreamEvent, error) {
	eventsChan := make(chan StreamEvent, 100)

	go func() {
		defer close(eventsChan)

		// 0. 设置 uploadsBaseDir
		d.setUploadsBaseDir()

		// 1. 初始化模型
		if err := d.initModels(ctx); err != nil {
			eventsChan <- StreamEvent{Type: "error", Data: map[string]any{"error": fmt.Sprintf("init models failed: %v", err)}}
			return
		}

		// 1.5 初始化压缩服务
		d.initCompactService(ctx)

		// 2. 初始化工具
		if err := d.initTools(ctx); err != nil {
			eventsChan <- StreamEvent{Type: "error", Data: map[string]any{"error": fmt.Sprintf("init tools failed: %v", err)}}
			return
		}

		// 2.1 初始化知识检索器
		if err := d.initKnowledgeRetriever(ctx); err != nil {
			logger.Infof("[Dispatcher] RunStream: warning - init knowledge retriever failed: %v", err)
		}

		// 2.2 初始化文件检索工具
		if err := d.initFileRetrieval(ctx); err != nil {
			logger.Infof("[Dispatcher] RunStream: warning - init file retrieval failed: %v", err)
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

		// 6.1 初始化 SubAgents
		if err := d.initSubAgents(ctx); err != nil {
			logger.Infof("[Dispatcher] RunStream: warning - init sub-agents failed: %v", err)
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
			Name:             "main_agent",
			Description:      "Main agent with skill, A2A, MCP and tool support",
			Instruction:      d.buildSystemPrompt(),
			Model:            d.defaultModel,
			MaxIterations:    maxIterations,
			ModelRetryConfig: d.buildModelRetryConfig(),
			ToolsConfig: adk.ToolsConfig{
				ToolsNodeConfig: compose.ToolsNodeConfig{
					Tools:               d.tools,
					ToolCallMiddlewares: []compose.ToolMiddleware{*toolCallEventsMiddleware()},
				},
			},
		}

		// 将 eventsChan 和 toolArgsMap 添加到 context 中，供中间件使用
		ctxWithChan := withEventsChan(ctx, eventsChan)
		toolArgsMap := make(toolArgsMapType)
		ctxWithChan = withToolArgsMap(ctxWithChan, toolArgsMap)

		mainAgent, err := adk.NewChatModelAgent(ctxWithChan, agentConfig)
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
		events := runner.Run(ctxWithChan, messages, adk.WithCheckPointID(checkpointID))

		eventsChan <- StreamEvent{Type: "meta", Data: map[string]any{"checkpoint_id": checkpointID}}

		// 10.5 启动异步压缩 worker
		if d.compactService != nil && d.compactService.IsEnabled() {
			go d.compactionWorker(ctx)
			d.checkAndTriggerCompaction(messages)
		}

		var out strings.Builder
		var totalPromptTokens int
		var totalCompletionTokens int
		var toolCallsCount int
		// 从 context 获取 toolArgsMap
		toolArgsMap = getToolArgsMap(ctxWithChan)
		if toolArgsMap == nil {
			toolArgsMap = make(toolArgsMapType)
		}
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
						// 保存 arguments 到 map
						toolArgsMap[tc.ID] = tc.Function.Arguments
						eventsChan <- StreamEvent{
							Type: "tool_call",
							Data: map[string]any{
								"agent":     event.AgentName,
								"tool":      tc.Function.Name,
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
					// 获取对应的 arguments
					args := toolArgsMap[mo.Message.ToolCallID]
					eventsChan <- StreamEvent{
						Type: "tool",
						Data: map[string]any{
							"agent":        event.AgentName,
							"tool":         mo.Message.ToolName,
							"tool_call_id": mo.Message.ToolCallID,
							"arguments":    args,
							"output":       content,
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
								// 获取对应的 arguments
								args := toolArgsMap[chunk.ToolCallID]
								eventsChan <- StreamEvent{
									Type: "tool",
									Data: map[string]any{
										"agent":        event.AgentName,
										"tool":         chunk.ToolName,
										"tool_call_id": chunk.ToolCallID,
										"arguments":    args,
										"output":       content,
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
			"content":           out.String(),
			"prompt_tokens":     totalPromptTokens,
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
