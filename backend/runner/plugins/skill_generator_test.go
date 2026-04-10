package plugins

import (
	"testing"
	"time"
)

// TestSkillGenerator_AnalyzeToolCalls 测试工具调用分析
func TestSkillGenerator_AnalyzeToolCalls(t *testing.T) {
	gen := NewSkillGenerator()

	// 模拟重复的工具调用序列
	// [file_reader -> data_analyzer -> report_generator] x 3
	toolCalls := []ToolCall{
		{ToolName: "file_reader", Args: "{}", Time: time.Now()},
		{ToolName: "data_analyzer", Args: "{}", Time: time.Now()},
		{ToolName: "report_generator", Args: "{}", Time: time.Now()},
		{ToolName: "file_reader", Args: "{}", Time: time.Now()},
		{ToolName: "data_analyzer", Args: "{}", Time: time.Now()},
		{ToolName: "report_generator", Args: "{}", Time: time.Now()},
		{ToolName: "file_reader", Args: "{}", Time: time.Now()},
		{ToolName: "data_analyzer", Args: "{}", Time: time.Now()},
		{ToolName: "report_generator", Args: "{}", Time: time.Now()},
	}

	pattern := gen.AnalyzeToolCalls(toolCalls)
	if pattern == nil {
		t.Fatal("Expected to detect pattern, got nil")
	}

	t.Logf("Detected pattern: %s", pattern.Name)
	t.Logf("Tool sequence: %v", pattern.ToolSeq)
	t.Logf("Frequency: %d", pattern.Frequency)

	if len(pattern.ToolSeq) != 3 {
		t.Errorf("Expected 3 tools in sequence, got %d", len(pattern.ToolSeq))
	}

	if pattern.Frequency != 3 {
		t.Errorf("Expected frequency 3, got %d", pattern.Frequency)
	}
}

// TestSkillGenerator_ShouldCreateSkill 测试是否应该创建技能
func TestSkillGenerator_ShouldCreateSkill(t *testing.T) {
	gen := NewSkillGenerator()

	// 测试频率不足的情况
	lowFreqPattern := &ConversationPattern{
		Name:      "test_pattern",
		ToolSeq:   []string{"tool_a", "tool_b"},
		Frequency: 1,
	}
	if gen.ShouldCreateSkill(lowFreqPattern) {
		t.Error("Should not create skill with frequency 1")
	}

	// 测试频率足够的情况
	highFreqPattern := &ConversationPattern{
		Name:      "auto_tool_a_tool_b_123",
		ToolSeq:   []string{"tool_a", "tool_b"},
		Frequency: 3,
	}
	if !gen.ShouldCreateSkill(highFreqPattern) {
		t.Error("Should create skill with frequency >= 2")
	}

	// 测试单个工具的情况
	singleToolPattern := &ConversationPattern{
		Name:      "single_tool",
		ToolSeq:   []string{"tool_a"},
		Frequency: 5,
	}
	if gen.ShouldCreateSkill(singleToolPattern) {
		t.Error("Should not create skill with only 1 tool")
	}
}

// TestSkillGenerator_GenerateSkillDraft 测试生成技能草稿
func TestSkillGenerator_GenerateSkillDraft(t *testing.T) {
	gen := NewSkillGenerator()

	pattern := &ConversationPattern{
		Name:        "auto_data_pipeline",
		ToolSeq:     []string{"file_reader", "data_analyzer", "report_generator"},
		Frequency:   5,
		Description: "数据分析流水线",
		Examples:    []string{"file_reader -> data_analyzer -> report_generator"},
		CreatedAt:   time.Now(),
	}

	draft := gen.GenerateSkillDraft(pattern)
	if draft == nil {
		t.Fatal("Expected draft, got nil")
	}

	t.Logf("Generated skill draft:")
	t.Logf("  Name: %s", draft.Name)
	t.Logf("  Description: %s", draft.Description)
	t.Logf("  Trigger: %s", draft.Trigger)
	t.Logf("  Content:\n%s", draft.Content)

	if draft.Version != "0.1.0" {
		t.Errorf("Expected version 0.1.0, got %s", draft.Version)
	}

	if len(draft.Pattern.ToolSeq) != 3 {
		t.Errorf("Expected 3 tools in pattern, got %d", len(draft.Pattern.ToolSeq))
	}
}

// TestSkillGenerator_CommonSequences 测试常见序列检测
func TestSkillGenerator_CommonSequences(t *testing.T) {
	gen := NewSkillGenerator()

	// 测试常见序列
	toolCalls := []ToolCall{
		{ToolName: "web_search", Args: "{}", Time: time.Now()},
		{ToolName: "web_fetch", Args: "{}", Time: time.Now()},
		{ToolName: "summary", Args: "{}", Time: time.Now()},
		{ToolName: "web_search", Args: "{}", Time: time.Now()},
		{ToolName: "web_fetch", Args: "{}", Time: time.Now()},
		{ToolName: "summary", Args: "{}", Time: time.Now()},
	}

	pattern := gen.AnalyzeToolCalls(toolCalls)
	if pattern == nil {
		t.Fatal("Expected to detect pattern, got nil")
	}

	t.Logf("Detected common sequence: %v (frequency: %d)", pattern.ToolSeq, pattern.Frequency)
}
