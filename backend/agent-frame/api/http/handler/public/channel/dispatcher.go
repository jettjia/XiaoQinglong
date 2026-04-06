package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	agentDto "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/agent"
	chatDto "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/chat"
	agentSvc "github.com/jettjia/xiaoqinglong/agent-frame/application/service/agent"
	chatSvc "github.com/jettjia/xiaoqinglong/agent-frame/application/service/chat"
	channelDto "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/channel"
	channelSvc "github.com/jettjia/xiaoqinglong/agent-frame/application/service/channel"
	memorySvc "github.com/jettjia/xiaoqinglong/agent-frame/domain/srv/memory"
	"github.com/jettjia/xiaoqinglong/agent-frame/pkg/logger"
)

// FeishuWSSender 飞书 WS 发送器接口（避免循环依赖）
type FeishuWSSender interface {
	SendText(ctx context.Context, receiveID, msgType, content string) error
}

// DingTalkWSSender 钉钉 WS 发送器接口（避免循环依赖）
type DingTalkWSSender interface {
	SendText(ctx context.Context, receiveID, msgType, content string) error
}

// WeWorkWSSender 企业微信 WS 发送器接口（避免循环依赖）
type WeWorkWSSender interface {
	SendText(ctx context.Context, receiveID, msgType, content string) error
}

// WeixinWSSender 微信 WS 发送器接口（避免循环依赖）
type WeixinWSSender interface {
	SendText(ctx context.Context, receiveID, msgType, content string) error
}

// ChannelDispatcher 渠道调度器
type ChannelDispatcher struct {
	inboundHandlers  map[string]InboundHandler
	outboundHandlers map[string]OutboundHandler
	runnerURL        string
	agentSvc         *agentSvc.SysAgentService
	chatSvc          *chatSvc.ChatMessageService
	channelSvc       *channelSvc.SysChannelService
	memorySvc        *memorySvc.AgentMemorySvc
}

// 全局调度器实例（供 boot 包使用）
var globalDispatcher *ChannelDispatcher

// 全局 Feishu WS 发送器（供 dispatcher 使用，避免循环依赖）
var globalFeishuWSSender FeishuWSSender

// SetFeishuWSSender 设置全局 Feishu WS 发送器
func SetFeishuWSSender(sender FeishuWSSender) {
	globalFeishuWSSender = sender
}

// 全局 DingTalk WS 发送器（供 dispatcher 使用，避免循环依赖）
var globalDingTalkWSSender DingTalkWSSender

// SetDingTalkWSSender 设置全局 DingTalk WS 发送器
func SetDingTalkWSSender(sender DingTalkWSSender) {
	globalDingTalkWSSender = sender
}

// 全局 WeWork WS 发送器（供 dispatcher 使用，避免循环依赖）
var globalWeWorkWSSender WeWorkWSSender

// SetWeWorkWSSender 设置全局 WeWork WS 发送器
func SetWeWorkWSSender(sender WeWorkWSSender) {
	globalWeWorkWSSender = sender
}

// 全局 Weixin WS 发送器（供 dispatcher 使用，避免循环依赖）
var globalWeixinWSSender WeixinWSSender

// SetWeixinWSSender 设置全局 Weixin WS 发送器
func SetWeixinWSSender(sender WeixinWSSender) {
	globalWeixinWSSender = sender
}

// GetGlobalDispatcher 获取全局调度器实例
func GetGlobalDispatcher() *ChannelDispatcher {
	return globalDispatcher
}

// NewChannelDispatcher 创建调度器
func NewChannelDispatcher() *ChannelDispatcher {
	dispatcher := &ChannelDispatcher{
		inboundHandlers:  make(map[string]InboundHandler),
		outboundHandlers: make(map[string]OutboundHandler),
		runnerURL:        "http://localhost:18080",
		agentSvc:         agentSvc.NewSysAgentService(),
		chatSvc:          chatSvc.NewChatMessageService(),
		channelSvc:       channelSvc.NewSysChannelService(),
		memorySvc:        memorySvc.NewAgentMemorySvc(),
	}
	globalDispatcher = dispatcher
	return dispatcher
}

// RegisterInboundHandler 注册入站处理器
func (d *ChannelDispatcher) RegisterInboundHandler(code string, handler InboundHandler) {
	d.inboundHandlers[code] = handler
}

// RegisterOutboundHandler 注册出站处理器
func (d *ChannelDispatcher) RegisterOutboundHandler(code string, handler OutboundHandler) {
	d.outboundHandlers[code] = handler
}

// HandleCallback 处理 webhook 回调（飞书、微信等）
func (d *ChannelDispatcher) HandleCallback() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 获取 channel code
		channelCode := c.Param("channel")
		if channelCode == "" {
			c.JSON(400, gin.H{"error": "missing channel code"})
			return
		}

		// 2. 获取 handler
		inboundHandler, ok := d.inboundHandlers[channelCode]
		if !ok {
			c.JSON(404, gin.H{"error": "channel not supported: " + channelCode})
			return
		}

		// 3. 特殊处理：GET 请求通常是 URL 验证（如飞书）
		if c.Request.Method == "GET" {
			if err := inboundHandler.Validate(c); err != nil {
				logger.GetRunnerLogger().Warnf("Channel %s URL verification failed: %v", channelCode, err)
				c.JSON(401, gin.H{"error": "verification failed: " + err.Error()})
				return
			}
			// Validate 已经返回了 challenge 响应
			return
		}

		// 4. POST 请求：Validate 签名
		if err := inboundHandler.Validate(c); err != nil {
			logger.GetRunnerLogger().Warnf("Channel %s validation failed: %v", channelCode, err)
			c.JSON(401, gin.H{"error": "validation failed: " + err.Error()})
			return
		}

		// 5. Parse request
		ctx, err := inboundHandler.ParseRequest(c)
		if err != nil {
			logger.GetRunnerLogger().Warnf("Channel %s parse failed: %v", channelCode, err)
			c.JSON(400, gin.H{"error": "parse request failed: " + err.Error()})
			return
		}

		// 6. 获取渠道配置
		channelConfig, err := d.channelSvc.FindSysChannelByCode(c.Request.Context(), channelCode)
		if err != nil {
			logger.GetRunnerLogger().Errorf("Failed to get channel config for %s: %v", channelCode, err)
			c.JSON(500, gin.H{"error": "failed to get channel config"})
			return
		}
		if channelConfig == nil {
			c.JSON(404, gin.H{"error": "channel not found: " + channelCode})
			return
		}
		ctx.ChannelID = channelConfig.Ulid
		ctx.Config = channelConfig.Config

		// 7. Ack 确认收到（如果是 webhook 的话）
		outboundHandler, ok := d.outboundHandlers[channelCode]
		if ok {
			if err := outboundHandler.Ack(ctx); err != nil {
				logger.GetRunnerLogger().Warnf("Channel %s ack failed: %v", channelCode, err)
			}
		}

		// 8. 调用 runner 并响应
		d.runAndRespond(c, ctx)
	}
}

// runAndRespond 调用 runner 并通过 OutboundHandler 发送响应
func (d *ChannelDispatcher) runAndRespond(c *gin.Context, ctx *ChannelContext) {
	// 1. 获取 Agent 配置（从请求中的 AgentID 或者从 Session 关联的 Agent）
	var agentRsp *agentDto.FindSysAgentRsp
	var err error

	if ctx.AgentID != "" {
		agentRsp, err = d.agentSvc.FindSysAgentById(c.Request.Context(), &agentDto.FindSysAgentByIdReq{
			Ulid: ctx.AgentID,
		})
		if err != nil || agentRsp == nil {
			d.sendError(c, ctx, errors.New("agent not found"))
			return
		}
	} else {
		// TODO: 如果没有 AgentID，需要从 Session 或者其他方式获取
		d.sendError(c, ctx, errors.New("agent_id is required"))
		return
	}

	// 2. 构建 runner 请求
	runnerReq, err := d.buildRunnerRequest(c, agentRsp, ctx)
	if err != nil {
		d.sendError(c, ctx, err)
		return
	}

	// 3. 序列化请求
	runnerBody, err := json.Marshal(runnerReq)
	if err != nil {
		d.sendError(c, ctx, errors.New("failed to build runner request"))
		return
	}

	// 4. 发送请求到 runner
	runURL := d.runnerURL + "/run"
	req, err := http.NewRequest("POST", runURL, bytes.NewReader(runnerBody))
	if err != nil {
		d.sendError(c, ctx, errors.New("failed to create request"))
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.GetRunnerLogger().WithError(err).Error("Failed to call runner")
		d.sendError(c, ctx, errors.New("failed to call runner service"))
		return
	}
	defer resp.Body.Close()

	// 5. 检查是否是流式响应
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/event-stream") {
		// 流式响应 - Web 渠道支持，飞书/微信不支持
		outboundHandler, ok := d.outboundHandlers[ctx.ChannelCode]
		if ok {
			outboundHandler.SendStream(ctx, resp.Body)
		}
		return
	}

	// 6. 非流式响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		d.sendError(c, ctx, errors.New("failed to read runner response"))
		return
	}

	// 7. 解析响应，发送给渠道
	var respData map[string]any
	if err := json.Unmarshal(respBody, &respData); err == nil {
		outboundHandler, ok := d.outboundHandlers[ctx.ChannelCode]
		if ok && respData["content"] != nil {
			if content, ok := respData["content"].(string); ok {
				if err := outboundHandler.SendText(ctx, content); err != nil {
					logger.GetRunnerLogger().WithError(err).Errorf("Failed to send text to channel %s", ctx.ChannelCode)
				}
			}
		}
	}

	// 8. 返回原始响应（供调试）
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
}

// buildRunnerRequest 构建 runner 请求
func (d *ChannelDispatcher) buildRunnerRequest(c *gin.Context, agentRsp *agentDto.FindSysAgentRsp, ctx *ChannelContext) (map[string]any, error) {
	// 1. 解析 Agent config_json
	var agentConfig map[string]any
	if agentRsp.ConfigJson != "" {
		if err := json.Unmarshal([]byte(agentRsp.ConfigJson), &agentConfig); err != nil {
			return nil, errors.New("failed to parse agent config: " + err.Error())
		}
	}

	// 2. 获取 runner URL
	runnerURL := d.runnerURL
	if endpoint, ok := agentConfig["endpoint"].(string); ok && endpoint != "" {
		runnerURL = strings.TrimSuffix(endpoint, "/run")
		runnerURL = strings.TrimSuffix(runnerURL, "/")
	}
	d.runnerURL = runnerURL

	// 3. 构建 runner 请求
	runnerReq := make(map[string]any)

	// 从 agent config 中提取 runner 需要的字段
	if models, ok := agentConfig["models"].(map[string]any); ok && len(models) > 0 {
		runnerReq["models"] = models
	}
	if systemPrompt, ok := agentConfig["system_prompt"].(string); ok {
		runnerReq["prompt"] = systemPrompt
	}
	if tools, ok := agentConfig["tools"].([]any); ok && len(tools) > 0 {
		runnerReq["tools"] = tools
	}
	if skills, ok := agentConfig["skills"].([]any); ok && len(skills) > 0 {
		runnerReq["skills"] = skills
	}
	if options, ok := agentConfig["options"].(map[string]any); ok && len(options) > 0 {
		runnerReq["options"] = options
	}
	if knowledgeBases, ok := agentConfig["knowledge_bases"].([]any); ok && len(knowledgeBases) > 0 {
		runnerReq["knowledge_bases"] = knowledgeBases
	}
	if mcps, ok := agentConfig["mcps"].([]any); ok && len(mcps) > 0 {
		runnerReq["mcps"] = mcps
	}
	if a2a, ok := agentConfig["a2a"].([]any); ok && len(a2a) > 0 {
		runnerReq["a2a"] = a2a
	}
	if sandbox, ok := agentConfig["sandbox"].(map[string]any); ok && len(sandbox) > 0 {
		runnerReq["sandbox"] = sandbox
	}

	// 4. 加载历史消息
	chatReq := ctx.Request.(ChatRunReq)
	var historicalMessages []map[string]any
	if chatReq.SessionID != "" && !chatReq.IsTest {
		historicalMessages, _ = d.loadHistoricalMessages(c.Request.Context(), chatReq.SessionID, agentConfig)
	}

	// 5. 加载长期记忆
	var memoryContext []map[string]any
	if chatReq.SessionID != "" && chatReq.AgentID != "" && !chatReq.IsTest {
		memoryContext, _ = d.loadMemoryContext(c.Request.Context(), chatReq.AgentID, chatReq.UserID, chatReq.Input, agentConfig)
	}

	// 6. 构建 messages 数组
	messages := make([]map[string]any, 0)
	messages = append(messages, memoryContext...)
	messages = append(messages, historicalMessages...)
	messages = append(messages, map[string]any{
		"role":    "user",
		"content": chatReq.Input,
	})
	runnerReq["messages"] = messages

	// 7. 获取 uploads 目录
	uploadsDir := os.Getenv("APP_DATA")
	if uploadsDir == "" {
		uploadsDir = "/tmp/xiaoqinglong/data"
	}
	uploadsDir = filepath.Join(uploadsDir, "uploads")

	// 8. 构建 context
	runnerReq["context"] = map[string]any{
		"session_id":  chatReq.SessionID,
		"user_id":     chatReq.UserID,
		"channel_id":  ctx.ChannelCode,
		"agent_id":    chatReq.AgentID,
		"uploads_dir": uploadsDir,
		"memory_svc":  d.memorySvc,
	}

	return runnerReq, nil
}

// loadHistoricalMessages 加载历史消息
func (d *ChannelDispatcher) loadHistoricalMessages(ctx context.Context, sessionID string, agentConfig map[string]any) ([]map[string]any, error) {
	msgs, err := d.chatSvc.FindChatMessagesBySessionId(ctx, &chatDto.FindChatMessagesBySessionIdReq{
		SessionId: sessionID,
	})
	if err != nil {
		return nil, err
	}
	if len(msgs) == 0 {
		return []map[string]any{}, nil
	}

	// 解析 context_window 配置
	contextWindow := map[string]any{
		"max_rounds": 10,
		"strategy":   "sliding_window",
	}
	if cw, ok := agentConfig["context_window"].(map[string]any); ok {
		if maxRounds, ok := cw["max_rounds"].(float64); ok {
			contextWindow["max_rounds"] = int(maxRounds)
		}
		if strategy, ok := cw["strategy"].(string); ok {
			contextWindow["strategy"] = strategy
		}
	}

	maxRounds := contextWindow["max_rounds"].(int)
	strategy := contextWindow["strategy"].(string)

	if strategy == "sliding_window" {
		return buildSlidingWindowMessages(msgs, maxRounds), nil
	}

	result := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		result = append(result, map[string]any{
			"role":    m.Role,
			"content": m.Content,
		})
	}
	return result, nil
}

// buildSlidingWindowMessages 构建滑动窗口消息
func buildSlidingWindowMessages(msgs []*chatDto.ChatMessageRsp, maxRounds int) []map[string]any {
	if len(msgs) == 0 {
		return []map[string]any{}
	}

	rounds := make([][]*chatDto.ChatMessageRsp, 0)
	currentRound := make([]*chatDto.ChatMessageRsp, 0)

	for _, msg := range msgs {
		if msg.Role == "user" {
			if len(currentRound) > 0 && currentRound[len(currentRound)-1].Role == "assistant" {
				rounds = append(rounds, currentRound)
				currentRound = make([]*chatDto.ChatMessageRsp, 0)
			}
			currentRound = append(currentRound, msg)
		} else if msg.Role == "assistant" {
			currentRound = append(currentRound, msg)
		}
	}

	if len(currentRound) > 0 {
		rounds = append(rounds, currentRound)
	}

	if len(rounds) <= maxRounds {
		result := make([]map[string]any, 0)
		for _, round := range rounds {
			for _, msg := range round {
				result = append(result, map[string]any{
					"role":    msg.Role,
					"content": msg.Content,
				})
			}
		}
		return result
	}

	result := make([]map[string]any, 0)
	for _, round := range rounds[len(rounds)-maxRounds:] {
		for _, msg := range round {
			result = append(result, map[string]any{
				"role":    msg.Role,
				"content": msg.Content,
			})
		}
	}
	return result
}

// loadMemoryContext 加载长期记忆上下文
func (d *ChannelDispatcher) loadMemoryContext(ctx context.Context, agentId, userId, query string, agentConfig map[string]any) ([]map[string]any, error) {
	enableMemory := false
	maxMemoryCount := 5

	if memConfig, ok := agentConfig["long_term_memory"].(map[string]any); ok {
		if enabled, ok := memConfig["enabled"].(bool); ok {
			enableMemory = enabled
		}
		if count, ok := memConfig["max_count"].(float64); ok {
			maxMemoryCount = int(count)
		}
	}

	if !enableMemory {
		return nil, nil
	}

	memories, err := d.memorySvc.GetRelevantMemories(ctx, agentId, userId, query, maxMemoryCount)
	if err != nil {
		return nil, err
	}

	if len(memories) == 0 {
		return nil, nil
	}

	result := make([]map[string]any, 0, len(memories))
	for _, m := range memories {
		result = append(result, map[string]any{
			"role":    "system",
			"content": "[记忆] " + m.Content,
		})
	}

	return result, nil
}

// sendError 发送错误响应
func (d *ChannelDispatcher) sendError(c *gin.Context, ctx *ChannelContext, err error) {
	outboundHandler, ok := d.outboundHandlers[ctx.ChannelCode]
	if ok {
		outboundHandler.SendError(ctx, err)
	}
	c.JSON(500, gin.H{"error": err.Error()})
}

// GetChannelConfig 根据 code 获取渠道配置
func (d *ChannelDispatcher) GetChannelConfig(code string) (*channelSvc.SysChannelService, error) {
	return nil, nil
}

// HandleFeishuWSMessage 处理飞书 WebSocket 消息
func (d *ChannelDispatcher) HandleFeishuWSMessage(ctx *MessageContext) error {
	log := logger.GetRunnerLogger()
	log.Infof("[Dispatcher] HandleFeishuWSMessage: sessionID=%s, userID=%s, content=%s",
		ctx.SessionID, ctx.UserID, ctx.Content)

	// 1. 查找或创建 Session
	var agentId string
	session, err := d.chatSvc.FindOrCreateChatSessionByChannel(context.Background(), ctx.UserID, "feishu", "")
	if err != nil {
		log.Warnf("[Dispatcher] Failed to find/create session: %v", err)
		return err
	}

	// 如果 session 没有绑定 agent，需要路由选择
	if session.AgentId == "" {
		log.Infof("[Dispatcher] Session has no agent bound, routing...")

		// 2. LLM 路由选择 Agent
		agent, err := d.routeAgentForChannel(ctx.Content, "feishu")
		if err != nil {
			log.Warnf("[Dispatcher] Failed to route agent: %v", err)
			// 发送错误消息
			d.sendFeishuWSText(ctx, "抱歉，暂时没有可用的 Agent 处理您的请求。")
			return err
		}
		agentId = agent.Ulid

		// 3. 更新 Session 绑定 Agent
		err = d.chatSvc.UpdateChatSession(context.Background(), &chatDto.UpdateChatSessionReq{
			Ulid:    session.Ulid,
			AgentId: agentId,
		})
		if err != nil {
			log.Warnf("[Dispatcher] Failed to bind agent to session: %v", err)
		}
		log.Infof("[Dispatcher] Bound agent %s to session %s", agentId, session.Ulid)
	} else {
		agentId = session.AgentId
		log.Infof("[Dispatcher] Session already bound to agent %s", agentId)
	}

	// 4. 构建请求并调用 runner
	return d.runAndRespondWS(ctx, agentId, session.Ulid)
}

// routeAgentForChannel 根据消息内容和渠道路由选择 Agent（LLM 路由）
func (d *ChannelDispatcher) routeAgentForChannel(content, channel string) (*agentDto.FindSysAgentRsp, error) {
	log := logger.GetRunnerLogger()

	// 1. 获取所有 Agent
	allAgents, err := d.agentSvc.FindSysAgentAll(context.Background(), &agentDto.FindSysAgentAllReq{})
	if err != nil {
		return nil, err
	}
	if len(allAgents) == 0 {
		return nil, errors.New("no agent available")
	}

	// 2. 过滤出支持该渠道的 Agent
	var feishuAgents []*agentDto.FindSysAgentRsp
	for _, agent := range allAgents {
		// 检查 Agent 的 channels 配置
		if agent.Channels != "" {
			var channels []string
			if err := json.Unmarshal([]byte(agent.Channels), &channels); err == nil {
				for _, ch := range channels {
					if ch == channel {
						feishuAgents = append(feishuAgents, agent)
						break
					}
				}
			}
		}
	}

	// 如果没有配置 channels，则默认支持所有渠道
	if len(feishuAgents) == 0 {
		feishuAgents = allAgents
	}

	// 3. 只有一个 Agent，直接返回
	if len(feishuAgents) == 1 {
		return feishuAgents[0], nil
	}

	// 4. 多个 Agent，使用 LLM 选择最合适的
	log.Infof("[Dispatcher] Multiple agents (%d), using LLM to select", len(feishuAgents))

	// 构建选择 prompt
	agentNames := make([]string, 0)
	agentMap := make(map[string]string)
	for _, agent := range feishuAgents {
		agentNames = append(agentNames, agent.Name)
		agentMap[agent.Name] = agent.Ulid
	}

	// 简单的 LLM 调用（这里使用第一个，实际应该调用 LLM）
	// TODO: 实现真正的 LLM 路由
	selectedName := agentNames[0]
	log.Warnf("[Dispatcher] LLM routing not implemented, defaulting to first agent: %s", selectedName)

	return &agentDto.FindSysAgentRsp{
		Ulid: agentMap[selectedName],
		Name: selectedName,
	}, nil
}

// runAndRespondWS 非 HTTP context 下调用 runner 并发送响应
func (d *ChannelDispatcher) runAndRespondWS(ctx *MessageContext, agentId, sessionId string) error {
	log := logger.GetRunnerLogger()

	// 1. 获取 Agent 配置
	agentRsp, err := d.agentSvc.FindSysAgentById(context.Background(), &agentDto.FindSysAgentByIdReq{
		Ulid: agentId,
	})
	if err != nil || agentRsp == nil {
		log.Warnf("[Dispatcher] Agent not found: %s, err=%v", agentId, err)
		d.sendFeishuWSText(ctx, "抱歉，Agent 未找到。")
		return err
	}

	// 2. 构建 runner 请求
	runnerReq, err := d.buildRunnerRequestWS(agentRsp, ctx, agentId, sessionId)
	if err != nil {
		log.Warnf("[Dispatcher] Failed to build runner request: %v", err)
		d.sendFeishuWSText(ctx, "抱歉，请求构建失败。")
		return err
	}

	// 3. 序列化请求
	runnerBody, err := json.Marshal(runnerReq)
	if err != nil {
		log.Warnf("[Dispatcher] Failed to marshal runner request: %v", err)
		d.sendFeishuWSText(ctx, "抱歉，请求序列化失败。")
		return err
	}

	// 4. 发送请求到 runner
	runURL := d.runnerURL + "/run"
	req, err := http.NewRequest("POST", runURL, bytes.NewReader(runnerBody))
	if err != nil {
		log.Warnf("[Dispatcher] Failed to create request: %v", err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.WithError(err).Error("[Dispatcher] Failed to call runner")
		d.sendFeishuWSText(ctx, "抱歉，暂时无法处理您的请求。")
		return err
	}
	defer resp.Body.Close()

	// 5. 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Warnf("[Dispatcher] Failed to read runner response: %v", err)
		d.sendFeishuWSText(ctx, "抱歉，响应读取失败。")
		return err
	}

	// 6. 解析响应，发送文本
	var respData map[string]any
	if err := json.Unmarshal(respBody, &respData); err != nil {
		// 非 JSON 响应（可能是 SSE 事件流格式）
		respText := string(respBody)
		log.Warnf("[Dispatcher] Response is not JSON, attempting SSE parse: %s", respText[:min(200, len(respText))])

		// 尝试解析 SSE 格式的事件
		content := parseSSEResponse(respText)
		if content != "" {
			if err := d.sendFeishuWSText(ctx, content); err != nil {
				log.WithError(err).Error("[Dispatcher] Failed to send Feishu WS text")
				return err
			}
			return nil
		}

		d.sendFeishuWSText(ctx, "抱歉，暂时无法处理您的请求。")
		return errors.New("failed to parse response")
	}

	if respData["Content"] != nil {
		if content, ok := respData["Content"].(string); ok {
			if err := d.sendFeishuWSText(ctx, content); err != nil {
				log.WithError(err).Error("[Dispatcher] Failed to send Feishu WS text")
				return err
			}
		}
	}

	// 7. 保存消息到历史记录
	go d.saveWSMessageToHistory(agentId, ctx.UserID, sessionId, ctx.Content, respData)

	return nil
}

// buildRunnerRequestWS 为 WebSocket 上下文构建 runner 请求
func (d *ChannelDispatcher) buildRunnerRequestWS(agentRsp *agentDto.FindSysAgentRsp, ctx *MessageContext, agentId, sessionId string) (map[string]any, error) {
	// 1. 解析 Agent config_json
	var agentConfig map[string]any
	if agentRsp.ConfigJson != "" {
		if err := json.Unmarshal([]byte(agentRsp.ConfigJson), &agentConfig); err != nil {
			return nil, errors.New("failed to parse agent config: " + err.Error())
		}
	}

	// 2. 获取 runner URL
	runnerURL := d.runnerURL
	if endpoint, ok := agentConfig["endpoint"].(string); ok && endpoint != "" {
		runnerURL = strings.TrimSuffix(endpoint, "/run")
		runnerURL = strings.TrimSuffix(runnerURL, "/")
	}
	d.runnerURL = runnerURL

	// 3. 构建 runner 请求
	runnerReq := make(map[string]any)

	// 从 agent config 中提取 runner 需要的字段
	if models, ok := agentConfig["models"].(map[string]any); ok && len(models) > 0 {
		runnerReq["models"] = models
	}
	if systemPrompt, ok := agentConfig["system_prompt"].(string); ok {
		runnerReq["prompt"] = systemPrompt
	}
	if tools, ok := agentConfig["tools"].([]any); ok && len(tools) > 0 {
		runnerReq["tools"] = tools
	}
	if skills, ok := agentConfig["skills"].([]any); ok && len(skills) > 0 {
		runnerReq["skills"] = skills
	}
	if options, ok := agentConfig["options"].(map[string]any); ok && len(options) > 0 {
		runnerReq["options"] = options
	}

	// 禁用流式响应（飞书不支持 SSE）
	if runnerReq["options"] == nil {
		runnerReq["options"] = map[string]any{}
	}
	if opts, ok := runnerReq["options"].(map[string]any); ok {
		opts["stream"] = false
	}

	if knowledgeBases, ok := agentConfig["knowledge_bases"].([]any); ok && len(knowledgeBases) > 0 {
		runnerReq["knowledge_bases"] = knowledgeBases
	}
	if mcps, ok := agentConfig["mcps"].([]any); ok && len(mcps) > 0 {
		runnerReq["mcps"] = mcps
	}
	if a2a, ok := agentConfig["a2a"].([]any); ok && len(a2a) > 0 {
		runnerReq["a2a"] = a2a
	}
	if sandbox, ok := agentConfig["sandbox"].(map[string]any); ok && len(sandbox) > 0 {
		runnerReq["sandbox"] = sandbox
	}

	// 4. 加载历史消息
	historicalMessages, _ := d.loadHistoricalMessages(context.Background(), sessionId, agentConfig)

	// 5. 加载长期记忆
	memoryContext, _ := d.loadMemoryContext(context.Background(), agentId, ctx.UserID, ctx.Content, agentConfig)

	// 6. 构建 messages 数组
	messages := make([]map[string]any, 0)
	messages = append(messages, memoryContext...)
	messages = append(messages, historicalMessages...)
	messages = append(messages, map[string]any{
		"role":    "user",
		"content": ctx.Content,
	})
	runnerReq["messages"] = messages

	// 7. 获取 uploads 目录
	uploadsDir := os.Getenv("APP_DATA")
	if uploadsDir == "" {
		uploadsDir = "/tmp/xiaoqinglong/data"
	}
	uploadsDir = filepath.Join(uploadsDir, "uploads")

	// 8. 构建 context
	runnerReq["context"] = map[string]any{
		"session_id":  sessionId,
		"user_id":     ctx.UserID,
		"channel_id":  "feishu",
		"agent_id":    agentId,
		"uploads_dir": uploadsDir,
		"memory_svc":  d.memorySvc,
	}

	return runnerReq, nil
}

// saveWSMessageToHistory 保存 WebSocket 消息到历史记录
func (d *ChannelDispatcher) saveWSMessageToHistory(agentId, userId, sessionId, userInput string, respData map[string]any) {
	ctx := context.Background()

	// 保存用户消息
	d.chatSvc.CreateChatMessage(ctx, &chatDto.CreateChatMessageReq{
		SessionId: sessionId,
		Role:      "user",
		Content:   userInput,
	})

	// 保存助手消息
	content := ""
	if respData["Content"] != nil {
		if c, ok := respData["Content"].(string); ok {
			content = c
		}
	}
	if content == "" && respData["content"] != nil {
		if c, ok := respData["content"].(string); ok {
			content = c
		}
	}
	model := ""
	if respData["model"] != nil {
		if m, ok := respData["model"].(string); ok {
			model = m
		}
	}
	d.chatSvc.CreateChatMessage(ctx, &chatDto.CreateChatMessageReq{
		SessionId: sessionId,
		Role:      "assistant",
		Content:   content,
		Model:     model,
	})
}

// parseSSEResponse 解析 SSE 格式响应，提取 content
// SSE 格式: event: completion\ndata: {...}\n
// done 事件格式: event: done\ndata: {"content": "...", ...}
func parseSSEResponse(sseText string) string {
	lines := strings.Split(sseText, "\n")
	var content string

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		// 找到 event: 行后的数据行
		if strings.HasPrefix(line, "event:") {
			eventName := strings.TrimPrefix(line, "event:")
			// 检查下一行
			if i+1 < len(lines) {
				nextLine := strings.TrimSpace(lines[i+1])
				var jsonStr string
				if strings.HasPrefix(nextLine, "data:") {
					jsonStr = strings.TrimPrefix(nextLine, "data:")
				} else {
					jsonStr = nextLine
				}

				// 跳过空行
				if jsonStr == "" {
					continue
				}

				var data map[string]any
				if err := json.Unmarshal([]byte(jsonStr), &data); err == nil {
					// done 事件包含最终的 content
					if eventName == "done" {
						if c, ok := data["content"].(string); ok && c != "" {
							content = c
							break
						}
					}
					// 其他事件也尝试提取 content
					if c, ok := data["content"].(string); ok && c != "" {
						content = c
					}
				}
			}
			continue
		}

		// 处理单个 data: 行的情况（无 event: 前缀）
		if strings.HasPrefix(line, "data:") {
			jsonStr := strings.TrimPrefix(line, "data:")
			if jsonStr == "" {
				continue
			}
			var data map[string]any
			if err := json.Unmarshal([]byte(jsonStr), &data); err == nil {
				// 尝试 Content（大写，runner 返回格式）
				if c, ok := data["Content"].(string); ok && c != "" {
					content = c
				}
				// 也尝试 content（小写）
				if content == "" {
					if c, ok := data["content"].(string); ok && c != "" {
						content = c
					}
				}
				if content != "" {
					break
				}
			}
		}
	}

	return content
}

// HandleDingTalkWSMessage 处理钉钉 WebSocket 消息
func (d *ChannelDispatcher) HandleDingTalkWSMessage(ctx *MessageContext) error {
	log := logger.GetRunnerLogger()
	log.Infof("[Dispatcher] HandleDingTalkWSMessage: sessionID=%s, userID=%s, content=%s",
		ctx.SessionID, ctx.UserID, ctx.Content)

	// 1. 查找或创建 Session
	var agentId string
	session, err := d.chatSvc.FindOrCreateChatSessionByChannel(context.Background(), ctx.UserID, "dingtalk", "")
	if err != nil {
		log.Warnf("[Dispatcher] Failed to find/create session: %v", err)
		return err
	}

	// 如果 session 没有绑定 agent，需要路由选择
	if session.AgentId == "" {
		log.Infof("[Dispatcher] Session has no agent bound, routing...")

		// 2. LLM 路由选择 Agent
		agent, err := d.routeAgentForChannel(ctx.Content, "dingtalk")
		if err != nil {
			log.Warnf("[Dispatcher] Failed to route agent: %v", err)
			// 发送错误消息
			d.sendDingTalkWSText(ctx, "抱歉，暂时没有可用的 Agent 处理您的请求。")
			return err
		}
		agentId = agent.Ulid

		// 3. 更新 Session 绑定 Agent
		err = d.chatSvc.UpdateChatSession(context.Background(), &chatDto.UpdateChatSessionReq{
			Ulid:    session.Ulid,
			AgentId: agentId,
		})
		if err != nil {
			log.Warnf("[Dispatcher] Failed to bind agent to session: %v", err)
		}
		log.Infof("[Dispatcher] Bound agent %s to session %s", agentId, session.Ulid)
	} else {
		agentId = session.AgentId
		log.Infof("[Dispatcher] Session already bound to agent %s", agentId)
	}

	// 4. 构建请求并调用 runner
	return d.runAndRespondDingTalkWS(ctx, agentId, session.Ulid)
}

// runAndRespondDingTalkWS 非 HTTP context 下调用 runner 并发送响应（钉钉）
func (d *ChannelDispatcher) runAndRespondDingTalkWS(ctx *MessageContext, agentId, sessionId string) error {
	log := logger.GetRunnerLogger()

	// 1. 获取 Agent 配置
	agentRsp, err := d.agentSvc.FindSysAgentById(context.Background(), &agentDto.FindSysAgentByIdReq{
		Ulid: agentId,
	})
	if err != nil || agentRsp == nil {
		log.Warnf("[Dispatcher] Agent not found: %s, err=%v", agentId, err)
		d.sendDingTalkWSText(ctx, "抱歉，Agent 未找到。")
		return err
	}

	// 2. 构建 runner 请求
	runnerReq, err := d.buildRunnerRequestDingTalkWS(agentRsp, ctx, agentId, sessionId)
	if err != nil {
		log.Warnf("[Dispatcher] Failed to build runner request: %v", err)
		d.sendDingTalkWSText(ctx, "抱歉，请求构建失败。")
		return err
	}

	// 3. 序列化请求
	runnerBody, err := json.Marshal(runnerReq)
	if err != nil {
		log.Warnf("[Dispatcher] Failed to marshal runner request: %v", err)
		d.sendDingTalkWSText(ctx, "抱歉，请求序列化失败。")
		return err
	}

	// 4. 发送请求到 runner
	runURL := d.runnerURL + "/run"
	req, err := http.NewRequest("POST", runURL, bytes.NewReader(runnerBody))
	if err != nil {
		log.Warnf("[Dispatcher] Failed to create request: %v", err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.WithError(err).Error("[Dispatcher] Failed to call runner")
		d.sendDingTalkWSText(ctx, "抱歉，暂时无法处理您的请求。")
		return err
	}
	defer resp.Body.Close()

	// 5. 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Warnf("[Dispatcher] Failed to read runner response: %v", err)
		d.sendDingTalkWSText(ctx, "抱歉，响应读取失败。")
		return err
	}

	// 6. 解析响应，发送文本
	var respData map[string]any
	if err := json.Unmarshal(respBody, &respData); err != nil {
		// 非 JSON 响应（可能是 SSE 事件流格式）
		respText := string(respBody)
		log.Warnf("[Dispatcher] Response is not JSON, attempting SSE parse: %s", respText[:min(200, len(respText))])

		// 尝试解析 SSE 格式的事件
		content := parseSSEResponse(respText)
		if content != "" {
			if err := d.sendDingTalkWSText(ctx, content); err != nil {
				log.WithError(err).Error("[Dispatcher] Failed to send DingTalk WS text")
				return err
			}
			return nil
		}

		d.sendDingTalkWSText(ctx, "抱歉，暂时无法处理您的请求。")
		return errors.New("failed to parse response")
	}

	if respData["Content"] != nil {
		if content, ok := respData["Content"].(string); ok {
			if err := d.sendDingTalkWSText(ctx, content); err != nil {
				log.WithError(err).Error("[Dispatcher] Failed to send DingTalk WS text")
				return err
			}
		}
	}

	// 7. 保存消息到历史记录
	go d.saveDingTalkWSMessageToHistory(agentId, ctx.UserID, sessionId, ctx.Content, respData)

	return nil
}

// buildRunnerRequestDingTalkWS 为钉钉 WebSocket 上下文构建 runner 请求
func (d *ChannelDispatcher) buildRunnerRequestDingTalkWS(agentRsp *agentDto.FindSysAgentRsp, ctx *MessageContext, agentId, sessionId string) (map[string]any, error) {
	// 1. 解析 Agent config_json
	var agentConfig map[string]any
	if agentRsp.ConfigJson != "" {
		if err := json.Unmarshal([]byte(agentRsp.ConfigJson), &agentConfig); err != nil {
			return nil, errors.New("failed to parse agent config: " + err.Error())
		}
	}

	// 2. 获取 runner URL
	runnerURL := d.runnerURL
	if endpoint, ok := agentConfig["endpoint"].(string); ok && endpoint != "" {
		runnerURL = strings.TrimSuffix(endpoint, "/run")
		runnerURL = strings.TrimSuffix(runnerURL, "/")
	}
	d.runnerURL = runnerURL

	// 3. 构建 runner 请求
	runnerReq := make(map[string]any)

	// 从 agent config 中提取 runner 需要的字段
	if models, ok := agentConfig["models"].(map[string]any); ok && len(models) > 0 {
		runnerReq["models"] = models
	}
	if systemPrompt, ok := agentConfig["system_prompt"].(string); ok {
		runnerReq["prompt"] = systemPrompt
	}
	if tools, ok := agentConfig["tools"].([]any); ok && len(tools) > 0 {
		runnerReq["tools"] = tools
	}
	if skills, ok := agentConfig["skills"].([]any); ok && len(skills) > 0 {
		runnerReq["skills"] = skills
	}
	if options, ok := agentConfig["options"].([]any); ok && len(options) > 0 {
		runnerReq["options"] = options
	}

	// 禁用流式响应（钉钉不支持 SSE）
	if runnerReq["options"] == nil {
		runnerReq["options"] = map[string]any{}
	}
	if opts, ok := runnerReq["options"].(map[string]any); ok {
		opts["stream"] = false
	}

	if knowledgeBases, ok := agentConfig["knowledge_bases"].([]any); ok && len(knowledgeBases) > 0 {
		runnerReq["knowledge_bases"] = knowledgeBases
	}
	if mcps, ok := agentConfig["mcps"].([]any); ok && len(mcps) > 0 {
		runnerReq["mcps"] = mcps
	}
	if a2a, ok := agentConfig["a2a"].([]any); ok && len(a2a) > 0 {
		runnerReq["a2a"] = a2a
	}
	if sandbox, ok := agentConfig["sandbox"].(map[string]any); ok && len(sandbox) > 0 {
		runnerReq["sandbox"] = sandbox
	}

	// 4. 加载历史消息
	historicalMessages, _ := d.loadHistoricalMessages(context.Background(), sessionId, agentConfig)

	// 5. 加载长期记忆
	memoryContext, _ := d.loadMemoryContext(context.Background(), agentId, ctx.UserID, ctx.Content, agentConfig)

	// 6. 构建 messages 数组
	messages := make([]map[string]any, 0)
	messages = append(messages, memoryContext...)
	messages = append(messages, historicalMessages...)
	messages = append(messages, map[string]any{
		"role":    "user",
		"content": ctx.Content,
	})
	runnerReq["messages"] = messages

	// 7. 获取 uploads 目录
	uploadsDir := os.Getenv("APP_DATA")
	if uploadsDir == "" {
		uploadsDir = "/tmp/xiaoqinglong/data"
	}
	uploadsDir = filepath.Join(uploadsDir, "uploads")

	// 8. 构建 context
	runnerReq["context"] = map[string]any{
		"session_id":  sessionId,
		"user_id":     ctx.UserID,
		"channel_id":  "dingtalk",
		"agent_id":    agentId,
		"uploads_dir": uploadsDir,
		"memory_svc":  d.memorySvc,
	}

	return runnerReq, nil
}

// saveDingTalkWSMessageToHistory 保存钉钉 WebSocket 消息到历史记录
func (d *ChannelDispatcher) saveDingTalkWSMessageToHistory(agentId, userId, sessionId, userInput string, respData map[string]any) {
	ctx := context.Background()

	// 保存用户消息
	d.chatSvc.CreateChatMessage(ctx, &chatDto.CreateChatMessageReq{
		SessionId: sessionId,
		Role:      "user",
		Content:   userInput,
	})

	// 保存助手消息
	content := ""
	if respData["Content"] != nil {
		if c, ok := respData["Content"].(string); ok {
			content = c
		}
	}
	if content == "" && respData["content"] != nil {
		if c, ok := respData["content"].(string); ok {
			content = c
		}
	}
	model := ""
	if respData["model"] != nil {
		if m, ok := respData["model"].(string); ok {
			model = m
		}
	}
	d.chatSvc.CreateChatMessage(ctx, &chatDto.CreateChatMessageReq{
		SessionId: sessionId,
		Role:      "assistant",
		Content:   content,
		Model:     model,
	})
}

// sendDingTalkWSText 通过钉钉 WebSocket 发送文本消息
func (d *ChannelDispatcher) sendDingTalkWSText(ctx *MessageContext, text string) error {
	log := logger.GetRunnerLogger()

	if globalDingTalkWSSender == nil {
		log.Warn("[Dispatcher] DingTalk WS sender not set")
		return errors.New("dingtalk ws sender not set")
	}

	// 使用 session_id 作为 receive_id
	err := globalDingTalkWSSender.SendText(context.Background(), ctx.SessionID, "text", text)
	if err != nil {
		log.WithError(err).Error("[Dispatcher] Failed to send DingTalk WS text")
		return err
	}

	log.Infof("[Dispatcher] Sent DingTalk WS text to sessionID=%s, length=%d", ctx.SessionID, len(text))
	return nil
}

// HandleWeWorkWSMessage 处理企业微信 WebSocket 消息
func (d *ChannelDispatcher) HandleWeWorkWSMessage(ctx *MessageContext) error {
	log := logger.GetRunnerLogger()
	log.Infof("[Dispatcher] HandleWeWorkWSMessage: sessionID=%s, userID=%s, content=%s",
		ctx.SessionID, ctx.UserID, ctx.Content)

	// 1. 查找或创建 Session
	var agentId string
	session, err := d.chatSvc.FindOrCreateChatSessionByChannel(context.Background(), ctx.UserID, "wework", "")
	if err != nil {
		log.Warnf("[Dispatcher] Failed to find/create session: %v", err)
		return err
	}

	// 如果 session 没有绑定 agent，需要路由选择
	if session.AgentId == "" {
		log.Infof("[Dispatcher] Session has no agent bound, routing...")

		// 2. LLM 路由选择 Agent
		agent, err := d.routeAgentForChannel(ctx.Content, "wework")
		if err != nil {
			log.Warnf("[Dispatcher] Failed to route agent: %v", err)
			// 发送错误消息
			d.sendWeWorkWSText(ctx, "抱歉，暂时没有可用的 Agent 处理您的请求。")
			return err
		}
		agentId = agent.Ulid

		// 3. 更新 Session 绑定 Agent
		err = d.chatSvc.UpdateChatSession(context.Background(), &chatDto.UpdateChatSessionReq{
			Ulid:    session.Ulid,
			AgentId: agentId,
		})
		if err != nil {
			log.Warnf("[Dispatcher] Failed to bind agent to session: %v", err)
		}
		log.Infof("[Dispatcher] Bound agent %s to session %s", agentId, session.Ulid)
	} else {
		agentId = session.AgentId
		log.Infof("[Dispatcher] Session already bound to agent %s", agentId)
	}

	// 4. 构建请求并调用 runner
	return d.runAndRespondWeWorkWS(ctx, agentId, session.Ulid)
}

// runAndRespondWeWorkWS 非 HTTP context 下调用 runner 并发送响应（企业微信）
func (d *ChannelDispatcher) runAndRespondWeWorkWS(ctx *MessageContext, agentId, sessionId string) error {
	log := logger.GetRunnerLogger()

	// 1. 获取 Agent 配置
	agentRsp, err := d.agentSvc.FindSysAgentById(context.Background(), &agentDto.FindSysAgentByIdReq{
		Ulid: agentId,
	})
	if err != nil || agentRsp == nil {
		log.Warnf("[Dispatcher] Agent not found: %s, err=%v", agentId, err)
		d.sendWeWorkWSText(ctx, "抱歉，Agent 未找到。")
		return err
	}

	// 2. 构建 runner 请求
	runnerReq, err := d.buildRunnerRequestWeWorkWS(agentRsp, ctx, agentId, sessionId)
	if err != nil {
		log.Warnf("[Dispatcher] Failed to build runner request: %v", err)
		d.sendWeWorkWSText(ctx, "抱歉，请求构建失败。")
		return err
	}

	// 3. 序列化请求
	runnerBody, err := json.Marshal(runnerReq)
	if err != nil {
		log.Warnf("[Dispatcher] Failed to marshal runner request: %v", err)
		d.sendWeWorkWSText(ctx, "抱歉，请求序列化失败。")
		return err
	}

	// 4. 发送请求到 runner
	runURL := d.runnerURL + "/run"
	req, err := http.NewRequest("POST", runURL, bytes.NewReader(runnerBody))
	if err != nil {
		log.Warnf("[Dispatcher] Failed to create request: %v", err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.WithError(err).Error("[Dispatcher] Failed to call runner")
		d.sendWeWorkWSText(ctx, "抱歉，暂时无法处理您的请求。")
		return err
	}
	defer resp.Body.Close()

	// 5. 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Warnf("[Dispatcher] Failed to read runner response: %v", err)
		d.sendWeWorkWSText(ctx, "抱歉，响应读取失败。")
		return err
	}

	// 6. 解析响应，发送文本
	var respData map[string]any
	if err := json.Unmarshal(respBody, &respData); err != nil {
		// 非 JSON 响应（可能是 SSE 事件流格式）
		respText := string(respBody)
		log.Warnf("[Dispatcher] Response is not JSON, attempting SSE parse: %s", respText[:min(200, len(respText))])

		// 尝试解析 SSE 格式的事件
		content := parseSSEResponse(respText)
		if content != "" {
			if err := d.sendWeWorkWSText(ctx, content); err != nil {
				log.WithError(err).Error("[Dispatcher] Failed to send WeWork WS text")
				return err
			}
			return nil
		}

		d.sendWeWorkWSText(ctx, "抱歉，暂时无法处理您的请求。")
		return errors.New("failed to parse response")
	}

	if respData["Content"] != nil {
		if content, ok := respData["Content"].(string); ok {
			if err := d.sendWeWorkWSText(ctx, content); err != nil {
				log.WithError(err).Error("[Dispatcher] Failed to send WeWork WS text")
				return err
			}
		}
	}

	// 7. 保存消息到历史记录
	go d.saveWeWorkWSMessageToHistory(agentId, ctx.UserID, sessionId, ctx.Content, respData)

	return nil
}

// buildRunnerRequestWeWorkWS 为企业微信 WebSocket 上下文构建 runner 请求
func (d *ChannelDispatcher) buildRunnerRequestWeWorkWS(agentRsp *agentDto.FindSysAgentRsp, ctx *MessageContext, agentId, sessionId string) (map[string]any, error) {
	// 1. 解析 Agent config_json
	var agentConfig map[string]any
	if agentRsp.ConfigJson != "" {
		if err := json.Unmarshal([]byte(agentRsp.ConfigJson), &agentConfig); err != nil {
			return nil, errors.New("failed to parse agent config: " + err.Error())
		}
	}

	// 2. 获取 runner URL
	runnerURL := d.runnerURL
	if endpoint, ok := agentConfig["endpoint"].(string); ok && endpoint != "" {
		runnerURL = strings.TrimSuffix(endpoint, "/run")
		runnerURL = strings.TrimSuffix(runnerURL, "/")
	}
	d.runnerURL = runnerURL

	// 3. 构建 runner 请求
	runnerReq := make(map[string]any)

	// 从 agent config 中提取 runner 需要的字段
	if models, ok := agentConfig["models"].(map[string]any); ok && len(models) > 0 {
		runnerReq["models"] = models
	}
	if systemPrompt, ok := agentConfig["system_prompt"].(string); ok {
		runnerReq["prompt"] = systemPrompt
	}
	if tools, ok := agentConfig["tools"].([]any); ok && len(tools) > 0 {
		runnerReq["tools"] = tools
	}
	if skills, ok := agentConfig["skills"].([]any); ok && len(skills) > 0 {
		runnerReq["skills"] = skills
	}
	if options, ok := agentConfig["options"].([]any); ok && len(options) > 0 {
		runnerReq["options"] = options
	}

	// 禁用流式响应（企业微信不支持 SSE）
	if runnerReq["options"] == nil {
		runnerReq["options"] = map[string]any{}
	}
	if opts, ok := runnerReq["options"].(map[string]any); ok {
		opts["stream"] = false
	}

	if knowledgeBases, ok := agentConfig["knowledge_bases"].([]any); ok && len(knowledgeBases) > 0 {
		runnerReq["knowledge_bases"] = knowledgeBases
	}
	if mcps, ok := agentConfig["mcps"].([]any); ok && len(mcps) > 0 {
		runnerReq["mcps"] = mcps
	}
	if a2a, ok := agentConfig["a2a"].([]any); ok && len(a2a) > 0 {
		runnerReq["a2a"] = a2a
	}
	if sandbox, ok := agentConfig["sandbox"].(map[string]any); ok && len(sandbox) > 0 {
		runnerReq["sandbox"] = sandbox
	}

	// 4. 加载历史消息
	historicalMessages, _ := d.loadHistoricalMessages(context.Background(), sessionId, agentConfig)

	// 5. 加载长期记忆
	memoryContext, _ := d.loadMemoryContext(context.Background(), agentId, ctx.UserID, ctx.Content, agentConfig)

	// 6. 构建 messages 数组
	messages := make([]map[string]any, 0)
	messages = append(messages, memoryContext...)
	messages = append(messages, historicalMessages...)
	messages = append(messages, map[string]any{
		"role":    "user",
		"content": ctx.Content,
	})
	runnerReq["messages"] = messages

	// 7. 获取 uploads 目录
	uploadsDir := os.Getenv("APP_DATA")
	if uploadsDir == "" {
		uploadsDir = "/tmp/xiaoqinglong/data"
	}
	uploadsDir = filepath.Join(uploadsDir, "uploads")

	// 8. 构建 context
	runnerReq["context"] = map[string]any{
		"session_id":  sessionId,
		"user_id":     ctx.UserID,
		"channel_id":  "wework",
		"agent_id":    agentId,
		"uploads_dir": uploadsDir,
		"memory_svc":  d.memorySvc,
	}

	return runnerReq, nil
}

// saveWeWorkWSMessageToHistory 保存企业微信 WebSocket 消息到历史记录
func (d *ChannelDispatcher) saveWeWorkWSMessageToHistory(agentId, userId, sessionId, userInput string, respData map[string]any) {
	ctx := context.Background()

	// 保存用户消息
	d.chatSvc.CreateChatMessage(ctx, &chatDto.CreateChatMessageReq{
		SessionId: sessionId,
		Role:      "user",
		Content:   userInput,
	})

	// 保存助手消息
	content := ""
	if respData["Content"] != nil {
		if c, ok := respData["Content"].(string); ok {
			content = c
		}
	}
	if content == "" && respData["content"] != nil {
		if c, ok := respData["content"].(string); ok {
			content = c
		}
	}
	model := ""
	if respData["model"] != nil {
		if m, ok := respData["model"].(string); ok {
			model = m
		}
	}
	d.chatSvc.CreateChatMessage(ctx, &chatDto.CreateChatMessageReq{
		SessionId: sessionId,
		Role:      "assistant",
		Content:   content,
		Model:     model,
	})
}

// sendWeWorkWSText 通过企业微信 WebSocket 发送文本消息
func (d *ChannelDispatcher) sendWeWorkWSText(ctx *MessageContext, text string) error {
	log := logger.GetRunnerLogger()

	if globalWeWorkWSSender == nil {
		log.Warn("[Dispatcher] WeWork WS sender not set")
		return errors.New("wework ws sender not set")
	}

	// 使用 session_id 作为 receive_id
	err := globalWeWorkWSSender.SendText(context.Background(), ctx.SessionID, "text", text)
	if err != nil {
		log.WithError(err).Error("[Dispatcher] Failed to send WeWork WS text")
		return err
	}

	log.Infof("[Dispatcher] Sent WeWork WS text to sessionID=%s, length=%d", ctx.SessionID, len(text))
	return nil
}

// HandleWeixinWSMessage 处理微信 WebSocket 消息
func (d *ChannelDispatcher) HandleWeixinWSMessage(ctx *MessageContext) error {
	log := logger.GetRunnerLogger()
	log.Infof("[Dispatcher] HandleWeixinWSMessage: sessionID=%s, userID=%s, content=%s",
		ctx.SessionID, ctx.UserID, ctx.Content)

	// 1. 查找或创建 Session
	var agentId string
	session, err := d.chatSvc.FindOrCreateChatSessionByChannel(context.Background(), ctx.UserID, "weixin", "")
	if err != nil {
		log.Warnf("[Dispatcher] Failed to find/create session: %v", err)
		return err
	}

	// 如果 session 没有绑定 agent，需要路由选择
	if session.AgentId == "" {
		log.Infof("[Dispatcher] Session has no agent bound, routing...")

		// 2. LLM 路由选择 Agent
		agent, err := d.routeAgentForChannel(ctx.Content, "weixin")
		if err != nil {
			log.Warnf("[Dispatcher] Failed to route agent: %v", err)
			// 发送错误消息
			d.sendWeixinWSText(ctx, "抱歉，暂时没有可用的 Agent 处理您的请求。")
			return err
		}
		agentId = agent.Ulid

		// 3. 更新 Session 绑定 Agent
		err = d.chatSvc.UpdateChatSession(context.Background(), &chatDto.UpdateChatSessionReq{
			Ulid:    session.Ulid,
			AgentId: agentId,
		})
		if err != nil {
			log.Warnf("[Dispatcher] Failed to bind agent to session: %v", err)
		}
		log.Infof("[Dispatcher] Bound agent %s to session %s", agentId, session.Ulid)
	} else {
		agentId = session.AgentId
		log.Infof("[Dispatcher] Session already bound to agent %s", agentId)
	}

	// 4. 构建请求并调用 runner
	return d.runAndRespondWeixinWS(ctx, agentId, session.Ulid)
}

// runAndRespondWeixinWS 非 HTTP context 下调用 runner 并发送响应（微信）
func (d *ChannelDispatcher) runAndRespondWeixinWS(ctx *MessageContext, agentId, sessionId string) error {
	log := logger.GetRunnerLogger()

	// 1. 获取 Agent 配置
	agentRsp, err := d.agentSvc.FindSysAgentById(context.Background(), &agentDto.FindSysAgentByIdReq{
		Ulid: agentId,
	})
	if err != nil || agentRsp == nil {
		log.Warnf("[Dispatcher] Agent not found: %s, err=%v", agentId, err)
		d.sendWeixinWSText(ctx, "抱歉，Agent 未找到。")
		return err
	}

	// 2. 构建 runner 请求
	runnerReq, err := d.buildRunnerRequestWeixinWS(agentRsp, ctx, agentId, sessionId)
	if err != nil {
		log.Warnf("[Dispatcher] Failed to build runner request: %v", err)
		d.sendWeixinWSText(ctx, "抱歉，请求构建失败。")
		return err
	}

	// 3. 序列化请求
	runnerBody, err := json.Marshal(runnerReq)
	if err != nil {
		log.Warnf("[Dispatcher] Failed to marshal runner request: %v", err)
		d.sendWeixinWSText(ctx, "抱歉，请求序列化失败。")
		return err
	}

	// 4. 发送请求到 runner
	runURL := d.runnerURL + "/run"
	req, err := http.NewRequest("POST", runURL, bytes.NewReader(runnerBody))
	if err != nil {
		log.Warnf("[Dispatcher] Failed to create request: %v", err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.WithError(err).Error("[Dispatcher] Failed to call runner")
		d.sendWeixinWSText(ctx, "抱歉，暂时无法处理您的请求。")
		return err
	}
	defer resp.Body.Close()

	// 5. 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Warnf("[Dispatcher] Failed to read runner response: %v", err)
		d.sendWeixinWSText(ctx, "抱歉，响应读取失败。")
		return err
	}

	// 6. 解析响应，发送文本
	var respData map[string]any
	if err := json.Unmarshal(respBody, &respData); err != nil {
		// 非 JSON 响应（可能是 SSE 事件流格式）
		respText := string(respBody)
		log.Warnf("[Dispatcher] Response is not JSON, attempting SSE parse: %s", respText[:min(200, len(respText))])

		// 尝试解析 SSE 格式的事件
		content := parseSSEResponse(respText)
		if content != "" {
			if err := d.sendWeixinWSText(ctx, content); err != nil {
				log.WithError(err).Error("[Dispatcher] Failed to send Weixin WS text")
				return err
			}
			return nil
		}

		d.sendWeixinWSText(ctx, "抱歉，暂时无法处理您的请求。")
		return errors.New("failed to parse response")
	}

	if respData["Content"] != nil {
		if content, ok := respData["Content"].(string); ok {
			if err := d.sendWeixinWSText(ctx, content); err != nil {
				log.WithError(err).Error("[Dispatcher] Failed to send Weixin WS text")
				return err
			}
		}
	}

	// 7. 保存消息到历史记录
	go d.saveWeixinWSMessageToHistory(agentId, ctx.UserID, sessionId, ctx.Content, respData)

	return nil
}

// buildRunnerRequestWeixinWS 为微信 WebSocket 上下文构建 runner 请求
func (d *ChannelDispatcher) buildRunnerRequestWeixinWS(agentRsp *agentDto.FindSysAgentRsp, ctx *MessageContext, agentId, sessionId string) (map[string]any, error) {
	// 1. 解析 Agent config_json
	var agentConfig map[string]any
	if agentRsp.ConfigJson != "" {
		if err := json.Unmarshal([]byte(agentRsp.ConfigJson), &agentConfig); err != nil {
			return nil, errors.New("failed to parse agent config: " + err.Error())
		}
	}

	// 2. 获取 runner URL
	runnerURL := d.runnerURL
	if endpoint, ok := agentConfig["endpoint"].(string); ok && endpoint != "" {
		runnerURL = strings.TrimSuffix(endpoint, "/run")
		runnerURL = strings.TrimSuffix(runnerURL, "/")
	}
	d.runnerURL = runnerURL

	// 3. 构建 runner 请求
	runnerReq := make(map[string]any)

	// 从 agent config 中提取 runner 需要的字段
	if models, ok := agentConfig["models"].(map[string]any); ok && len(models) > 0 {
		runnerReq["models"] = models
	}
	if systemPrompt, ok := agentConfig["system_prompt"].(string); ok {
		runnerReq["prompt"] = systemPrompt
	}
	if tools, ok := agentConfig["tools"].([]any); ok && len(tools) > 0 {
		runnerReq["tools"] = tools
	}
	if skills, ok := agentConfig["skills"].([]any); ok && len(skills) > 0 {
		runnerReq["skills"] = skills
	}
	if options, ok := agentConfig["options"].([]any); ok && len(options) > 0 {
		runnerReq["options"] = options
	}

	// 禁用流式响应（微信不支持 SSE）
	if runnerReq["options"] == nil {
		runnerReq["options"] = map[string]any{}
	}
	if opts, ok := runnerReq["options"].(map[string]any); ok {
		opts["stream"] = false
	}

	if knowledgeBases, ok := agentConfig["knowledge_bases"].([]any); ok && len(knowledgeBases) > 0 {
		runnerReq["knowledge_bases"] = knowledgeBases
	}
	if mcps, ok := agentConfig["mcps"].([]any); ok && len(mcps) > 0 {
		runnerReq["mcps"] = mcps
	}
	if a2a, ok := agentConfig["a2a"].([]any); ok && len(a2a) > 0 {
		runnerReq["a2a"] = a2a
	}
	if sandbox, ok := agentConfig["sandbox"].(map[string]any); ok && len(sandbox) > 0 {
		runnerReq["sandbox"] = sandbox
	}

	// 4. 加载历史消息
	historicalMessages, _ := d.loadHistoricalMessages(context.Background(), sessionId, agentConfig)

	// 5. 加载长期记忆
	memoryContext, _ := d.loadMemoryContext(context.Background(), agentId, ctx.UserID, ctx.Content, agentConfig)

	// 6. 构建 messages 数组
	messages := make([]map[string]any, 0)
	messages = append(messages, memoryContext...)
	messages = append(messages, historicalMessages...)
	messages = append(messages, map[string]any{
		"role":    "user",
		"content": ctx.Content,
	})
	runnerReq["messages"] = messages

	// 7. 获取 uploads 目录
	uploadsDir := os.Getenv("APP_DATA")
	if uploadsDir == "" {
		uploadsDir = "/tmp/xiaoqinglong/data"
	}
	uploadsDir = filepath.Join(uploadsDir, "uploads")

	// 8. 构建 context
	runnerReq["context"] = map[string]any{
		"session_id":  sessionId,
		"user_id":     ctx.UserID,
		"channel_id":  "weixin",
		"agent_id":    agentId,
		"uploads_dir": uploadsDir,
		"memory_svc":  d.memorySvc,
	}

	return runnerReq, nil
}

// saveWeixinWSMessageToHistory 保存微信 WebSocket 消息到历史记录
func (d *ChannelDispatcher) saveWeixinWSMessageToHistory(agentId, userId, sessionId, userInput string, respData map[string]any) {
	ctx := context.Background()

	// 保存用户消息
	d.chatSvc.CreateChatMessage(ctx, &chatDto.CreateChatMessageReq{
		SessionId: sessionId,
		Role:      "user",
		Content:   userInput,
	})

	// 保存助手消息
	content := ""
	if respData["Content"] != nil {
		if c, ok := respData["Content"].(string); ok {
			content = c
		}
	}
	if content == "" && respData["content"] != nil {
		if c, ok := respData["content"].(string); ok {
			content = c
		}
	}
	model := ""
	if respData["model"] != nil {
		if m, ok := respData["model"].(string); ok {
			model = m
		}
	}
	d.chatSvc.CreateChatMessage(ctx, &chatDto.CreateChatMessageReq{
		SessionId: sessionId,
		Role:      "assistant",
		Content:   content,
		Model:     model,
	})
}

// sendWeixinWSText 通过微信 WebSocket 发送文本消息
func (d *ChannelDispatcher) sendWeixinWSText(ctx *MessageContext, text string) error {
	log := logger.GetRunnerLogger()

	if globalWeixinWSSender == nil {
		log.Warn("[Dispatcher] Weixin WS sender not set")
		return errors.New("weixin ws sender not set")
	}

	// 使用 session_id 作为 receive_id
	err := globalWeixinWSSender.SendText(context.Background(), ctx.SessionID, "text", text)
	if err != nil {
		log.WithError(err).Error("[Dispatcher] Failed to send Weixin WS text")
		return err
	}

	log.Infof("[Dispatcher] Sent Weixin WS text to sessionID=%s, length=%d", ctx.SessionID, len(text))
	return nil
}

// sendFeishuWSText 通过飞书 WebSocket 发送文本消息
func (d *ChannelDispatcher) sendFeishuWSText(ctx *MessageContext, text string) error {
	log := logger.GetRunnerLogger()

	if globalFeishuWSSender == nil {
		log.Warn("[Dispatcher] Feishu WS sender not set")
		return errors.New("feishu ws sender not set")
	}

	// 使用 chat_id 作为 receive_id
	err := globalFeishuWSSender.SendText(context.Background(), ctx.SessionID, "text", text)
	if err != nil {
		log.WithError(err).Error("[Dispatcher] Failed to send Feishu WS text")
		return err
	}

	log.Infof("[Dispatcher] Sent Feishu WS text to sessionID=%s, length=%d", ctx.SessionID, len(text))
	return nil
}

// FindSysChannelByCode 根据 code 查询渠道
func (d *ChannelDispatcher) FindSysChannelByCode(ctx context.Context, code string) (*channelDto.FindSysChannelRsp, error) {
	ch, err := d.channelSvc.FindSysChannelByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if ch == nil {
		return nil, nil
	}

	return &channelDto.FindSysChannelRsp{
		Ulid:        ch.Ulid,
		CreatedAt:   ch.CreatedAt,
		UpdatedAt:   ch.UpdatedAt,
		CreatedBy:   ch.CreatedBy,
		UpdatedBy:   ch.UpdatedBy,
		Name:        ch.Name,
		Code:        ch.Code,
		Description: ch.Description,
		Icon:        ch.Icon,
		Enabled:     ch.Enabled,
		Sort:        ch.Sort,
		Config:      ch.Config,
	}, nil
}
