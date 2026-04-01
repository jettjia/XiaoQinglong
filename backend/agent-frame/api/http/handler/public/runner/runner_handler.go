package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	memoryEntity "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/memory"
	memorySvc "github.com/jettjia/xiaoqinglong/agent-frame/domain/srv/memory"
	"github.com/jettjia/xiaoqinglong/agent-frame/pkg/logger"
)

// FileInfo 文件信息
type FileInfo struct {
	Name        string `json:"name"`
	VirtualPath string `json:"virtual_path"`
	Size        int64  `json:"size"`
	Type        string `json:"type"`
}

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
	runnerURL string
	agentSvc  *agentSvc.SysAgentService
	chatSvc   *chatSvc.ChatMessageService
	memorySvc *memorySvc.AgentMemorySvc
}

// NewHandler NewHandler
func NewHandler() *Handler {
	return &Handler{
		runnerURL: "http://localhost:18080", // 默认runner服务地址
		agentSvc:  agentSvc.NewSysAgentService(),
		chatSvc:   chatSvc.NewChatMessageService(),
		memorySvc: memorySvc.NewAgentMemorySvc(),
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
	uploadsDir := os.Getenv("APP_DATA")
	if uploadsDir == "" {
		uploadsDir = "/tmp/xiaoqinglong/data"
	}
	uploadsDir = filepath.Join(uploadsDir, "uploads")

	runnerReq["context"] = map[string]any{
		"session_id":  chatReq.SessionID,
		"user_id":     chatReq.UserID,
		"channel_id":  "web",
		"agent_id":    chatReq.AgentID,
		"uploads_dir": uploadsDir,
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
		// 流式响应：直接转发 SSE 数据
		log.Info("====== RUNNER STREAM RESPONSE ======")
		log.Infof("Status: %d, Content-Type: %s", resp.StatusCode, contentType)

		// 设置流式响应头
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Access-Control-Allow-Origin", "*")

		// 直接将 SSE 数据流式转发给客户端
		// 注意：知识库检索由 Runner 在运行时通过 retrieve_knowledge 工具完成
		c.Status(resp.StatusCode)
		buf := make([]byte, 4096)
		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
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

// loadMemoryContext 加载长期记忆上下文
func (h *Handler) loadMemoryContext(ctx context.Context, agentId, userId, query string, agentConfig map[string]any) ([]map[string]any, error) {
	// 检查是否启用长期记忆
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

	// 获取相关记忆
	memories, err := h.memorySvc.GetRelevantMemories(ctx, agentId, userId, query, maxMemoryCount)
	if err != nil {
		return nil, err
	}

	if len(memories) == 0 {
		return nil, nil
	}

	// 构建记忆上下文消息
	result := make([]map[string]any, 0, len(memories))
	for _, m := range memories {
		result = append(result, map[string]any{
			"role":    "system",
			"content": "[记忆] " + m.Content,
		})
	}

	return result, nil
}

// saveMemoryContext 保存对话产生的记忆
func (h *Handler) saveMemoryContext(ctx context.Context, agentId, userId, sessionId, userInput, assistantOutput string) error {
	// 检查是否启用长期记忆
	// 这里简单处理，实际应该由runner在返回结果中携带要保存的记忆
	// 目前通过简单分词提取关键词作为记忆

	// 提取关键词作为记忆
	words := extractKeywords(userInput + " " + assistantOutput)

	for _, word := range words {
		if len(word) < 2 {
			continue
		}
		memory := &memoryEntity.AgentMemory{
			AgentId:    agentId,
			UserId:     userId,
			SessionId:  sessionId,
			MemoryType: "entity",
			Content:    word,
			Keywords:   word,
			Importance: 1,
		}
		h.memorySvc.CreateMemory(ctx, memory)
	}

	return nil
}

// extractKeywords 简单提取关键词
func extractKeywords(text string) []string {
	var words []string
	var current []rune

	for _, r := range text {
		if r == ' ' || r == ',' || r == '.' || r == '，' || r == '。' || r == '\n' || r == '\t' || r == '！' || r == '？' || r == '?' || r == '!' {
			if len(current) > 0 {
				words = append(words, string(current))
				current = nil
			}
		} else {
			current = append(current, r)
		}
	}

	if len(current) > 0 {
		words = append(words, string(current))
	}

	return words
}

// Upload 文件上传
func (h *Handler) Upload(c *gin.Context) {
	// 1. 获取 session_id
	sessionID := c.PostForm("session_id")
	if sessionID == "" {
		c.JSON(400, gin.H{"error": "session_id is required"})
		return
	}

	// 2. 获取上传目录
	uploadDir := os.Getenv("APP_DATA")
	if uploadDir == "" {
		uploadDir = "/tmp/xiaoqinglong/data"
	}
	uploadDir = filepath.Join(uploadDir, "uploads", sessionID)

	// 3. 创建上传目录
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		c.JSON(500, gin.H{"error": "failed to create upload directory: " + err.Error()})
		return
	}

	// 4. 获取文件
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(400, gin.H{"error": "failed to parse multipart form: " + err.Error()})
		return
	}

	fileHeaders := form.File["files"]
	if len(fileHeaders) == 0 {
		c.JSON(400, gin.H{"error": "no files provided"})
		return
	}

	// 5. 保存文件
	var uploadedFiles []map[string]any
	for _, fh := range fileHeaders {
		dst := filepath.Join(uploadDir, fh.Filename)
		src, err := fh.Open()
		if err != nil {
			c.JSON(500, gin.H{"error": "failed to open uploaded file: " + err.Error()})
			return
		}
		out, err := os.Create(dst)
		if err != nil {
			src.Close()
			c.JSON(500, gin.H{"error": "failed to create destination file: " + err.Error()})
			return
		}
		if _, err := io.Copy(out, src); err != nil {
			src.Close()
			out.Close()
			c.JSON(500, gin.H{"error": "failed to save file: " + err.Error()})
			return
		}
		src.Close()
		out.Close()

		uploadedFiles = append(uploadedFiles, map[string]any{
			"name":         fh.Filename,
			"size":         fh.Size,
			"type":         fh.Header.Get("Content-Type"),
			"virtual_path": fmt.Sprintf("/mnt/uploads/%s/%s", sessionID, fh.Filename),
		})
	}

	c.JSON(200, gin.H{
		"files": uploadedFiles,
		"count": len(uploadedFiles),
	})
}
