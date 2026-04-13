package main

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jettjia/XiaoQinglong/runner/pkg/logger"
	"github.com/jettjia/XiaoQinglong/runner/types"
)

// LoopController 管理连续循环执行
type LoopController struct {
	loopID string
	req    *types.LoopRequest
	opts   *types.LoopOptions

	// 累积的消息历史
	accumulatedMessages []types.Message

	// 全局注册表
	controllers map[string]*LoopController
	mu          sync.RWMutex
	stopChan    chan struct{}
	doneChan    chan struct{}
	status      string
}

// getSessionID 获取 session_id
func (c *LoopController) getSessionID() string {
	if c.req.Context != nil {
		if s, ok := c.req.Context["session_id"].(string); ok {
			return s
		}
	}
	return ""
}

// ActiveLoops 全局循环注册表
var ActiveLoops = make(map[string]*LoopController)
var loopsMu sync.RWMutex

// NewLoopController 创建循环控制器
func NewLoopController(req *types.LoopRequest) *LoopController {
	opts := req.Options
	if opts == nil {
		opts = &types.LoopOptions{}
	}
	// 默认值
	if opts.MaxIterations == 0 {
		opts.MaxIterations = 100
	}
	if opts.Interval == "" {
		opts.Interval = "5s"
	}

	return &LoopController{
		loopID:              fmt.Sprintf("loop-%d", time.Now().UnixNano()),
		req:                 req,
		opts:                opts,
		accumulatedMessages: []types.Message{},
		stopChan:            make(chan struct{}),
		doneChan:            make(chan struct{}),
		status:              "running",
	}
}

// GetLoopController 获取循环控制器
func GetLoopController(id string) *LoopController {
	loopsMu.RLock()
	defer loopsMu.RUnlock()
	return ActiveLoops[id]
}

// RegisterLoopController 注册循环控制器
func RegisterLoopController(id string, c *LoopController) {
	loopsMu.Lock()
	defer loopsMu.Unlock()
	ActiveLoops[id] = c
}

// UnregisterLoopController 注销循环控制器
func UnregisterLoopController(id string) {
	loopsMu.Lock()
	defer loopsMu.Unlock()
	delete(ActiveLoops, id)
}

// Run 执行连续循环（非流式）
func (c *LoopController) Run(ctx context.Context) (*types.LoopResponse, error) {
	c.loopID = fmt.Sprintf("loop-%d", time.Now().UnixNano())
	RegisterLoopController(c.loopID, c)

	// 注册到 stopFuncs（支持 checkpoint 停止）
	sessionID := c.getSessionID()
	stopMu.Lock()
	stopFuncs[c.loopID] = func() { c.Stop() }
	if sessionID != "" {
		stopFuncs[sessionID] = func() { c.Stop() }
	}
	stopMu.Unlock()

	// 确保退出时清理
	defer func() {
		stopMu.Lock()
		delete(stopFuncs, c.loopID)
		if sessionID != "" {
			delete(stopFuncs, sessionID)
		}
		stopMu.Unlock()
	}()

	interval := c.parseInterval(c.opts.Interval)
	maxIterations := c.opts.MaxIterations
	iterations := make([]types.LoopIterationResult, 0)
	totalTokens := 0

	for {
		select {
		case <-ctx.Done():
			c.status = "stopped"
			break
		case <-c.stopChan:
			c.status = "stopped"
			break
		default:
		}

		// 检查 maxIterations
		if maxIterations > 0 && len(iterations) >= maxIterations {
			c.status = "completed"
			break
		}

		// 执行单次迭代
		result := c.runSingleIteration(ctx)
		iterations = append(iterations, *result)
		totalTokens += result.TokensUsed

		// 检查停止条件
		if c.opts.StopCondition != "" && c.checkStopCondition(result) {
			result.Done = true
			c.status = "completed"
			break
		}

		// 间隔等待
		if interval > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-c.stopChan:
				c.status = "stopped"
				return nil, nil
			case <-time.After(interval):
			}
		}
	}

	return &types.LoopResponse{
		LoopID:          c.loopID,
		Status:          c.status,
		Iterations:      iterations,
		TotalTokens:     totalTokens,
		TotalIterations: len(iterations),
		FinalContent:    iterations[len(iterations)-1].Content,
	}, nil
}

// RunStream 执行连续循环（流式）
func (c *LoopController) RunStream(ctx context.Context) (<-chan StreamEvent, error) {
	eventsChan := make(chan StreamEvent, 100)

	c.loopID = fmt.Sprintf("loop-%d", time.Now().UnixNano())
	RegisterLoopController(c.loopID, c)

	// 注册到 stopFuncs（支持 checkpoint 停止）
	sessionID := c.getSessionID()
	stopMu.Lock()
	stopFuncs[c.loopID] = func() { c.Stop() }
	if sessionID != "" {
		stopFuncs[sessionID] = func() { c.Stop() }
	}
	stopMu.Unlock()

	go func() {
		defer close(eventsChan)
		defer UnregisterLoopController(c.loopID)

		// 清理 stopFuncs
		stopMu.Lock()
		delete(stopFuncs, c.loopID)
		if sessionID != "" {
			delete(stopFuncs, sessionID)
		}
		stopMu.Unlock()

		interval := c.parseInterval(c.opts.Interval)
		maxIterations := c.opts.MaxIterations
		iterations := make([]types.LoopIterationResult, 0)
		totalTokens := 0
		iteration := 0

		for {
			select {
			case <-ctx.Done():
				eventsChan <- StreamEvent{Type: "error", Data: map[string]any{"error": "context cancelled"}}
				return
			case <-c.stopChan:
				c.status = "stopped"
				eventsChan <- StreamEvent{Type: "loop_stopped", Data: map[string]any{"loop_id": c.loopID}}
				return
			default:
			}

			if maxIterations > 0 && iteration >= maxIterations {
				c.status = "completed"
				break
			}

			iteration++
			eventsChan <- StreamEvent{Type: "iteration_start", Data: map[string]any{"iteration": iteration, "checkpoint_id": c.loopID}}

			result := c.runSingleIterationStream(ctx, eventsChan)
			iterations = append(iterations, *result)
			totalTokens += result.TokensUsed

			eventsChan <- StreamEvent{Type: "iteration_done", Data: map[string]any{
				"iteration":    result.Iteration,
				"content":      result.Content,
				"tool_calls":   result.ToolCalls,
				"finish_reason": result.FinishReason,
				"tokens_used":  result.TokensUsed,
				"done":         result.Done,
			}}

			if c.opts.StopCondition != "" && c.checkStopCondition(result) {
				result.Done = true
				c.status = "completed"
				break
			}

			if interval > 0 {
				select {
				case <-ctx.Done():
					return
				case <-c.stopChan:
					return
				case <-time.After(interval):
				}
			}
		}

		eventsChan <- StreamEvent{
			Type: "loop_done",
			Data: map[string]any{
				"status":           c.status,
				"iterations":      iteration,
				"total_tokens":    totalTokens,
				"final_content":   iterations[len(iterations)-1].Content,
			},
		}
	}()

	return eventsChan, nil
}

// Stop 停止循环
func (c *LoopController) Stop() {
	if c.stopChan != nil {
		close(c.stopChan)
	}
	c.status = "stopped"
}

// runSingleIteration 执行单次迭代
func (c *LoopController) runSingleIteration(ctx context.Context) *types.LoopIterationResult {
	result := &types.LoopIterationResult{}

	req := c.buildRunRequest()
	runner := NewRunner(req)
	resp, err := runner.Run(ctx)

	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.Iteration = len(c.accumulatedMessages) + 1
	result.Content = resp.Content
	result.FinishReason = resp.FinishReason
	if resp.Metadata != nil {
		result.TokensUsed = resp.Metadata.PromptTokens + resp.Metadata.CompletionTokens
	}

	// 累积消息
	c.appendToAccumulatedMessages(resp)

	return result
}

// runSingleIterationStream 执行单次迭代（流式）
func (c *LoopController) runSingleIterationStream(ctx context.Context, eventsChan chan<- StreamEvent) *types.LoopIterationResult {
	result := &types.LoopIterationResult{}

	req := c.buildRunRequest()
	runner := NewRunner(req)
	streamEvents, err := runner.RunStream(ctx)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	var content strings.Builder
	var toolCalls []map[string]any

	for {
		event, ok := <-streamEvents
		if !ok {
			break
		}

		// 收集内容
		if event.Type == "delta" {
			if text, ok := event.Data["text"].(string); ok {
				content.WriteString(text)
			}
		}

		// 收集 tool_call 信息
		if event.Type == "tool_call" {
			toolCalls = append(toolCalls, event.Data)
		}

		// 转发事件
		eventsChan <- event
	}

	result.Iteration = len(c.accumulatedMessages) + 1
	result.Content = content.String()
	result.Done = true // 流式模式假定立即完成

	c.appendToAccumulatedMessages(&DispatchResult{
		Content: result.Content,
	})

	return result
}

// buildRunRequest 构建 RunRequest
func (c *LoopController) buildRunRequest() *types.RunRequest {
	// 添加引导连续执行的 system prompt
	systemPrompt := c.buildLoopSystemPrompt()

	messages := make([]types.Message, 0)
	if len(c.accumulatedMessages) > 0 {
		messages = c.accumulatedMessages
	}
	messages = append(messages, types.Message{
		Role:    "user",
		Content: c.req.Prompt,
	})

	return &types.RunRequest{
		Prompt:   systemPrompt,
		Models:   c.req.Models,
		Context: c.req.Context,
		Skills:   c.req.Skills,
		MCPs:     c.req.MCPs,
		Tools:    c.req.Tools,
		Messages: messages,
		Options: &types.RunOptions{
			CheckPointID: fmt.Sprintf("%s-iter-%d", c.loopID, len(c.accumulatedMessages)+1),
		},
	}
}

// buildLoopSystemPrompt 生成循环模式引导
func (c *LoopController) buildLoopSystemPrompt() string {
	return `You are running in continuous loop mode. Continue executing the task autonomously.

After each tool execution, evaluate:
1. Has the task been completed?
2. Should I use more tools to accomplish the goal?
3. Is there an error that needs to be handled?

When the task is complete, respond with "LOOP_COMPLETE" in your final message.`
}

// parseInterval 解析时间间隔
func (c *LoopController) parseInterval(interval string) time.Duration {
	interval = strings.TrimSpace(interval)
	if interval == "" {
		return 0
	}

	re := regexp.MustCompile(`^(\d+)(s|m|h|d)?$`)
	matches := re.FindStringSubmatch(interval)
	if matches == nil {
		return 0
	}

	value, _ := strconv.Atoi(matches[1])
	unit := matches[2]
	if unit == "" {
		unit = "s"
	}

	switch unit {
	case "s":
		return time.Duration(value) * time.Second
	case "m":
		return time.Duration(value) * time.Minute
	case "h":
		return time.Duration(value) * time.Hour
	case "d":
		return time.Duration(value) * 24 * time.Hour
	default:
		return time.Duration(value) * time.Second
	}
}

// checkStopCondition 检查停止条件
func (c *LoopController) checkStopCondition(result *types.LoopIterationResult) bool {
	if c.opts.StopCondition == "" {
		return false
	}

	// 简单实现：检查内容是否包含 "LOOP_COMPLETE"
	if strings.Contains(result.Content, "LOOP_COMPLETE") {
		return true
	}

	return false
}

// appendToAccumulatedMessages 累积消息用于下一轮
func (c *LoopController) appendToAccumulatedMessages(resp *DispatchResult) {
	maxHistory := 100

	// 添加 user 消息
	c.accumulatedMessages = append(c.accumulatedMessages, types.Message{
		Role:    "user",
		Content: c.req.Prompt,
	})

	// 添加 assistant 消息
	c.accumulatedMessages = append(c.accumulatedMessages, types.Message{
		Role:    "assistant",
		Content: resp.Content,
	})

	// 截断过长的历史
	if len(c.accumulatedMessages) > maxHistory {
		c.accumulatedMessages = c.accumulatedMessages[len(c.accumulatedMessages)-maxHistory:]
	}

	logger.Infof("[Loop] Accumulated %d messages", len(c.accumulatedMessages))
}
