package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jettjia/XiaoQinglong/runner/memory"
	"github.com/jettjia/XiaoQinglong/runner/pkg/logger"
	"github.com/jettjia/XiaoQinglong/runner/pkg/xqldir"
	"github.com/jettjia/XiaoQinglong/runner/types"
)

// stopFuncs 保存 checkpoint_id -> cancel 函数
var stopFuncs = make(map[string]context.CancelFunc)
var stopMu sync.Mutex

func main() {
	// 初始化统一目录
	xqldir.Init()

	http.HandleFunc("/run", handleRun)
	http.HandleFunc("/agent", handleAgent)
	http.HandleFunc("/resume", handleResume)
	http.HandleFunc("/stop", handleStop)
	logger.GetRunnerLogger().Println("Runner server starting on :18080")
	log.Fatal(http.ListenAndServe(":18080", nil))
}

func handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req types.RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// 展开环境变量
	expandEnvInRequest(&req)

	// 检查是否需要流式输出
	if req.Options != nil && req.Options.Stream {
		handleRunStream(w, r, &req)
		return
	}

	runner := NewRunner(&req)
	resp, err := runner.Run(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("Run failed: %v", err), http.StatusInternalServerError)
		return
	}

	// 同步提取记忆（等待结果，确保在响应中返回）
	if len(req.Messages) >= 2 {
		modelConfig := memory.GetModelConfigForMemory(req.Models)
		logger.GetRunnerLogger().Infof("[Memory Debug] modelConfig: %+v", modelConfig)
		extractor := memory.NewMemoryExtractor(modelConfig)
		if extractor != nil {
			logger.GetRunnerLogger().Infof("[Memory Debug] extractor created successfully")
			// 获取最后一条 user 和 assistant 的内容
			var userInput, assistantOutput string
			for i := len(req.Messages) - 1; i >= 0; i-- {
				if req.Messages[i].Role == "user" && userInput == "" {
					userInput = req.Messages[i].Content
				}
				if req.Messages[i].Role == "assistant" && assistantOutput == "" {
					assistantOutput = req.Messages[i].Content
				}
				if userInput != "" && assistantOutput != "" {
					break
				}
			}
			logger.Infof("[Memory Debug] userInput: %s, assistantOutput: %s", userInput[:min(50, len(userInput))], assistantOutput[:min(50, len(assistantOutput))])
			if userInput != "" && assistantOutput != "" {
				memories, err := extractor.ExtractMemories(r.Context(), userInput, assistantOutput)
				logger.Infof("[Memory Debug] ExtractMemories returned, err=%v, len(memories)=%d", err, len(memories))
				if err != nil {
					logger.Errorf("[Memory] Failed to extract memories: %v", err)
				} else if len(memories) > 0 {
					logger.Infof("[Memory] Extracted %d memories from conversation", len(memories))
					resp.Memories = memories

					// 流结束后，回调保存记忆
					callbackURL := getContextStr(req.Context, "agent_frame_callback_url")
					if callbackURL != "" {
						go extractAndSaveMemoriesCallback(req.Context, callbackURL, req.Models, req.Messages)
					}
				} else {
					logger.Infof("[Memory] No memories extracted (empty result)")
				}
			} else {
				logger.Infof("[Memory Debug] userInput or assistantOutput is empty, skipping")
			}
		} else {
			logger.Infof("[Memory Debug] extractor is nil, modelConfig: %+v", modelConfig)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleAgent 处理 Agent 自主执行请求（LLM 自动规划使用哪些 tools/skills）
func handleAgent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req types.AgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if req.Task == "" {
		http.Error(w, "task is required", http.StatusBadRequest)
		return
	}

	logger.Infof("[Agent] Received task: %s", req.Task)

	// 构建 RunRequest（使用内置的默认配置）
	// 从环境变量或使用默认值
	defaultModel := os.Getenv("DEFAULT_MODEL")
	defaultAPIKey := os.Getenv("DEFAULT_API_KEY")
	defaultAPIBase := os.Getenv("DEFAULT_API_BASE")

	models := make(map[string]types.ModelConfig)
	if defaultModel != "" {
		models["default"] = types.ModelConfig{
			Name:    defaultModel,
			APIKey:  defaultAPIKey,
			APIBase: defaultAPIBase,
		}
	} else {
		// 如果没有配置默认模型，返回错误
		http.Error(w, "no model configured: set DEFAULT_MODEL env or provide models in request", http.StatusBadRequest)
		return
	}

	// 构建 RunRequest
	runReq := &types.RunRequest{
		Prompt:   req.Task,
		Models:   models,
		Messages: []types.Message{},
		Context:  req.Context,
		Options:  &types.RunOptions{},
	}

	// 展开环境变量
	expandEnvInRequest(runReq)

	// 检查是否流式输出
	if req.Stream {
		handleAgentStream(w, r, runReq)
		return
	}

	// 执行
	runner := NewRunner(runReq)
	resp, err := runner.Run(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("Agent run failed: %v", err), http.StatusInternalServerError)
		return
	}

	// 返回 AgentResponse
	agentResp := types.AgentResponse{
		Content:      resp.Content,
		ToolCalls:    resp.ToolCalls,
		TokensUsed:   resp.TokensUsed,
		FinishReason: resp.FinishReason,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agentResp)
}

// handleAgentStream 处理 Agent 流式输出
func handleAgentStream(w http.ResponseWriter, r *http.Request, req *types.RunRequest) {
	sw := newSSEWriter(w)
	write := sw.Write

	// 设置 SSE 头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	sw.flusher.Flush()

	// 设置 checkpoint_id
	if req.Options == nil {
		req.Options = &types.RunOptions{}
	}
	if req.Options.CheckPointID == "" {
		req.Options.CheckPointID = fmt.Sprintf("agent-%d", time.Now().UnixNano())
	}
	checkpointID := req.Options.CheckPointID

	startedAt := time.Now()
	_ = write("meta", map[string]any{
		"started_at":      startedAt.Format(time.RFC3339Nano),
		"stream_protocol": "sse",
		"checkpoint_id":   checkpointID,
	})

	// 创建可取消的上下文
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	runner := NewRunner(req)
	eventsChan, err := runner.RunStream(ctx)
	if err != nil {
		_ = write("error", map[string]any{"error": fmt.Sprintf("run stream failed: %v", err)})
		return
	}

	var out strings.Builder

	for event := range eventsChan {
		switch event.Type {
		case "delta":
			if text, ok := event.Data["text"].(string); ok {
				out.WriteString(text)
			}
			_ = write("delta", event.Data)
		case "tool_call":
			_ = write("tool_call", event.Data)
		case "tool":
			_ = write("tool", event.Data)
		case "interrupted":
			_ = write("interrupted", event.Data)
		case "error":
			_ = write("error", event.Data)
		case "done":
			data := map[string]any{
				"content":     out.String(),
				"finished_at": time.Now().Format(time.RFC3339Nano),
			}
			if v, ok := event.Data["prompt_tokens"].(int); ok {
				data["prompt_tokens"] = v
			}
			if v, ok := event.Data["completion_tokens"].(int); ok {
				data["completion_tokens"] = v
			}
			if v, ok := event.Data["total_tokens"].(int); ok {
				data["total_tokens"] = v
			}
			_ = write("done", data)
		}
	}
}

// sseWriter SSE写入器
type sseWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

func newSSEWriter(w http.ResponseWriter) *sseWriter {
	flusher, _ := w.(http.Flusher)
	return &sseWriter{w: w, flusher: flusher}
}

func (s *sseWriter) Write(event string, data any) error {
	if s.flusher == nil {
		return fmt.Errorf("flusher not available")
	}
	s.w.Write([]byte(fmt.Sprintf("event: %s\n", event)))
	enc := json.NewEncoder(s.w)
	enc.Encode(data)
	s.w.Write([]byte("\n"))
	s.flusher.Flush()
	return nil
}

// handleRunStream 处理流式输出
func handleRunStream(w http.ResponseWriter, r *http.Request, req *types.RunRequest) {
	sw := newSSEWriter(w)
	write := sw.Write

	// 设置 SSE 头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	sw.flusher.Flush()

	// 确保有 Options 并设置 checkpoint_id
	if req.Options == nil {
		req.Options = &types.RunOptions{}
	}
	if req.Options.CheckPointID == "" {
		req.Options.CheckPointID = fmt.Sprintf("run-%d", time.Now().UnixNano())
	}
	checkpointID := req.Options.CheckPointID

	startedAt := time.Now()
	_ = write("meta", map[string]any{
		"started_at":      startedAt.Format(time.RFC3339Nano),
		"stream_protocol": "sse",
		"checkpoint_id":   checkpointID,
	})

	// 创建可取消的上下文
	ctx, cancel := context.WithCancel(r.Context())

	// 保存取消函数到 stopFuncs (同时用 checkpoint_id 和 session_id 作为 key)
	sessionID := ""
	if req.Context != nil {
		if s, ok := req.Context["session_id"].(string); ok {
			sessionID = s
		}
	}
	stopMu.Lock()
	stopFuncs[checkpointID] = cancel
	if sessionID != "" {
		stopFuncs[sessionID] = cancel
	}
	stopMu.Unlock()

	// 确保退出时清理
	defer func() {
		stopMu.Lock()
		delete(stopFuncs, checkpointID)
		if sessionID != "" {
			delete(stopFuncs, sessionID)
		}
		stopMu.Unlock()
	}()

	runner := NewRunner(req)
	eventsChan, err := runner.RunStream(ctx)
	if err != nil {
		_ = write("error", map[string]any{"error": fmt.Sprintf("run stream failed: %v", err)})
		return
	}

	var out strings.Builder

	// heartbeat with idle watchdog
	stopHeartbeat := make(chan struct{})
	idleTimeout := time.NewTimer(90 * time.Second)
	defer idleTimeout.Stop()
	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stopHeartbeat:
				return
			case <-t.C:
				_ = write("meta", map[string]any{"heartbeat_at": time.Now().Format(time.RFC3339Nano)})
			case <-idleTimeout.C:
				// 静默断连 - 90秒无活动
				logger.GetRunnerLogger().Infof("[SSE] idle timeout, closing connection")
				return
			}
		}
	}()

	// 重置 idle 看门狗的函数
	resetIdleWatchdog := func() {
		if !idleTimeout.Stop() {
			select {
			case <-idleTimeout.C:
			default:
			}
		}
		idleTimeout.Reset(90 * time.Second)
	}

	for event := range eventsChan {
		// 每次收到事件，重置 idle 看门狗
		resetIdleWatchdog()

		switch event.Type {
		case "delta":
			if text, ok := event.Data["text"].(string); ok {
				out.WriteString(text)
			}
			_ = write("delta", event.Data)
		case "tool_call":
			_ = write("tool_call", event.Data)
		case "tool":
			_ = write("tool", event.Data)
		case "interrupted":
			_ = write("interrupted", event.Data)
		case "error":
			_ = write("error", event.Data)
		case "done":
			data := map[string]any{
				"content":     out.String(),
				"finished_at": time.Now().Format(time.RFC3339Nano),
			}
			// Include token info from dispatcher
			if v, ok := event.Data["prompt_tokens"].(int); ok {
				data["prompt_tokens"] = v
			}
			if v, ok := event.Data["completion_tokens"].(int); ok {
				data["completion_tokens"] = v
			}
			if v, ok := event.Data["total_tokens"].(int); ok {
				data["total_tokens"] = v
			}
			if v, ok := event.Data["tool_calls_count"].(int); ok {
				data["tool_calls_count"] = v
			}
			_ = write("done", data)
		case "meta":
			// meta 事件已在上方处理，这里忽略
		}
	}

	close(stopHeartbeat)

	// 流结束后，提取记忆并回调保存
	callbackURL := getContextStr(req.Context, "agent_frame_callback_url")
	if callbackURL != "" {
		extractAndSaveMemoriesCallback(req.Context, callbackURL, req.Models, req.Messages)
	}
}

// getContextStr 从 context 中获取字符串值
func getContextStr(context map[string]any, key string) string {
	if context == nil {
		return ""
	}
	if v, ok := context[key].(string); ok {
		return v
	}
	return ""
}

// extractAndSaveMemoriesCallback 从消息中提取记忆并回调保存
func extractAndSaveMemoriesCallback(ctx map[string]any, callbackURL string, models map[string]types.ModelConfig, messages []types.Message) {
	// 获取最后一条 user 和 assistant 的内容
	var userInput, assistantOutput string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" && userInput == "" {
			userInput = messages[i].Content
		}
		if messages[i].Role == "assistant" && assistantOutput == "" {
			assistantOutput = messages[i].Content
		}
		if userInput != "" && assistantOutput != "" {
			break
		}
	}
	if userInput == "" || assistantOutput == "" {
		return
	}

	// 提取记忆
	extractor := memory.NewMemoryExtractor(memory.GetModelConfigForMemory(models))
	if extractor == nil {
		return
	}

	memories, err := extractor.ExtractMemories(context.Background(), userInput, assistantOutput)
	if err != nil || len(memories) == 0 {
		if err != nil {
			logger.GetRunnerLogger().Infof("[Memory] Failed to extract memories: %v", err)
		} else {
			logger.GetRunnerLogger().Infof("[Memory] No memories extracted, userInput=%s", userInput[:min(50, len(userInput))])
		}
		return
	}

	logger.GetRunnerLogger().Infof("[Memory] Extracted %d memories", len(memories))
	for i, m := range memories {
		contentPreview := m.Content
		if len(contentPreview) > 50 {
			contentPreview = contentPreview[:50] + "..."
		}
		logger.GetRunnerLogger().Infof("[Memory]   [%d] name=%s, type=%s, description=%s, content=%s",
			i, m.Name, m.Type, m.Description, contentPreview)
	}

	// 构建回调请求
	callbackReq := map[string]any{
		"agent_id":   getContextStr(ctx, "agent_id"),
		"user_id":    getContextStr(ctx, "user_id"),
		"session_id": getContextStr(ctx, "session_id"),
		"memories":   memories,
	}

	reqBytes, _ := json.Marshal(callbackReq)
	go func() {
		req, err := http.NewRequestWithContext(context.Background(), "POST", callbackURL, bytes.NewReader(reqBytes))
		if err != nil {
			logger.GetRunnerLogger().Infof("[Memory] Failed to create callback request: %v", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			logger.GetRunnerLogger().Infof("[Memory] Failed to send memories callback: %v", err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			logger.GetRunnerLogger().Infof("[Memory] Callback returned status %d", resp.StatusCode)
		} else {
			logger.GetRunnerLogger().Infof("[Memory] Saved %d memories via callback", len(memories))
		}
	}()
}

func handleResume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Decode into map first to detect format
	var rawReq map[string]any
	if err := json.NewDecoder(r.Body).Decode(&rawReq); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Convert to ResumeRequest
	req := types.ResumeRequest{}

	// Handle two formats:
	// 1. New format: {checkpoint_id: "xxx", approvals: [{interrupt_id: "xxx", approved: true}]}
	// 2. Old/direct format: {interrupt_id: "xxx", approved: true, approved_by: "user", reason: "xxx"}
	if checkpointID, ok := rawReq["checkpoint_id"].(string); ok && checkpointID != "" {
		req.CheckPointID = checkpointID
		if approvals, ok := rawReq["approvals"].([]any); ok {
			for _, a := range approvals {
				if approvalMap, ok := a.(map[string]any); ok {
					approval := types.ResumeApproval{}
					if id, ok := approvalMap["interrupt_id"].(string); ok {
						approval.InterruptID = id
					}
					if approved, ok := approvalMap["approved"].(bool); ok {
						approval.Approved = approved
					}
					req.Approvals = append(req.Approvals, approval)
				}
			}
		}
	} else if interruptID, ok := rawReq["interrupt_id"].(string); ok {
		// Old/direct format - interrupt_id is used as checkpoint_id
		req.CheckPointID = interruptID
		approved := true
		if a, ok := rawReq["approved"].(bool); ok {
			approved = a
		}
		req.Approvals = []types.ResumeApproval{{
			InterruptID: interruptID,
			Approved:    approved,
		}}
	}

	runner := NewRunner(&types.RunRequest{})
	resp, err := runner.Resume(r.Context(), &req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Resume failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleStop(w http.ResponseWriter, r *http.Request) {
	logger.GetRunnerLogger().Infof("[handleStop] Received stop request")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		CheckpointID string `json:"checkpoint_id"`
		SessionID    string `json:"session_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.GetRunnerLogger().Infof("[handleStop] Failed to decode request: %v", err)
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	logger.GetRunnerLogger().Infof("[handleStop] checkpoint_id: %s, session_id: %s", req.CheckpointID, req.SessionID)

	// 优先用 checkpoint_id 查找，其次用 session_id
	targetID := req.CheckpointID
	if targetID == "" {
		targetID = req.SessionID
	}

	logger.GetRunnerLogger().Infof("[handleStop] Looking for cancel func with key: %s", targetID)
	stopMu.Lock()
	cancel, ok := stopFuncs[targetID]
	if ok {
		logger.GetRunnerLogger().Infof("[handleStop] Found cancel func for %s, calling it", targetID)
		cancel()
		delete(stopFuncs, targetID)
	} else {
		logger.GetRunnerLogger().Infof("[handleStop] No cancel func found for %s", targetID)
	}
	stopMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"stopped": ok})
}

// expandEnvInRequest 展开请求中的环境变量
func expandEnvInRequest(req *types.RunRequest) {
	// 展开 models 中的环境变量
	for key, model := range req.Models {
		model.Name = expandEnvStr(model.Name)
		model.APIKey = expandEnvStr(model.APIKey)
		model.APIBase = expandEnvStr(model.APIBase)
		req.Models[key] = model
	}
}

// expandEnvStr 展开 ${ENV_VAR} 格式的环境变量
func expandEnvStr(s string) string {
	if s == "" {
		return s
	}
	re := regexp.MustCompile(`\$\{([^}]+)\}`)
	return re.ReplaceAllStringFunc(s, func(match string) string {
		envVar := match[2 : len(match)-1]
		return os.Getenv(envVar)
	})
}

// containsEnvVar 检查字符串是否包含环境变量
func containsEnvVar(s string) bool {
	return strings.Contains(s, "${")
}
