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

// NewChannelDispatcher 创建调度器
func NewChannelDispatcher() *ChannelDispatcher {
	return &ChannelDispatcher{
		inboundHandlers:  make(map[string]InboundHandler),
		outboundHandlers: make(map[string]OutboundHandler),
		runnerURL:        "http://localhost:18080",
		agentSvc:         agentSvc.NewSysAgentService(),
		chatSvc:          chatSvc.NewChatMessageService(),
		channelSvc:       channelSvc.NewSysChannelService(),
		memorySvc:        memorySvc.NewAgentMemorySvc(),
	}
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
