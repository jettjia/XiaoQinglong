package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/jettjia/XiaoQinglong/runner/pkg/logger"
	"github.com/jettjia/XiaoQinglong/runner/pkg/xqldir"
)

// ToolCall 工具调用记录
type ToolCall struct {
	ToolName string    `json:"tool_name"`
	Args     string    `json:"args"`
	Result   string    `json:"result"`
	Duration int64     `json:"duration_ms"`
	Time     time.Time `json:"time"`
}

// ConversationPattern 对话模式
type ConversationPattern struct {
	Name        string      `json:"name"`        // 模式名称
	Description string      `json:"description"` // 模式描述
	ToolSeq     []string    `json:"tool_seq"`    // 工具调用序列
	Frequency   int         `json:"frequency"`   // 出现频率
	Examples    []string    `json:"examples"`    // 示例
	CreatedAt   time.Time   `json:"created_at"`
}

// SkillDraft 技能草稿
type SkillDraft struct {
	Name        string `json:"name"`        // 技能名称
	Description string `json:"description"` // 触发描述
	Trigger     string `json:"trigger"`     // 触发关键词
	Version     string `json:"version"`
	Content     string `json:"content"`      // SKILL.md 内容
	Pattern     *ConversationPattern `json:"pattern,omitempty"` // 来源模式
}

// SkillGenerator 技能生成器
type SkillGenerator struct {
	skillsDir string
	patterns  []ConversationPattern
}

// NewSkillGenerator 创建技能生成器
func NewSkillGenerator() *SkillGenerator {
	return &SkillGenerator{
		skillsDir: xqldir.GetSkillsDir(),
		patterns:  make([]ConversationPattern, 0),
	}
}

// AnalyzeToolCalls 分析工具调用序列，检测可复用模式
func (g *SkillGenerator) AnalyzeToolCalls(toolCalls []ToolCall) *ConversationPattern {
	if len(toolCalls) < 2 {
		return nil
	}

	// 提取工具序列
	var seq []string
	for _, tc := range toolCalls {
		seq = append(seq, tc.ToolName)
	}

	// 检测重复模式
	pattern := g.detectRepeatingPattern(seq)
	if pattern == nil {
		return nil
	}

	// 生成模式名称
	pattern.Name = g.generatePatternName(seq)
	pattern.Description = g.generatePatternDescription(seq)
	pattern.Examples = g.extractExamples(toolCalls, seq)
	pattern.CreatedAt = time.Now()

	g.patterns = append(g.patterns, *pattern)
	return pattern
}

// detectRepeatingPattern 检测重复模式
func (g *SkillGenerator) detectRepeatingPattern(seq []string) *ConversationPattern {
	if len(seq) < 2 {
		return nil
	}

	// 检查是否有连续重复的工具序列
	// 例如: [A, B, C, A, B, C] 表示模式 [A, B, C]
	for patternLen := 1; patternLen <= len(seq)/2; patternLen++ {
		isRepeating := true
		for i := 0; i < len(seq)-patternLen; i++ {
			if seq[i] != seq[i%patternLen] {
				isRepeating = false
				break
			}
		}

		if isRepeating && patternLen > 1 {
			// 找到重复模式
			patternSeq := seq[:patternLen]
			return &ConversationPattern{
				ToolSeq:   patternSeq,
				Frequency: len(seq) / patternLen,
			}
		}
	}

	// 没有重复模式，但工具组合可能值得提取
	// 检查常见组合
	commonSeqs := g.findCommonSequences(seq)
	if len(commonSeqs) > 0 {
		return &ConversationPattern{
			ToolSeq:   commonSeqs,
			Frequency: 1,
		}
	}

	return nil
}

// findCommonSequences 查找常见序列
func (g *SkillGenerator) findCommonSequences(seq []string) []string {
	if len(seq) < 2 {
		return nil
	}

	// 简单的 2-3 个工具的序列检测
	seen := make(map[string]int)
	for i := 0; i < len(seq)-1; i++ {
		for j := i + 2; j <= len(seq) && j-i <= 3; j++ {
			subSeq := strings.Join(seq[i:j], "->")
			seen[subSeq]++
		}
	}

	// 找到出现次数最多的序列
	var maxSeq string
	var maxCount int
	for subSeq, count := range seen {
		if count > maxCount && count >= 2 {
			maxCount = count
			maxSeq = subSeq
		}
	}

	if maxSeq != "" {
		return strings.Split(maxSeq, "->")
	}
	return nil
}

// generatePatternName 生成模式名称
func (g *SkillGenerator) generatePatternName(seq []string) string {
	// 基于工具序列生成名称
	tools := strings.Join(seq, "_")
	return fmt.Sprintf("auto_%s_%d", tools, time.Now().Unix())
}

// generatePatternDescription 生成模式描述
func (g *SkillGenerator) generatePatternDescription(seq []string) string {
	toolList := strings.Join(seq, " → ")
	return fmt.Sprintf("自动检测到的工具调用模式，包含: %s", toolList)
}

// extractExamples 提取示例
func (g *SkillGenerator) extractExamples(toolCalls []ToolCall, seq []string) []string {
	var examples []string
	for i := 0; i <= len(toolCalls)-len(seq); i++ {
		match := true
		for j := 0; j < len(seq); j++ {
			if toolCalls[i+j].ToolName != seq[j] {
				match = false
				break
			}
		}
		if match {
			// 提取参数作为示例
			var args []string
			for j := 0; j < len(seq) && i+j < len(toolCalls); j++ {
				args = append(args, fmt.Sprintf("%s(%s)", seq[j], truncate(toolCalls[i+j].Args, 100)))
			}
			examples = append(examples, strings.Join(args, " → "))
			if len(examples) >= 3 { // 最多3个示例
				break
			}
		}
	}
	return examples
}

// truncate 截断字符串
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// GenerateSkillDraft 生成技能草稿
func (g *SkillGenerator) GenerateSkillDraft(pattern *ConversationPattern) *SkillDraft {
	if pattern == nil {
		return nil
	}

	draft := &SkillDraft{
		Name:        pattern.Name,
		Description: pattern.Description,
		Trigger:     g.generateTrigger(pattern),
		Version:     "0.1.0",
		Pattern:     pattern,
	}

	// 生成 SKILL.md 内容
	draft.Content = g.generateSkillMD(draft)
	return draft
}

// generateTrigger 生成触发关键词
func (g *SkillGenerator) generateTrigger(pattern *ConversationPattern) string {
	// 基于工具名称生成触发词
	var triggers []string
	for _, tool := range pattern.ToolSeq {
		// 提取工具名中的关键词
		words := strings.Split(tool, "_")
		for _, w := range words {
			if len(w) > 3 {
				triggers = append(triggers, w)
			}
		}
	}

	// 合并触发词
	triggerSet := make(map[string]bool)
	for _, t := range triggers {
		triggerSet[strings.ToLower(t)] = true
	}

	var result []string
	for t := range triggerSet {
		result = append(result, t)
	}

	return strings.Join(result, ", ")
}

// generateSkillMD 生成 SKILL.md 内容
func (g *SkillGenerator) generateSkillMD(draft *SkillDraft) string {
	toolSeq := strings.Join(draft.Pattern.ToolSeq, " → ")
	examples := ""
	if len(draft.Pattern.Examples) > 0 {
		examples = "## Examples\n\n" + strings.Join(draft.Pattern.Examples, "\n\n") + "\n\n"
	}

	return fmt.Sprintf(`---
name: %s
description: "%s"
trigger: "%s"
version: "%s"
---

# %s

## Overview

%s

## When to Use

当用户需要执行以下操作序列时触发：
- %s

## Tool Sequence

本技能使用以下工具序列：

%s

## Workflow

1. 执行第一个工具
2. 根据结果决定下一步
3. 继续执行直到完成目标

%s## Notes

- 此技能由自动模式检测生成
- 首次使用前请审核并调整描述
- 可根据实际情况修改工具参数
`,
		draft.Name,
		draft.Description,
		draft.Trigger,
		draft.Version,
		draft.Name,
		draft.Description,
		toolSeq,
		strings.Join(draft.Pattern.ToolSeq, "\n  - "),
		examples,
	)
}

// SaveSkillDraft 保存技能草稿到文件
func (g *SkillGenerator) SaveSkillDraft(draft *SkillDraft) (string, error) {
	if draft == nil {
		return "", fmt.Errorf("draft is nil")
	}

	// 创建技能目录
	skillDir := filepath.Join(g.skillsDir, draft.Name)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return "", fmt.Errorf("create skill dir failed: %w", err)
	}

	// 写入 SKILL.md
	skillMDPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillMDPath, []byte(draft.Content), 0644); err != nil {
		return "", fmt.Errorf("write SKILL.md failed: %w", err)
	}

	return skillDir, nil
}

// ListPatterns 列出所有检测到的模式
func (g *SkillGenerator) ListPatterns() []ConversationPattern {
	return g.patterns
}

// SavePatterns 保存模式到文件
func (g *SkillGenerator) SavePatterns(path string) error {
	data, err := json.MarshalIndent(g.patterns, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadPatterns 从文件加载模式
func (g *SkillGenerator) LoadPatterns(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &g.patterns)
}

// ShouldCreateSkill 判断是否应该创建技能
func (g *SkillGenerator) ShouldCreateSkill(pattern *ConversationPattern) bool {
	if pattern == nil {
		return false
	}

	// 频率阈值：至少出现2次
	if pattern.Frequency < 2 {
		return false
	}

	// 至少2个工具
	if len(pattern.ToolSeq) < 2 {
		return false
	}

	// 检查是否已经为这个模式创建了技能
	existingSkill := filepath.Join(g.skillsDir, pattern.Name)
	if _, err := os.Stat(existingSkill); err == nil {
		return false // 技能已存在
	}

	return true
}

// CreateSkillFromPattern 从模式创建技能
func (g *SkillGenerator) CreateSkillFromPattern(pattern *ConversationPattern) (string, error) {
	if !g.ShouldCreateSkill(pattern) {
		return "", fmt.Errorf("pattern does not meet criteria for skill creation")
	}

	draft := g.GenerateSkillDraft(pattern)
	if draft == nil {
		return "", fmt.Errorf("failed to generate skill draft")
	}

	skillDir, err := g.SaveSkillDraft(draft)
	if err != nil {
		return "", fmt.Errorf("failed to save skill draft: %w", err)
	}

	return skillDir, nil
}

// ============ 安全扫描（复用 memstore 的逻辑）============

// injectPattern 检测提示注入模式
var injectPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)ignore\s+(previous|all)\s+instructions`),
	regexp.MustCompile(`(?i)you\s+are\s+now\s+(a|an)`),
	regexp.MustCompile(`(?i)disregard\s+(previous|all)\s+(instructions?|rules?)`),
	regexp.MustCompile(`\$[A-Z_]+\s*=.*curl|wget`),
	regexp.MustCompile(`authorized_keys`),
}

// scanContent 扫描内容是否安全
func scanSkillContent(content string) bool {
	for _, pattern := range injectPatterns {
		if pattern.MatchString(content) {
			return true
		}
	}
	return false
}

// ============ CreateSkill Tool（让 LLM 主动创建技能）============

// CreateSkillInput create_skill 工具输入
type CreateSkillInput struct {
	Name        string `json:"name"`        // 技能名称（必须，文件系统安全）
	Content     string `json:"content"`     // 完整的 SKILL.md 内容（必须，包含 frontmatter + 正文）
	Category    string `json:"category"`    // 分类（可选）
	Description string `json:"description"` // 简短描述（可选）
}

// CreateSkillTool 主动创建技能工具
// 允许 Agent 在完成任务后主动创建可复用的技能
type CreateSkillTool struct {
	generator *SkillGenerator
}

// NewCreateSkillTool 创建 create_skill 工具
func NewCreateSkillTool() *CreateSkillTool {
	return &CreateSkillTool{
		generator: NewSkillGenerator(),
	}
}

// Info 返回工具信息
func (t *CreateSkillTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	params := schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"name": {
			Type:     schema.String,
			Desc:     "技能名称，只能包含小写字母、数字、点、下划线和连字符。如：data-analysis, my-skill",
			Required: true,
		},
		"content": {
			Type:     schema.String,
			Desc:     "完整的 SKILL.md 内容，包括 frontmatter (name, description, trigger, version) 和正文",
			Required: true,
		},
		"category": {
			Type:     schema.String,
			Desc:     "技能分类目录，可选。如：data-processing, web-automation",
			Required: false,
		},
		"description": {
			Type:     schema.String,
			Desc:     "简短描述，说明这个技能的用途",
			Required: false,
		},
	})

	return &schema.ToolInfo{
		Name:        "create_skill",
		Desc:        "当 Agent 成功完成一个值得复用的工作流程时，调用此工具将其保存为可复用的技能。保存后，下次遇到类似任务可以自动或手动触发使用。",
		ParamsOneOf: params,
	}, nil
}

// InvokableRun 执行创建技能
func (t *CreateSkillTool) InvokableRun(ctx context.Context, argumentsInJSON string, opt ...tool.Option) (string, error) {
	var input CreateSkillInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("parse create_skill input failed: %w", err)
	}

	// 验证必填参数
	if input.Name == "" {
		return "", fmt.Errorf("name is required")
	}
	if input.Content == "" {
		return "", fmt.Errorf("content is required")
	}

	// 验证名称格式（文件系统安全）
	if err := validateSkillName(input.Name); err != nil {
		return "", fmt.Errorf("invalid name: %w", err)
	}

	// 安全扫描内容
	if scanSkillContent(input.Content) {
		logger.GetRunnerLogger().Infof("[CreateSkillTool] content failed security scan, blocking creation")
		return "", fmt.Errorf("skill content failed security scan")
	}

	// 确定存储路径
	skillsDir := xqldir.GetSkillsDir()
	if input.Category != "" {
		skillsDir = filepath.Join(skillsDir, input.Category)
	}

	// 创建技能目录
	skillDir := filepath.Join(skillsDir, input.Name)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return "", fmt.Errorf("create skill dir failed: %w", err)
	}

	// 写入 SKILL.md
	skillMDPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillMDPath, []byte(input.Content), 0644); err != nil {
		return "", fmt.Errorf("write SKILL.md failed: %w", err)
	}

	logger.GetRunnerLogger().Infof("[CreateSkillTool] created skill at: %s", skillDir)

	// 返回成功信息
	result := map[string]any{
		"success":   true,
		"name":      input.Name,
		"path":      skillDir,
		"message":   fmt.Sprintf("Skill '%s' created successfully at %s", input.Name, skillDir),
		"next_step": "下次遇到类似任务时，可使用 load_skill 工具加载此技能",
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// validateSkillName 验证技能名称
func validateSkillName(name string) error {
	if len(name) > 64 {
		return fmt.Errorf("name too long (max 64 chars)")
	}

	// 只能包含小写字母、数字、点、下划线和连字符
	validName := regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	if !validName.MatchString(name) {
		return fmt.Errorf("name can only contain lowercase letters, numbers, dots, underscores and hyphens, and must start with a letter or number")
	}

	// 保留名称检查
	reservedNames := []string{"skills", "auto", "template"}
	for _, reserved := range reservedNames {
		if name == reserved || strings.HasPrefix(name, reserved+"-") {
			return fmt.Errorf("name '%s' is reserved", reserved)
		}
	}

	return nil
}
