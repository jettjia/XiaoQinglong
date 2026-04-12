package main

import (
	"github.com/cloudwego/eino/components/tool"
	"github.com/jettjia/XiaoQinglong/runner/pkg/logger"
	"github.com/jettjia/XiaoQinglong/runner/types"
)

// ShouldWrapForApproval 判断工具是否需要包装审批（free function 版本）
func ShouldWrapForApproval(toolName, riskLevel string, policy *types.ApprovalPolicy) bool {
	if policy == nil {
		return false
	}

	if !policy.Enabled {
		return false
	}

	// 检查是否在白名单中
	for _, name := range policy.AutoApprove {
		if name == toolName {
			return false
		}
	}

	// 检查 risk_level 是否达到阈值
	return ShouldApprove(riskLevel, policy.RiskThreshold)
}

// WrapToolWithApproval 如果需要审批，包装工具（free function 版本）
func WrapToolWithApproval(t tool.InvokableTool, toolName, toolType, riskLevel string, policy *types.ApprovalPolicy) tool.BaseTool {
	if !ShouldWrapForApproval(toolName, riskLevel, policy) {
		return t
	}

	logger.Infof("[ApprovalHelper] Wrapping tool %s (%s) with approval, risk_level=%s", toolName, toolType, riskLevel)
	return NewInvokableApprovableTool(t, toolName, toolType, riskLevel)
}

// BuildToolRiskLevels 构建工具名称到风险级别的映射（free function 版本）
func BuildToolRiskLevels(tools []types.ToolConfig, a2as []types.A2AAgentConfig, mcps []types.MCPConfig) map[string]string {
	riskLevels := make(map[string]string)

	logger.Infof("[ApprovalHelper] BuildToolRiskLevels: tools has %d items", len(tools))
	for _, tc := range tools {
		logger.Infof("[ApprovalHelper]   tool: name=%s, type=%s, risk_level=%s", tc.Name, tc.Type, tc.RiskLevel)
		riskLevels[tc.Name] = tc.RiskLevel
	}

	logger.Infof("[ApprovalHelper] BuildToolRiskLevels: A2A has %d agents", len(a2as))
	for _, cfg := range a2as {
		logger.Infof("[ApprovalHelper]   a2a: name=%s, risk_level=%s", cfg.Name, cfg.RiskLevel)
		riskLevels["a2a"] = cfg.RiskLevel
	}

	logger.Infof("[ApprovalHelper] BuildToolRiskLevels: MCPs has %d configs", len(mcps))
	for _, cfg := range mcps {
		logger.Infof("[ApprovalHelper]   mcp: name=%s, risk_level=%s", cfg.Name, cfg.RiskLevel)
		riskLevels[cfg.Name] = cfg.RiskLevel
	}

	return riskLevels
}
