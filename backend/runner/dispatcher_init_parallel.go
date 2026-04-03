package main

import (
	"context"
	"sync"

	"github.com/jettjia/XiaoQinglong/runner/pkg/logger"
)

// initParallel 并行初始化独立组件（在 initModels 之后调用）
// 初始化顺序分组：
// - Group 1（必须串行，在最前）：initCompactService（如果有上下文压缩）
// - Group 2（可并行）：initTools, initKnowledgeRetriever, initFileRetrieval, initBuiltinTools
// - Group 3（可并行）：initA2A, initInternalAgents, initSubAgents
// - Group 4（可并行）：initMCPs, initSkills
// - Group 5（必须串行，最后）：initLoopCron, initCLIs
//
// 返回值：(fatalError, warningError)
// - fatalError: 导致必须终止的错误（如 initModels 失败）
// - warningError: 仅记录警告的错误（如 initKnowledgeRetriever 失败）
func (d *Dispatcher) initParallel(ctx context.Context) (fatal error, warning error) {
	// Phase 1: 初始化上下文压缩服务（如果配置了）
	d.initCompactService(ctx)

	// Phase 2: 可并行初始化工具类组件
	type phase2Result struct {
		toolsErr, kbErr, fileErr, builtinErr error
	}
	resultCh2 := make(chan phase2Result, 1)

	go func() {
		var wg sync.WaitGroup
		var mu sync.Mutex
		res := phase2Result{}

		safeCall := func(name string, fn func(context.Context) error) {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := fn(ctx); err != nil {
					mu.Lock()
					switch name {
					case "tools":
						res.toolsErr = err
					case "kb":
						res.kbErr = err
					case "file":
						res.fileErr = err
					case "builtin":
						res.builtinErr = err
					}
					mu.Unlock()
				}
			}()
		}

		safeCall("tools", d.initTools)
		safeCall("kb", d.initKnowledgeRetriever)
		safeCall("file", d.initFileRetrieval)
		safeCall("builtin", d.initBuiltinTools)

		wg.Wait()
		resultCh2 <- res
	}()

	res2 := <-resultCh2
	if res2.toolsErr != nil {
		return res2.toolsErr, nil
	}
	if res2.kbErr != nil {
		logger.Infof("[Dispatcher] Warning: init knowledge retriever failed: %v", res2.kbErr)
	}
	if res2.fileErr != nil {
		logger.Infof("[Dispatcher] Warning: init file retrieval failed: %v", res2.fileErr)
	}
	if res2.builtinErr != nil {
		logger.Infof("[Dispatcher] Warning: init builtin tools failed: %v", res2.builtinErr)
	}

	// Phase 3: 可并行初始化 Agent 类组件
	type phase3Result struct {
		a2aErr, internalErr, subAgentErr error
	}
	resultCh3 := make(chan phase3Result, 1)

	go func() {
		var wg sync.WaitGroup
		var mu sync.Mutex
		res := phase3Result{}

		safeCall := func(name string, fn func(context.Context) error) {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := fn(ctx); err != nil {
					mu.Lock()
					switch name {
					case "a2a":
						res.a2aErr = err
					case "internal":
						res.internalErr = err
					case "subAgent":
						res.subAgentErr = err
					}
					mu.Unlock()
				}
			}()
		}

		safeCall("a2a", d.initA2A)
		safeCall("internal", d.initInternalAgents)
		logger.Infof("[Dispatcher] About to call initSubAgents, SubAgents count in request = %d", len(d.request.SubAgents))
		safeCall("subAgent", d.initSubAgents)

		wg.Wait()
		resultCh3 <- res
	}()

	res3 := <-resultCh3
	if res3.a2aErr != nil {
		return res3.a2aErr, nil
	}
	if res3.internalErr != nil {
		return res3.internalErr, nil
	}
	if res3.subAgentErr != nil {
		logger.Infof("[Dispatcher] Warning: init sub-agents failed: %v", res3.subAgentErr)
	}

	// Phase 4: 可并行初始化扩展组件
	type phase4Result struct {
		mcpErr, skillErr error
	}
	resultCh4 := make(chan phase4Result, 1)

	go func() {
		var wg sync.WaitGroup
		var mu sync.Mutex
		res := phase4Result{}

		safeCall := func(name string, fn func(context.Context) error) {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := fn(ctx); err != nil {
					mu.Lock()
					switch name {
					case "mcp":
						res.mcpErr = err
					case "skill":
						res.skillErr = err
					}
					mu.Unlock()
				}
			}()
		}

		safeCall("mcp", d.initMCPs)
		safeCall("skill", d.initSkills)

		wg.Wait()
		resultCh4 <- res
	}()

	res4 := <-resultCh4
	if res4.mcpErr != nil {
		logger.Infof("[Dispatcher] Warning: init mcps failed: %v", res4.mcpErr)
	}
	if res4.skillErr != nil {
		return res4.skillErr, nil
	}

	// Phase 5: 必须串行执行的最后组件
	if err := d.initLoopCron(ctx); err != nil {
		logger.Infof("[Dispatcher] Warning: init loop cron failed: %v", err)
	}
	if err := d.initCLIs(ctx); err != nil {
		logger.Infof("[Dispatcher] Warning: init CLIs failed: %v", err)
	}

	return nil, nil
}

// Note: initCompactService is already defined in dispatcher.go
