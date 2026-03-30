package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	agentDto "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/agent"
	agentSvc "github.com/jettjia/xiaoqinglong/agent-frame/application/service/agent"
	chatDto "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/chat"
	chatSvc "github.com/jettjia/xiaoqinglong/agent-frame/application/service/chat"
)

// ChatRunReq 前端聊天请求
type ChatRunReq struct {
	AgentID   string `json:"agent_id"`
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	Input     string `json:"input"`
	Files     []any  `json:"files"`
}

// Handler runner代理处理器
type Handler struct {
	runnerURL string
	agentSvc  *agentSvc.SysAgentService
	chatSvc   *chatSvc.ChatMessageService
}

// NewHandler NewHandler
func NewHandler() *Handler {
	return &Handler{
		runnerURL: "http://localhost:18080", // runner服务地址
		agentSvc:  agentSvc.NewSysAgentService(),
		chatSvc:   chatSvc.NewChatMessageService(),
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
	if knowledge, ok := agentConfig["knowledge"].([]any); ok && len(knowledge) > 0 {
		runnerReq["knowledge"] = knowledge
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

	// 4. 加载历史消息并应用context_window策略
	var historicalMessages []map[string]any
	if chatReq.SessionID != "" {
		historicalMessages, err = h.loadHistoricalMessages(c.Request.Context(), chatReq.SessionID, agentConfig)
		if err != nil {
			log.Printf("[Runner Proxy] Failed to load historical messages: %v", err)
		}
	}

	// 构建messages数组：历史消息 + 当前输入
	messages := append(historicalMessages, map[string]any{
		"role":    "user",
		"content": chatReq.Input,
	})
	runnerReq["messages"] = messages

	runnerReq["context"] = map[string]any{
		"session_id": chatReq.SessionID,
		"user_id":    chatReq.UserID,
		"channel_id": "web",
	}

	// 5. 序列化runner请求
	runnerBody, err := json.Marshal(runnerReq)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to build runner request: " + err.Error()})
		return
	}

	// 格式化日志输出
	prettyReq, _ := json.MarshalIndent(runnerReq, "", "  ")
	log.Printf("[Runner Proxy] ===== RUNNER REQUEST =====")
	log.Printf("[Runner Proxy] Agent: %s (%s)", agentRsp.Name, chatReq.AgentID)
	log.Printf("[Runner Proxy] ConfigJson:\n%s", agentRsp.ConfigJson)
	log.Printf("[Runner Proxy] Request Body:\n%s", string(prettyReq))
	log.Printf("[Runner Proxy] ==========================")

	// 5. 转发请求到runner
	req, err := http.NewRequest("POST", h.runnerURL+"/run", bytes.NewReader(runnerBody))
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to create request"})
		return
	}
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[Runner Proxy] Failed to call runner: %v", err)
		c.JSON(502, gin.H{"error": "failed to call runner service: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	// 读取响应body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to read runner response"})
		return
	}

	// 格式化日志输出
	log.Printf("[Runner Proxy] ===== RUNNER RESPONSE =====")
	log.Printf("[Runner Proxy] Status: %d", resp.StatusCode)
	log.Printf("[Runner Proxy] Response Body:\n%s", string(respBody))
	log.Printf("[Runner Proxy] ==========================")

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

	req, err := http.NewRequest("POST", h.runnerURL+"/resume", bytes.NewReader(body))
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to create request"})
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[Runner Proxy] Failed to call runner resume: %v", err)
		c.JSON(502, gin.H{"error": "failed to call runner service"})
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to read runner response"})
		return
	}

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
