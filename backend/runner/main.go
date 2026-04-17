package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
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

// globalMemStore 全局记忆存储
var globalMemStore = memory.NewMemStore()

// globalBackgroundReviewer 全局后台记忆审查器
// 在主响应发送后才在后台运行，不与主任务竞争模型注意力
var globalBackgroundReviewer *memory.BackgroundReviewer

// init 初始化全局组件
func init() {
	// 初始化后台记忆审查器（默认启用）
	config := &memory.BackgroundReviewConfig{
		Enabled:   true,
		MaxMemory: 10,
	}
	globalBackgroundReviewer = memory.NewBackgroundReviewer(config, globalMemStore)
}

func main() {
	// 设置源码 skills 目录（用于首次初始化时复制）
	// 优先级：环境变量 XQL_SOURCE_SKILLS_DIR > 自动检测 > 同目录 skills
	if envDir := os.Getenv("XQL_SOURCE_SKILLS_DIR"); envDir != "" {
		xqldir.SourceSkillsDir = envDir
	} else {
		// Dev 环境：二进制在 runner/bin/runner，skills 在项目根目录的 skills/
		execPath, _ := os.Executable()
		if execPath != "" {
			// runner/bin/runner -> 项目根目录
			devSkills := filepath.Join(execPath, "..", "..", "..", "skills")
			if _, err := os.Stat(devSkills); err == nil {
				xqldir.SourceSkillsDir = devSkills
			}
		}

		// 如果找不到，尝试同目录（Windows 部署时 runner.exe 和 skills 同目录）
		if xqldir.SourceSkillsDir == "" {
			execPath, _ := os.Executable()
			if execPath != "" {
				execDir := filepath.Dir(execPath)
				localSkills := filepath.Join(execDir, "skills")
				if _, err := os.Stat(localSkills); err == nil {
					xqldir.SourceSkillsDir = localSkills
				}
			}
		}
	}

	// 初始化统一目录
	xqldir.Init()

	// 静态文件服务：/reports/ 映射到 ~/.xiaoqinglong/data/reports/
	reportsDir := filepath.Join(xqldir.GetReportsDir())
	if err := os.MkdirAll(reportsDir, 0755); err != nil {
		log.Printf("Warning: failed to create reports dir: %v", err)
	} else {
		http.Handle("/reports/", http.StripPrefix("/reports/", http.FileServer(http.Dir(reportsDir))))
		log.Printf("Reports static server enabled: /reports/ -> %s", reportsDir)
	}

	http.HandleFunc("/run", handleRun)
	http.HandleFunc("/agent", handleAgent)
	http.HandleFunc("/resume", handleResume)
	http.HandleFunc("/stop", handleStop)
	logger.GetRunnerLogger().Infof("Runner server starting on :18080")
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

	// 检查是否启用循环模式
	if req.Options != nil && req.Options.LoopInterval != "" {
		handleRunLoop(w, r, &req)
		return
	}

	runner := NewRunner(&req)
	resp, err := runner.Run(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("Run failed: %v", err), http.StatusInternalServerError)
		return
	}

	// 后台提取记忆（不阻塞主流程，参考 Hermes-agent 的 _spawn_background_review 模式）
	triggerBackgroundReview(r.Context(), req.Messages, req.Models, req.Context)

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

	// 流结束后，后台提取记忆（不阻塞，参考 Hermes-agent 的 _spawn_background_review 模式）
	triggerBackgroundReview(context.Background(), req.Messages, req.Models, req.Context)
}

// triggerBackgroundReview 触发后台记忆审查
// 参考 Hermes-agent 的 _spawn_background_review 模式
func triggerBackgroundReview(ctx context.Context, messages []types.Message, models map[string]types.ModelConfig, contextData map[string]any) {
	if globalBackgroundReviewer == nil || len(messages) < 2 {
		return
	}

	modelConfig := memory.GetModelConfigForMemory(models)
	if modelConfig == nil || modelConfig.APIKey == "" {
		return
	}

	globalBackgroundReviewer.UpdateModelConfig(modelConfig)

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

	if userInput != "" && assistantOutput != "" {
		globalBackgroundReviewer.ReviewIfNeeded(ctx, userInput, assistantOutput)
		logger.Infof("[Memory] Background review triggered for session: %s", getContextStr(contextData, "session_id"))
	}
}

// getContextStr 从 context 中获取字符串值
func getContextStr(contextData map[string]any, key string) string {
	if contextData == nil {
		return ""
	}
	if v, ok := contextData[key].(string); ok {
		return v
	}
	return ""
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

// handleRunLoop 处理连续循环执行（复用 /run 端点）
func handleRunLoop(w http.ResponseWriter, r *http.Request, req *types.RunRequest) {
	opts := req.Options

	// 构建 LoopRequest
	loopReq := &types.LoopRequest{
		Prompt:   req.Prompt,
		Models:   req.Models,
		Context:  req.Context,
		Skills:   req.Skills,
		MCPs:     req.MCPs,
		Tools:    req.Tools,
		Options: &types.LoopOptions{
			Interval:      opts.LoopInterval,
			MaxIterations: opts.LoopMaxIterations,
			StopCondition: opts.LoopStopCondition,
			Stream:        opts.Stream,
		},
	}

	// 创建循环控制器
	controller := NewLoopController(loopReq)

	// 执行循环
	if opts.Stream {
		// 流式输出
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		eventsChan, err := controller.RunStream(r.Context())
		if err != nil {
			fmt.Fprintf(w, "data: {\"error\": \"%v\"}\n\n", err)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "SSE not supported", http.StatusInternalServerError)
			return
		}

		for event := range eventsChan {
			data, _ := json.Marshal(event.Data)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, string(data))
			flusher.Flush()
		}

		// 流结束后，后台提取记忆（非阻塞）
		go func() {
			triggerBackgroundReview(context.Background(), []types.Message{}, req.Models, req.Context)
		}()
	} else {
		// 非流式输出
		resp, err := controller.Run(r.Context())
		if err != nil {
			http.Error(w, fmt.Sprintf("Loop execution failed: %v", err), http.StatusInternalServerError)
			return
		}

		// 后台提取记忆（非阻塞）
		go func() {
			triggerBackgroundReview(context.Background(), []types.Message{}, req.Models, req.Context)
		}()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

