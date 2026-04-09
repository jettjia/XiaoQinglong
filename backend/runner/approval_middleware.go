package main

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/jettjia/XiaoQinglong/runner/pkg/logger"
)

// ========== Approval Middleware for Tool Calls ==========

// approvalToolMiddleware 拦截工具调用，实现人工审批
type approvalToolMiddleware struct {
	toolRiskLevels map[string]string // tool name -> risk level
}

// newApprovalToolMiddleware 创建审批中间件
func newApprovalToolMiddleware(toolRiskLevels map[string]string) *approvalToolMiddleware {
	logger.GetRunnerLogger().Printf("[ApprovalMiddleware] Created with %d tools", len(toolRiskLevels))
	for k, v := range toolRiskLevels {
		logger.GetRunnerLogger().Printf("[ApprovalMiddleware]   - %s: %s", k, v)
	}
	return &approvalToolMiddleware{
		toolRiskLevels: toolRiskLevels,
	}
}

// Wrap 包装工具调用，添加审批拦截
func (m *approvalToolMiddleware) Wrap(endpoint compose.InvokableToolEndpoint) compose.InvokableToolEndpoint {
	logger.GetRunnerLogger().Printf("[ApprovalMiddleware] Wrap called!")
	return func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
		toolName := input.Name

		// 查找风险级别
		riskLevel := ""
		if m.toolRiskLevels != nil {
			riskLevel = m.toolRiskLevels[toolName]
		}

		logger.GetRunnerLogger().Printf("[ApprovalMiddleware] tool=%s, risk_level=%s", toolName, riskLevel)

		// 如果没有风险级别或风险级别不需要审批，直接执行
		if riskLevel == "" || !shouldApproveByRiskLevel(riskLevel) {
			logger.GetRunnerLogger().Printf("[ApprovalMiddleware] tool=%s skipped (no approval needed)", toolName)
			return endpoint(ctx, input)
		}

		logger.GetRunnerLogger().Printf("[ApprovalMiddleware] tool=%s requires approval (risk=%s)", toolName, riskLevel)

		// 检查是否已被中断过（resume 的情况）
		wasInterrupted, _, storedArgs := tool.GetInterruptState[string](ctx)
		if !wasInterrupted {
			// 第一次执行，触发中断等待审批
			logger.GetRunnerLogger().Printf("[ApprovalMiddleware] tool=%s triggering interrupt", toolName)
			approvalInfo := &ApprovalInfo{
				ToolName:        toolName,
				ToolType:        "http",
				ArgumentsInJSON: input.Arguments,
				RiskLevel:       riskLevel,
			}
			tool.StatefulInterrupt(ctx, approvalInfo, input.Arguments)
			// 返回特殊错误，通知框架发生了中断
			return nil, &ApprovalInterruptError{
				ToolName:     toolName,
				RiskLevel:    riskLevel,
				ApprovalInfo: approvalInfo,
			}
		}

		// resume 后，检查审批结果
		isTarget, hasData, data := tool.GetResumeContext[*ApprovalResult](ctx)
		if isTarget && hasData {
			if data.Approved {
				logger.GetRunnerLogger().Printf("[ApprovalMiddleware] tool=%s approved, executing", toolName)
				// 使用存储的参数执行
				input.Arguments = storedArgs
				return endpoint(ctx, input)
			}
			if data.DisapproveReason != nil {
				return &compose.ToolOutput{
					Result: fmt.Sprintf("tool '%s' disapproved, reason: %s", toolName, *data.DisapproveReason),
				}, nil
			}
			return &compose.ToolOutput{
				Result: fmt.Sprintf("tool '%s' disapproved", toolName),
			}, nil
		}

		// 继续等待审批
		logger.GetRunnerLogger().Printf("[ApprovalMiddleware] tool=%s continuing to wait for approval", toolName)
		approvalInfo := &ApprovalInfo{
			ToolName:        toolName,
			ToolType:        "http",
			ArgumentsInJSON: storedArgs,
			RiskLevel:       riskLevel,
		}
		tool.StatefulInterrupt(ctx, approvalInfo, storedArgs)

		return nil, &ApprovalInterruptError{
			ToolName:     toolName,
			RiskLevel:    riskLevel,
			ApprovalInfo: approvalInfo,
		}
	}
}

// ApprovalInterruptError 用于通知框架发生了审批中断
type ApprovalInterruptError struct {
	ToolName     string
	RiskLevel    string
	ApprovalInfo *ApprovalInfo
}

func (e *ApprovalInterruptError) Error() string {
	return fmt.Sprintf("tool '%s' interrupted, waiting for approval (risk=%s)", e.ToolName, e.RiskLevel)
}

// shouldApproveByRiskLevel 根据 risk_level 判断是否需要审批
var approvalThreshold = "medium"

func SetApprovalThreshold(threshold string) {
	approvalThreshold = threshold
}

func shouldApproveByRiskLevel(riskLevel string) bool {
	levels := map[string]int{
		"low":    1,
		"medium": 2,
		"high":   3,
	}

	risk := levels[riskLevel]
	thresh := levels[approvalThreshold]

	if risk == 0 || thresh == 0 {
		return false
	}

	return risk >= thresh
}
