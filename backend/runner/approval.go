package main

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/jettjia/XiaoQinglong/runner/pkg/logger"
)

// ApprovalInfo 审批信息，传递给前端
type ApprovalInfo struct {
	ToolName        string `json:"tool_name"`
	ToolType        string `json:"tool_type"` // http, mcp, skill, a2a
	ArgumentsInJSON string `json:"arguments_in_json"`
	RiskLevel       string `json:"risk_level"`
	Description     string `json:"description"`
}

// ApprovalResult 审批结果
type ApprovalResult struct {
	Approved         bool
	DisapproveReason *string
}

func (ai *ApprovalInfo) String() string {
	return fmt.Sprintf("tool '%s' (%s) interrupted, arguments: %s, waiting for approval (risk_level=%s)",
		ai.ToolName, ai.ToolType, ai.ArgumentsInJSON, ai.RiskLevel)
}

func init() {
	schema.Register[*ApprovalInfo]()
}

// RiskLevelGetter 动态获取风险级别的回调函数
type RiskLevelGetter func(argumentsInJSON string) string

// InvokableApprovableTool 包装工具，添加审批流程
type InvokableApprovableTool struct {
	tool.InvokableTool
	toolName        string
	toolType        string
	riskLevel       string
	riskLevelGetter RiskLevelGetter // 可选的动态风险级别获取器
}

func NewInvokableApprovableTool(baseTool tool.InvokableTool, toolName, toolType, riskLevel string) *InvokableApprovableTool {
	return &InvokableApprovableTool{
		InvokableTool: baseTool,
		toolName:      toolName,
		toolType:      toolType,
		riskLevel:     riskLevel,
	}
}

// NewInvokableApprovableToolWithGetter 创建带动态风险级别获取器的审批包装工具
func NewInvokableApprovableToolWithGetter(baseTool tool.InvokableTool, toolName, toolType, defaultRiskLevel string, getter RiskLevelGetter) *InvokableApprovableTool {
	return &InvokableApprovableTool{
		InvokableTool:   baseTool,
		toolName:        toolName,
		toolType:        toolType,
		riskLevel:       defaultRiskLevel,
		riskLevelGetter: getter,
	}
}

func (t *InvokableApprovableTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return t.InvokableTool.Info(ctx)
}

func (t *InvokableApprovableTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	logger.GetRunnerLogger().Infof("[Approval] InvokableRun called for tool: %s, risk_level: %s", t.toolName, t.riskLevel)

	// 获取实际的风险级别（如果是动态的）
	actualRiskLevel := t.riskLevel
	if t.riskLevelGetter != nil {
		actualRiskLevel = t.riskLevelGetter(argumentsInJSON)
	}

	logger.GetRunnerLogger().Infof("[Approval] actual risk_level: %s, checking threshold...", actualRiskLevel)

	// 检查是否已被中断过（resume 的情况）
	wasInterrupted, _, storedArguments := tool.GetInterruptState[string](ctx)
	if !wasInterrupted {
		// 第一次执行，触发中断等待审批
		return "", tool.StatefulInterrupt(ctx, &ApprovalInfo{
			ToolName:        t.toolName,
			ToolType:        t.toolType,
			ArgumentsInJSON: argumentsInJSON,
			RiskLevel:       actualRiskLevel,
		}, argumentsInJSON)
	}

	// resume 后，检查审批结果
	isResumeTarget, hasData, data := tool.GetResumeContext[*ApprovalResult](ctx)
	if isResumeTarget && hasData {
		if data.Approved {
			return t.InvokableTool.InvokableRun(ctx, storedArguments, opts...)
		}
		if data.DisapproveReason != nil {
			return fmt.Sprintf("tool '%s' disapproved, reason: %s", t.toolName, *data.DisapproveReason), nil
		}
		return fmt.Sprintf("tool '%s' disapproved", t.toolName), nil
	}

	// 继续等待审批
	return "", tool.StatefulInterrupt(ctx, &ApprovalInfo{
		ToolName:        t.toolName,
		ToolType:        t.toolType,
		ArgumentsInJSON: storedArguments,
		RiskLevel:       actualRiskLevel,
	}, storedArguments)
}

// ShouldApprove 根据 risk_level 和 threshold 判断是否需要审批
func ShouldApprove(riskLevel string, threshold string) bool {
	levels := map[string]int{
		"low":    1,
		"medium": 2,
		"high":   3,
	}

	risk := levels[riskLevel]
	thresh := levels[threshold]

	// 如果 risk_level 或 threshold 不在已知级别中，不审批
	if risk == 0 || thresh == 0 {
		return false
	}

	return risk >= thresh
}
