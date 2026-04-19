package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/jettjia/XiaoQinglong/runner/pkg/xqldir"
)

// ========== LoadSkillTool ==========

// LoadSkillInput for load_skill tool
type LoadSkillInput struct {
	SkillName string `json:"skill_name"` // Skill name to load
}

// LoadSkillOutput for load_skill result
type LoadSkillOutput struct {
	Success bool   `json:"success"`
	Name    string `json:"name"`
	Content string `json:"content"` // Full SKILL.md content
	Error   string `json:"error,omitempty"`
}

// LoadSkillTool loads the full content of a skill
type LoadSkillTool struct {
	skillsDir string
}

func NewLoadSkillTool(skillsDir string) *LoadSkillTool {
	if skillsDir == "" {
		skillsDir = xqldir.GetSkillsDir()
	}
	return &LoadSkillTool{skillsDir: skillsDir}
}

func init() {
	GlobalRegistry.Register(ToolMeta{
		Name:           "load_skill",
		Desc:           "Load the full content of a skill by name. Returns the complete SKILL.md content including instructions and examples.",
		IsReadOnly:     true,
		MaxResultChars: 100000,
		DefaultRisk:    "low",
		Creator: func(basePath string) interface{} {
			return NewLoadSkillTool("")
		},
	})
}

func (t *LoadSkillTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "load_skill",
		Desc: "Load the full content of a skill by name. Use this when you need to see the complete SKILL.md content including instructions, examples, and any supporting files.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"skill_name": {
				Type:        schema.String,
				Desc:        "The name of the skill to load (e.g., 'csv-data-analysis', 'pptx')",
				Required:    true,
			},
		}),
	}, nil
}

func (t *LoadSkillTool) ValidateInput(ctx context.Context, input string) *ValidationResult {
	var skillInput LoadSkillInput
	if err := json.Unmarshal([]byte(input), &skillInput); err != nil {
		return &ValidationResult{Valid: false, Message: fmt.Sprintf("invalid JSON: %v", err), ErrorCode: 1}
	}
	if skillInput.SkillName == "" {
		return &ValidationResult{Valid: false, Message: "skill_name is required", ErrorCode: 2}
	}
	return &ValidationResult{Valid: true}
}

func (t *LoadSkillTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var skillInput LoadSkillInput
	if err := json.Unmarshal([]byte(input), &skillInput); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	result := t.loadSkill(skillInput.SkillName)
	data, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	return string(data), nil
}

func (t *LoadSkillTool) loadSkill(name string) LoadSkillOutput {
	// Find skill by name
	skillPath := findSkillByName(name, t.skillsDir)
	if skillPath == "" {
		return LoadSkillOutput{
			Success: false,
			Error:   fmt.Sprintf("Skill '%s' not found. Use skills_list to see available skills.", name),
		}
	}

	// Read SKILL.md
	skillMdPath := filepath.Join(skillPath, "SKILL.md")
	content, err := os.ReadFile(skillMdPath)
	if err != nil {
		return LoadSkillOutput{
			Success: false,
			Error:   fmt.Sprintf("Failed to read SKILL.md: %v", err),
		}
	}

	return LoadSkillOutput{
		Success: true,
		Name:    name,
		Content: string(content),
	}
}
