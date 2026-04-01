package main

import (
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
)

// stopFuncs 保存 checkpoint_id -> cancel 函数
var stopFuncs = make(map[string]context.CancelFunc)
var stopMu sync.Mutex

func main() {
	http.HandleFunc("/run", handleRun)
	http.HandleFunc("/resume", handleResume)
	http.HandleFunc("/stop", handleStop)
	log.Println("Runner server starting on :18080")
	log.Fatal(http.ListenAndServe(":18080", nil))
}

func handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RunRequest
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
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
func handleRunStream(w http.ResponseWriter, r *http.Request, req *RunRequest) {
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
		req.Options = &RunOptions{}
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

	// heartbeat
	stopHeartbeat := make(chan struct{})
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
			}
		}
	}()

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
			_ = write("done", map[string]any{
				"content":     out.String(),
				"finished_at": time.Now().Format(time.RFC3339Nano),
			})
		case "meta":
			// meta 事件已在上方处理，这里忽略
		}
	}

	close(stopHeartbeat)
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
	req := ResumeRequest{}

	// Handle two formats:
	// 1. New format: {checkpoint_id: "xxx", approvals: [{interrupt_id: "xxx", approved: true}]}
	// 2. Old/direct format: {interrupt_id: "xxx", approved: true, approved_by: "user", reason: "xxx"}
	if checkpointID, ok := rawReq["checkpoint_id"].(string); ok && checkpointID != "" {
		req.CheckPointID = checkpointID
		if approvals, ok := rawReq["approvals"].([]any); ok {
			for _, a := range approvals {
				if approvalMap, ok := a.(map[string]any); ok {
					approval := ResumeApproval{}
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
		req.Approvals = []ResumeApproval{{
			InterruptID: interruptID,
			Approved:    approved,
		}}
	}

	runner := NewRunner(&RunRequest{})
	resp, err := runner.Resume(r.Context(), &req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Resume failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleStop(w http.ResponseWriter, r *http.Request) {
	log.Printf("[handleStop] Received stop request")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		CheckpointID string `json:"checkpoint_id"`
		SessionID    string `json:"session_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[handleStop] Failed to decode request: %v", err)
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("[handleStop] checkpoint_id: %s, session_id: %s", req.CheckpointID, req.SessionID)

	// 优先用 checkpoint_id 查找，其次用 session_id
	targetID := req.CheckpointID
	if targetID == "" {
		targetID = req.SessionID
	}

	log.Printf("[handleStop] Looking for cancel func with key: %s", targetID)
	stopMu.Lock()
	cancel, ok := stopFuncs[targetID]
	if ok {
		log.Printf("[handleStop] Found cancel func for %s, calling it", targetID)
		cancel()
		delete(stopFuncs, targetID)
	} else {
		log.Printf("[handleStop] No cancel func found for %s", targetID)
	}
	stopMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"stopped": ok})
}

// expandEnvInRequest 展开请求中的环境变量
func expandEnvInRequest(req *RunRequest) {
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