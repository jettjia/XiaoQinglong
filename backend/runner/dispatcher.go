package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino-ext/components/model/openai"
)

// ========== Dispatch Result ==========

type DispatchResult struct {
	Content      string
	ToolCalls    []ToolCall
	A2AResults   []A2AResult
	FinishReason string
}

// ========== Dispatcher ==========

type Dispatcher struct {
	request *RunRequest

	// 组件
	defaultModel model.ToolCallingChatModel
	models       map[string]model.ToolCallingChatModel
	tools        []tool.BaseTool
	a2aRunners   map[string]*adk.Runner
	internalAgents map[string]adk.Agent
	skillRunner  *SkillRunner
}

func NewDispatcher(req *RunRequest) *Dispatcher {
	return &Dispatcher{
		request: req,
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

	// 5. 初始化 skill runner
	if err := d.initSkills(ctx); err != nil {
		return nil, fmt.Errorf("init skills failed: %w", err)
	}

	// 6. 构建系统 prompt
	systemPrompt := d.buildSystemPrompt()

	// 7. 构建消息
	messages := d.buildMessages(systemPrompt)

	// 8. 创建 Agent 并运行
	return d.runAgent(ctx, messages)
}

func (d *Dispatcher) initModels(ctx context.Context) error {
	d.models = make(map[string]model.ToolCallingChatModel)

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

		// 设置 default model
		if key == "default" {
			d.defaultModel = cm
		}
	}

	if d.defaultModel == nil {
		return fmt.Errorf("no default model configured")
	}

	return nil
}

func (d *Dispatcher) initTools(ctx context.Context) error {
	var tools []tool.BaseTool

	for _, tc := range d.request.Tools {
		switch tc.Type {
		case "http":
			tools = append(tools, NewHTTPTool(tc))
		}
	}

	d.tools = tools
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
			d.tools = append(d.tools, NewA2ATool(clients))
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

	// 创建 skill tool 并添加到 tools
	skillTool := d.skillRunner.BuildSkillTool()
	if skillTool != nil {
		d.tools = append(d.tools, skillTool)
	}

	// 创建 load_skill tool
	loadSkillTool := d.skillRunner.BuildLoadSkillTool()
	if loadSkillTool != nil {
		d.tools = append(d.tools, loadSkillTool)
	}

	log.Printf("[Dispatcher] initSkills: %d skills registered, config: %s",
		len(d.request.Skills), DefaultSkillConfigPath())

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

func (d *Dispatcher) runAgent(ctx context.Context, messages []adk.Message) (*DispatchResult, error) {
	// 计算最大迭代次数
	maxIterations := 10
	if d.request.Options != nil && d.request.Options.MaxIterations > 0 {
		maxIterations = d.request.Options.MaxIterations
	}

	// 构建 Agent 配置
	agentConfig := &adk.ChatModelAgentConfig{
		Name:           "main_agent",
		Description:    "Main agent with skill, A2A, MCP and tool support",
		Instruction:    d.buildSystemPrompt(),
		Model:          d.defaultModel,
		MaxIterations:  maxIterations,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: d.tools,
			},
		},
	}

	// 创建 Agent
	mainAgent, err := adk.NewChatModelAgent(ctx, agentConfig)
	if err != nil {
		return nil, fmt.Errorf("create agent failed: %w", err)
	}

	// 创建 Runner
	runnerConfig := adk.RunnerConfig{
		Agent: mainAgent,
	}

	if d.request.Options != nil && d.request.Options.Stream {
		runnerConfig.EnableStreaming = true
	}

	runner := adk.NewRunner(ctx, runnerConfig)

	// 运行 Agent
	var finalContent string
	var toolCalls []ToolCall
	var finishReason string

	events := runner.Run(ctx, messages)
	for {
		event, ok := events.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			return nil, fmt.Errorf("agent error: %w", event.Err)
		}

		// 处理消息输出
		if msg, err := event.Output.MessageOutput.GetMessage(); err == nil {
			finalContent = msg.Content
			for _, tc := range msg.ToolCalls {
				toolCalls = append(toolCalls, ToolCall{
					Tool:   tc.Function.Name,
					Input:  tc.Function.Arguments,
					Output: nil,
				})
			}
			if len(msg.ToolCalls) > 0 {
				finishReason = "tool"
			} else {
				finishReason = "stop"
			}
		}

		// 处理流式输出
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
					toolCalls = append(toolCalls, ToolCall{
						Tool:   tc.Function.Name,
						Input:  tc.Function.Arguments,
						Output: nil,
					})
				}
				if len(chunk.ToolCalls) > 0 {
					finishReason = "tool"
				}
			}
		}
	}

	if finishReason == "" {
		finishReason = "stop"
	}

	return &DispatchResult{
		Content:      finalContent,
		ToolCalls:    toolCalls,
		FinishReason: finishReason,
	}, nil
}