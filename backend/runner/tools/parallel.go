package tools

import (
	"context"
	"sync"
)

// ParallelToolExecutor 并行执行工具的帮助类
type ParallelToolExecutor struct {
	maxWorkers int
}

// NewParallelToolExecutor 创建并行执行器
func NewParallelToolExecutor(maxWorkers int) *ParallelToolExecutor {
	if maxWorkers <= 0 {
		maxWorkers = 8 // 默认最大 8 个并发
	}
	return &ParallelToolExecutor{
		maxWorkers: maxWorkers,
	}
}

// ToolCall 表示一次工具调用
type ToolCall struct {
	Name      string
	Arguments string
	Tool      BaseTool
}

// ToolResult 表示工具执行结果
type ToolResult struct {
	Name    string
	Result  string
	Error   error
	Success bool
}

// ExecuteParallel 并行执行多个只读工具调用
// 只读工具: Glob, Grep, WebSearch, WebFetch, Read (对于读取) 等
// 注意: 这个方法主要用于外部批量调用，对于 ADK 框架内的工具调用，
// 并行化需要在框架层面处理
func (p *ParallelToolExecutor) ExecuteParallel(ctx context.Context, calls []ToolCall) []ToolResult {
	if len(calls) == 0 {
		return nil
	}

	// 过滤出只读工具
	var readonlyCalls []ToolCall
	var writeCalls []ToolCall
	for _, call := range calls {
		if GlobalRegistry.IsReadOnly(call.Name) {
			readonlyCalls = append(readonlyCalls, call)
		} else {
			writeCalls = append(writeCalls, call)
		}
	}

	results := make([]ToolResult, len(calls))
	var wg sync.WaitGroup
	var mu sync.Mutex

	// 限制并发数
	semaphore := make(chan struct{}, p.maxWorkers)

	// 执行只读工具（并行）
	for i, call := range readonlyCalls {
		wg.Add(1)
		go func(idx int, c ToolCall) {
			defer wg.Done()
			semaphore <- struct{}{}        // 获取信号量
			defer func() { <-semaphore }() // 释放信号量

			result := p.executeSingle(ctx, c)
			mu.Lock()
			results[idx] = result
			mu.Unlock()
		}(i, call)
	}

	// 等待只读工具完成
	wg.Wait()

	// 执行写工具（串行，保持顺序）
	readonlyCount := len(readonlyCalls)
	for i, call := range writeCalls {
		results[readonlyCount+i] = p.executeSingle(ctx, call)
	}

	return results
}

// ExecuteSingle 执行单个工具调用
func (p *ParallelToolExecutor) executeSingle(ctx context.Context, call ToolCall) ToolResult {
	result, err := call.Tool.InvokableRun(ctx, call.Arguments)
	if err != nil {
		return ToolResult{
			Name:    call.Name,
			Result:  "",
			Error:   err,
			Success: false,
		}
	}
	return ToolResult{
		Name:    call.Name,
		Result:  result,
		Error:   nil,
		Success: true,
	}
}

// CanParallelize 检查两个工具调用是否可以并行执行
// 规则:
// 1. 两者都必须是只读工具
// 2. 不能访问相同的文件路径（写入冲突）
func CanParallelize(call1, call2 ToolCall) bool {
	// 两者都必须是通过注册中心的只读工具
	if !GlobalRegistry.IsReadOnly(call1.Name) || !GlobalRegistry.IsReadOnly(call2.Name) {
		return false
	}

	// 检查是否有路径冲突
	if hasPathConflict(call1.Name, call1.Arguments, call2.Name, call2.Arguments) {
		return false
	}

	return true
}

// hasPathConflict 检查两个工具调用是否有路径冲突
// 这个简化版本只检查文件名是否相同
func hasPathConflict(name1, args1, name2, args2 string) bool {
	// 对于 Glob/Grep/Read 类工具，提取路径参数进行对比
	// 这里使用简化检查：实际应该解析 JSON 获取 file_path 等参数

	// 对于明显会写入文件的工具，直接返回 false
	writeTools := map[string]bool{
		"Edit":     true,
		"Write":    true,
		"Bash":     true, // Bash 可能修改文件
		"TaskCreate": true,
		"TaskUpdate": true,
		"TodoWrite":  true,
	}

	if writeTools[name1] || writeTools[name2] {
		return true
	}

	// 读取同一文件不算冲突（可以并行读取）
	// 但如果是写入后读取，可能有问题
	// 简化处理：如果工具名称相同且参数类似，认为有潜在冲突
	if name1 == name2 && args1 == args2 {
		return true
	}

	return false
}
