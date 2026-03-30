package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func main() {
	http.HandleFunc("/run", handleRun)
	http.HandleFunc("/resume", handleResume)
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

// handleRunStream 处理流式输出
func handleRunStream(w http.ResponseWriter, r *http.Request, req *RunRequest) {
	runner := NewRunner(req)

	// 设置 SSE 头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// 创建 context 用于取消
	ctx := r.Context()

	// 将 []Message 转换为 adk.Message
	messages := make([]adk.Message, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = schema.UserMessage(m.Content)
	}

	checkpointID := ""
	if req.Options != nil {
		checkpointID = req.Options.CheckPointID
	}

	events, err := runner.Run(ctx, messages, adk.WithCheckPointID(checkpointID))
	if err != nil {
		http.Error(w, fmt.Sprintf("Run failed: %v", err), http.StatusInternalServerError)
		return
	}

	for {
		event, ok := events.Next()
		if !ok {
			// 发送结束事件
			fmt.Fprintf(w, "event: done\ndata: {\"done\":true}\n\n")
			flusher.Flush()
			break
		}

		if event.Err != nil {
			// 发送错误事件
			errMsg := fmt.Sprintf("{\"error\":\"%v\"}", event.Err)
			fmt.Fprintf(w, "event: error\ndata: %s\n\n", errMsg)
			flusher.Flush()
			break
		}

		// 处理不同类型的事件
		if event.Action != nil {
			if event.Action.Interrupted != nil {
				// 中断事件 - 包含待审批信息
				data, _ := json.Marshal(map[string]interface{}{
					"type":       "interrupted",
					"data":       event.Action.Interrupted.Data,
					"checkpoint": checkpointID,
				})
				fmt.Fprintf(w, "event: interrupted\ndata: %s\n\n", data)
				flusher.Flush()
			}

			if event.Action.ToolsCall != nil {
				// 工具调用事件
				for _, tc := range event.Action.ToolsCall {
					data, _ := json.Marshal(map[string]interface{}{
						"type":      "tool_call",
						"tool":      tc.Function.Name,
						"arguments": tc.Function.Arguments,
					})
					fmt.Fprintf(w, "event: tool_call\ndata: %s\n\n", data)
					flusher.Flush()
				}
			}
		}

		// 处理消息输出
		if event.Output != nil && event.Output.MessageOutput != nil {
			msg := event.Output.MessageOutput
			if msg.Content != "" {
				data, _ := json.Marshal(map[string]interface{}{
					"type":    "content",
					"content": msg.Content,
				})
				fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
				flusher.Flush()
			}
		}
	}
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
