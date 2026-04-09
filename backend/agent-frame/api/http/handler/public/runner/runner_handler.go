package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	agentDto "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/agent"
	chatDto "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/chat"
	agentSvc "github.com/jettjia/xiaoqinglong/agent-frame/application/service/agent"
	chatSvc "github.com/jettjia/xiaoqinglong/agent-frame/application/service/chat"
	"github.com/jettjia/xiaoqinglong/agent-frame/config"
	memorySvc "github.com/jettjia/xiaoqinglong/agent-frame/domain/srv/memory"
	"github.com/jettjia/xiaoqinglong/agent-frame/pkg/logger"
	"github.com/jettjia/xiaoqinglong/agent-frame/pkg/xqldir"
)

// ChatRunReq 前端聊天请求
type ChatRunReq struct {
	AgentID   string     `json:"agent_id"`
	UserID    string     `json:"user_id"`
	SessionID string     `json:"session_id"`
	Input     string     `json:"input"`
	Files     []FileInfo `json:"files"`
	IsTest    bool       `json:"is_test"` // 是否测试模式，测试模式不保存消息
}

// Handler runner代理处理器
type Handler struct {
	runnerURL      string
	agentFrameURL string
	agentSvc      *agentSvc.SysAgentService
	chatSvc       *chatSvc.ChatMessageService
	memorySvc     *memorySvc.AgentMemorySvc
}

// NewHandler NewHandler
func NewHandler() *Handler {
	cfg := config.NewConfig()
	return &Handler{
		runnerURL:      "http://localhost:18080", // 默认runner服务地址
		agentFrameURL: fmt.Sprintf("http://localhost:%d", cfg.Server.PublicPort), // agent-frame服务地址，用于runner回调
		agentSvc:      agentSvc.NewSysAgentService(),
		chatSvc:       chatSvc.NewChatMessageService(),
		memorySvc:     memorySvc.NewAgentMemorySvc(),
	}
}

// Run 代理runner的run请求
func (h *Handler) Run(c *gin.Context) {
	var chatReq ChatRunReq
	if err := c.ShouldBindJSON(&chatReq); err != nil {
		c.JSON(400, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	// 1. 获取Agent配置
	agentRsp, err := h.agentSvc.FindSysAgentById(c.Request.Context(), &agentDto.FindSysAgentByIdReq{
		Ulid: chatReq.AgentID,
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to get agent: " + err.Error()})
		return
	}
	if agentRsp == nil {
		c.JSON(404, gin.H{"error": "agent not found"})
		return
	}

	// 2. 解析Agent的config_json
	var agentConfig map[string]any
	if agentRsp.ConfigJson != "" {
		if err := json.Unmarshal([]byte(agentRsp.ConfigJson), &agentConfig); err != nil {
			c.JSON(500, gin.H{"error": "failed to parse agent config: " + err.Error()})
			return
		}
	}

	// 获取 runner endpoint（默认 http://localhost:18080）
	runnerURL := "http://localhost:18080"
	if endpoint, ok := agentConfig["endpoint"].(string); ok && endpoint != "" {
		// 去掉末尾的 /run 如果有的话
		runnerURL = strings.TrimSuffix(endpoint, "/run")
		runnerURL = strings.TrimSuffix(runnerURL, "/")
	}

	logger.GetRunnerLogger().Infof("[Runner Proxy] Runner URL: %s", runnerURL)

	// 3. 构建runner请求
	runnerReq := make(map[string]any)

	// 从agent config中提取runner需要的字段（只添加非空值）
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
	// 传递 knowledge_bases 配置给 runner，供运行时检索使用
	// Runner 会使用 retrieve_knowledge 工具在需要时主动检索知识库
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

	// 添加上传的文件信息
	if len(chatReq.Files) > 0 {
		runnerReq["files"] = chatReq.Files
	}

	// 4. 加载历史消息并应用context_window策略（非测试模式）
	var historicalMessages []map[string]any
	if chatReq.SessionID != "" && !chatReq.IsTest {
		historicalMessages, err = h.loadHistoricalMessages(c.Request.Context(), chatReq.SessionID, agentConfig)
		if err != nil {
			logger.GetRunnerLogger().WithError(err).Error("[Runner Proxy] Failed to load historical messages")
		}
	}

	// 5. 加载长期记忆（非测试模式）
	var memoryContext []map[string]any
	if chatReq.SessionID != "" && chatReq.AgentID != "" && !chatReq.IsTest {
		memoryContext, err = h.loadMemoryContext(c.Request.Context(), chatReq.AgentID, chatReq.UserID, chatReq.Input, agentConfig)
		if err != nil {
			logger.GetRunnerLogger().WithError(err).Error("[Runner Proxy] Failed to load memory context")
		}
	}

	// 6. 构建messages数组：记忆上下文 + 历史消息 + 当前输入
	// 注意：知识库检索由 Runner 在运行时通过 retrieve_knowledge 工具自动完成
	//       不再需要在 agent-frame 层注入知识上下文
	messages := make([]map[string]any, 0)
	messages = append(messages, memoryContext...)
	messages = append(messages, historicalMessages...)

	messages = append(messages, map[string]any{
		"role":    "user",
		"content": chatReq.Input,
	})
	runnerReq["messages"] = messages

	// 获取 uploads 目录的宿主机路径
	uploadsDir := xqldir.GetUploadsDir()

	runnerReq["context"] = map[string]any{
		"session_id":               chatReq.SessionID,
		"user_id":                  chatReq.UserID,
		"channel_id":               "web",
		"agent_id":                 chatReq.AgentID,
		"uploads_dir":              uploadsDir,
		"agent_frame_callback_url": h.agentFrameURL + "/api/xiaoqinglong/agent-frame/v1/runner/memory", // runner回调保存记忆
	}

	// 5. 序列化runner请求
	runnerBody, err := json.Marshal(runnerReq)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to build runner request: " + err.Error()})
		return
	}

	// 格式化日志输出
	prettyReq, _ := json.MarshalIndent(runnerReq, "", "  ")
	log := logger.GetRunnerLogger()
	log.Info("====== RUNNER REQUEST ======")
	log.Infof("Agent: %s (%s)", agentRsp.Name, chatReq.AgentID)
	log.Infof("ConfigJson:\n%s", agentRsp.ConfigJson)
	log.Infof("Request Body:\n%s", string(prettyReq))
	log.Info("============================")

	// 5. 转发请求到runner
	runURL := runnerURL + "/run"
	req, err := http.NewRequest("POST", runURL, bytes.NewReader(runnerBody))
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to create request"})
		return
	}
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log := logger.GetRunnerLogger()
		log.WithError(err).Error("Failed to call runner")
		c.JSON(502, gin.H{"error": "failed to call runner service: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	// 检查是否是流式响应 (SSE)
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/event-stream") {
		// 流式响应：直接转发 SSE 数据，同时缓存完整响应用于后续解析 memories
		log.Info("====== RUNNER STREAM RESPONSE ======")
		log.Infof("Status: %d, Content-Type: %s", resp.StatusCode, contentType)

		// 设置流式响应头
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Access-Control-Allow-Origin", "*")

		// 缓存完整响应并转发 SSE
		var fullResp []byte
		c.Status(resp.StatusCode)
		buf := make([]byte, 4096)
		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				fullResp = append(fullResp, buf[:n]...)
				if _, writeErr := c.Writer.Write(buf[:n]); writeErr != nil {
					break
				}
				c.Writer.Flush()
			}
			if err != nil {
				if err == io.EOF {
					break
				}
				break
			}
		}

		// 流结束后，异步提取记忆（非测试模式）
		if !chatReq.IsTest && chatReq.SessionID != "" {
			go func() {
				// 从数据库获取对话消息
				msgs, err := h.chatSvc.FindChatMessagesBySessionId(context.Background(), &chatDto.FindChatMessagesBySessionIdReq{
					SessionId: chatReq.SessionID,
				})
				if err != nil || len(msgs) < 2 {
					logger.GetRunnerLogger().WithError(err).Warnf("Failed to get messages for memory extraction, sessionID=%s", chatReq.SessionID)
					return
				}
				// 获取最后一条 user 和 assistant 消息
				var lastUserInput, lastAssistantOutput string
				for i := len(msgs) - 1; i >= 0; i-- {
					if msgs[i].Role == "user" && lastUserInput == "" {
						lastUserInput = msgs[i].Content
					}
					if msgs[i].Role == "assistant" && lastAssistantOutput == "" {
						lastAssistantOutput = msgs[i].Content
					}
					if lastUserInput != "" && lastAssistantOutput != "" {
						break
					}
				}
				if lastUserInput != "" && lastAssistantOutput != "" {
					if err := h.memorySvc.ExtractAndSaveMemories(context.Background(), chatReq.AgentID, chatReq.UserID, chatReq.SessionID, lastUserInput, lastAssistantOutput); err != nil {
						logger.GetRunnerLogger().WithError(err).Errorf("Failed to extract memories, sessionID=%s", chatReq.SessionID)
					} else {
						logger.GetRunnerLogger().Infof("Memory extracted successfully for sessionID=%s", chatReq.SessionID)
					}
				}
			}()
		}
		return
	}

	// 非流式响应：读取完整 body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to read runner response"})
		return
	}

	// 格式化日志输出
	log = logger.GetRunnerLogger()
	log.Info("====== RUNNER RESPONSE ======")
	log.Infof("Status: %d", resp.StatusCode)
	log.Infof("Response Body:\n%s", string(respBody))
	log.Info("============================")

	// 6. 非测试模式下，保存 runner 返回的记忆（runner 已用 LLM 提取）
	if !chatReq.IsTest && chatReq.SessionID != "" {
		go func() {
			var respData map[string]any
			if err := json.Unmarshal(respBody, &respData); err != nil {
				logger.GetRunnerLogger().WithError(err).Warnf("Failed to parse runner response for memories, sessionID=%s", chatReq.SessionID)
				return
			}
			if memoriesRaw, ok := respData["memories"].([]any); ok && len(memoriesRaw) > 0 {
				if err := h.saveMemoriesFromRunner(context.Background(), chatReq.AgentID, chatReq.UserID, chatReq.SessionID, memoriesRaw); err != nil {
					logger.GetRunnerLogger().WithError(err).Errorf("Failed to save memories from runner, sessionID=%s", chatReq.SessionID)
				} else {
					logger.GetRunnerLogger().Infof("Saved %d memories from runner, sessionID=%s", len(memoriesRaw), chatReq.SessionID)
				}
			}
		}()
	}

	// 返回响应
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
}

// Resume 代理runner的resume请求
func (h *Handler) Resume(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(400, gin.H{"error": "failed to read request body"})
		return
	}

	log := logger.GetRunnerLogger()
	log.Info("====== RUNNER RESUME REQUEST ======")
	log.Infof("Request Body:\n%s", string(body))
	log.Info("============================")

	req, err := http.NewRequest("POST", h.runnerURL+"/resume", bytes.NewReader(body))
	if err != nil {
		log.WithError(err).Error("Failed to create resume request")
		c.JSON(500, gin.H{"error": "failed to create request"})
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.WithError(err).Error("Failed to call runner resume")
		c.JSON(502, gin.H{"error": "failed to call runner service"})
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.WithError(err).Error("Failed to read resume response")
		c.JSON(500, gin.H{"error": "failed to read runner response"})
		return
	}

	log.Info("====== RUNNER RESUME RESPONSE ======")
	log.Infof("Status: %d", resp.StatusCode)
	log.Infof("Response Body:\n%s", string(respBody))
	log.Info("============================")

	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
}

// Stop 代理runner的stop请求
func (h *Handler) Stop(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(400, gin.H{"error": "failed to read request body"})
		return
	}

	log := logger.GetRunnerLogger()
	log.Info("====== RUNNER STOP REQUEST ======")
	log.Infof("Request Body:\n%s", string(body))
	log.Info("============================")

	// 从 agent config 获取 runner endpoint
	runnerURL := h.runnerURL
	if runnerURL == "" {
		runnerURL = "http://localhost:18080"
	}
	stopURL := runnerURL + "/stop"

	req, err := http.NewRequest("POST", stopURL, bytes.NewReader(body))
	if err != nil {
		log.WithError(err).Error("Failed to create stop request")
		c.JSON(500, gin.H{"error": "failed to create request"})
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.WithError(err).Error("Failed to call runner stop")
		c.JSON(502, gin.H{"error": "failed to call runner service"})
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.WithError(err).Error("Failed to read stop response")
		c.JSON(500, gin.H{"error": "failed to read runner response"})
		return
	}

	log.Info("====== RUNNER STOP RESPONSE ======")
	log.Infof("Status: %d", resp.StatusCode)
	log.Infof("Response Body:\n%s", string(respBody))
	log.Info("============================")

	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
}

// loadHistoricalMessages 加载历史消息并根据context_window策略限制
func (h *Handler) loadHistoricalMessages(ctx context.Context, sessionID string, agentConfig map[string]any) ([]map[string]any, error) {
	// 1. 获取历史消息
	msgs, err := h.chatSvc.FindChatMessagesBySessionId(ctx, &chatDto.FindChatMessagesBySessionIdReq{
		SessionId: sessionID,
	})
	if err != nil {
		return nil, err
	}
	if len(msgs) == 0 {
		return []map[string]any{}, nil
	}

	// 2. 解析context_window配置
	contextWindow := map[string]any{
		"max_rounds": 10, // 默认保留10轮
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

	// 3. 按策略截取消息
	maxRounds := contextWindow["max_rounds"].(int)
	strategy := contextWindow["strategy"].(string)

	if strategy == "sliding_window" {
		// sliding_window: 只保留最近N轮对话（每轮包含user+assistant）
		return buildSlidingWindowMessages(msgs, maxRounds), nil
	}

	// 默认返回所有消息
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

	// 提取所有对话轮次（user + assistant）
	rounds := make([][]*chatDto.ChatMessageRsp, 0)
	currentRound := make([]*chatDto.ChatMessageRsp, 0)

	for _, msg := range msgs {
		if msg.Role == "user" {
			if len(currentRound) > 0 && currentRound[len(currentRound)-1].Role == "assistant" {
				// 开始新轮
				rounds = append(rounds, currentRound)
				currentRound = make([]*chatDto.ChatMessageRsp, 0)
			}
			currentRound = append(currentRound, msg)
		} else if msg.Role == "assistant" {
			currentRound = append(currentRound, msg)
		}
	}

	// 处理最后一轮
	if len(currentRound) > 0 {
		rounds = append(rounds, currentRound)
	}

	// 只保留最近maxRounds轮
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

	// 截取最后maxRounds轮
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

// getStringFromMap 安全地从 map 获取 string
func getStringFromMap(m map[string]any, key string, defaultVal ...string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	if len(defaultVal) > 0 {
		return defaultVal[0]
	}
	return ""
}
