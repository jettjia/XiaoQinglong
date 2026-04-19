package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/jettjia/XiaoQinglong/runner/pkg/xqldir"
	"gopkg.in/yaml.v3"
)

// ========== SkillManageTool ==========

// SkillManageInput for skill_manage tool
type SkillManageInput struct {
	Action     string `json:"action"`      // create, patch, delete
	Name       string `json:"name"`        // Skill name
	Content    string `json:"content"`    // SKILL.md content (for create/edit)
	Category   string `json:"category"`   // Category for new skill
	OldString  string `json:"old_string"` // Text to find (for patch)
	NewString  string `json:"new_string"` // Replacement text (for patch)
	ReplaceAll bool   `json:"replace_all"` // Replace all occurrences (for patch)
}

// SkillManageOutput for skill_manage result
type SkillManageOutput struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error  string `json:"error,omitempty"`
	Path   string `json:"path,omitempty"`
}

// SkillManageTool manages skills (create, update, delete)
type SkillManageTool struct {
	skillsDir string
}

func NewSkillManageTool(skillsDir string) *SkillManageTool {
	if skillsDir == "" {
		skillsDir = xqldir.GetSkillsDir()
	}
	return &SkillManageTool{skillsDir: skillsDir}
}

func init() {
	GlobalRegistry.Register(ToolMeta{
		Name:           "skill_manage",
		Desc:           "Manage skills (create, update, delete). Skills are your procedural memory — reusable approaches for recurring task types.",
		IsReadOnly:     false,
		MaxResultChars: 10000,
		DefaultRisk:    "medium",
		Creator: func(basePath string) interface{} {
			return NewSkillManageTool("")
		},
	})
}

func (t *SkillManageTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "skill_manage",
		Desc: "Manage user-created skills. Actions: create (new skill with SKILL.md), patch (find-replace in SKILL.md), delete (remove skill). Skills are stored in ~/.xiaoqinglong/skills/",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"action": {
				Type:        schema.String,
				Desc:        "Action to perform: create, patch, delete",
				Required:    true,
			},
			"name": {
				Type:        schema.String,
				Desc:        "Skill name (lowercase, hyphens/underscores allowed)",
				Required:    true,
			},
			"content": {
				Type:        schema.String,
				Desc:        "Full SKILL.md content (YAML frontmatter + markdown body). Required for 'create'.",
				Required:    false,
			},
			"category": {
				Type:        schema.String,
				Desc:        "Optional category for organizing skills (e.g., 'data-processing', 'devops')",
				Required:    false,
			},
			"old_string": {
				Type:        schema.String,
				Desc:        "Text to find in SKILL.md (for patch). Must be unique unless replace_all=true.",
				Required:    false,
			},
			"new_string": {
				Type:        schema.String,
				Desc:        "Replacement text (for patch). Can be empty to delete matched text.",
				Required:    false,
			},
			"replace_all": {
				Type:        schema.Boolean,
				Desc:        "Replace all occurrences (for patch, default false)",
				Required:    false,
			},
		}),
	}, nil
}

func (t *SkillManageTool) ValidateInput(ctx context.Context, input string) *ValidationResult {
	var skillInput SkillManageInput
	if err := json.Unmarshal([]byte(input), &skillInput); err != nil {
		return &ValidationResult{Valid: false, Message: fmt.Sprintf("invalid JSON: %v", err), ErrorCode: 1}
	}
	if skillInput.Action == "" {
		return &ValidationResult{Valid: false, Message: "action is required", ErrorCode: 2}
	}
	if skillInput.Action != "delete" && skillInput.Name == "" {
		return &ValidationResult{Valid: false, Message: "name is required", ErrorCode: 2}
	}
	validActions := map[string]bool{"create": true, "patch": true, "delete": true}
	if !validActions[skillInput.Action] {
		return &ValidationResult{Valid: false, Message: "action must be create, patch, or delete", ErrorCode: 2}
	}
	return &ValidationResult{Valid: true}
}

func (t *SkillManageTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var skillInput SkillManageInput
	if err := json.Unmarshal([]byte(input), &skillInput); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	var result SkillManageOutput
	switch skillInput.Action {
	case "create":
		result = t.createSkill(skillInput.Name, skillInput.Content, skillInput.Category)
	case "patch":
		result = t.patchSkill(skillInput.Name, skillInput.OldString, skillInput.NewString, skillInput.ReplaceAll)
	case "delete":
		result = t.deleteSkill(skillInput.Name)
	default:
		result = SkillManageOutput{Success: false, Error: fmt.Sprintf("unknown action: %s", skillInput.Action)}
	}

	data, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	return string(data), nil
}

// ===== Validation Helpers =====

var validNameRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

func validateName(name string) string {
	if name == "" {
		return "Skill name is required"
	}
	if len(name) > 64 {
		return "Skill name exceeds 64 characters"
	}
	if !validNameRegex.MatchString(name) {
		return "Invalid skill name. Use lowercase letters, numbers, hyphens, dots, and underscores. Must start with a letter or digit."
	}
	return ""
}

func validateCategory(category string) string {
	if category == "" {
		return ""
	}
	if strings.Contains(category, "/") || strings.Contains(category, "\\") {
		return "Category must be a single directory name (no slashes)"
	}
	if len(category) > 64 {
		return "Category exceeds 64 characters"
	}
	if !validNameRegex.MatchString(category) {
		return "Invalid category name"
	}
	return ""
}

// skillFrontmatter for parsing YAML frontmatter
type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Version     string `yaml:"version"`
}

func validateFrontmatter(content string) string {
	if content == "" {
		return "Content cannot be empty"
	}
	if !strings.HasPrefix(content, "---") {
		return "SKILL.md must start with YAML frontmatter (---)"
	}

	// Find closing ---
	endIdx := strings.Index(content[3:], "\n---")
	if endIdx < 0 {
		return "SKILL.md frontmatter is not closed (missing ---)"
	}

	yamlContent := content[3 : endIdx+3]
	var fm skillFrontmatter
	if err := yaml.Unmarshal([]byte(yamlContent), &fm); err != nil {
		return fmt.Sprintf("YAML frontmatter parse error: %v", err)
	}

	if fm.Name == "" {
		return "Frontmatter must include 'name' field"
	}
	if fm.Description == "" {
		return "Frontmatter must include 'description' field"
	}
	if len(fm.Description) > 1024 {
		return "Description exceeds 1024 characters"
	}

	// Check body exists after frontmatter
	body := strings.TrimSpace(content[endIdx+6:])
	if body == "" {
		return "SKILL.md must have content after the frontmatter (instructions, procedures, etc.)"
	}

	return ""
}

const maxSkillContentChars = 100000

func validateContentSize(content string) string {
	if len(content) > maxSkillContentChars {
		return fmt.Sprintf("Content exceeds %d characters. Consider splitting into a smaller SKILL.md with supporting files.", maxSkillContentChars)
	}
	return ""
}

// ===== Skill Operations =====

func (t *SkillManageTool) createSkill(name, content, category string) SkillManageOutput {
	// Validate name
	if err := validateName(name); err != "" {
		return SkillManageOutput{Success: false, Error: err}
	}

	// Validate category
	if err := validateCategory(category); err != "" {
		return SkillManageOutput{Success: false, Error: err}
	}

	// Validate content
	if err := validateFrontmatter(content); err != "" {
		return SkillManageOutput{Success: false, Error: err}
	}
	if err := validateContentSize(content); err != "" {
		return SkillManageOutput{Success: false, Error: err}
	}

	// Check for name collision
	if existing := findSkillByName(name, t.skillsDir); existing != "" {
		return SkillManageOutput{Success: false, Error: fmt.Sprintf("A skill named '%s' already exists at %s", name, existing)}
	}

	// Create skill directory
	var skillDir string
	if category != "" {
		skillDir = filepath.Join(t.skillsDir, category, name)
	} else {
		skillDir = filepath.Join(t.skillsDir, name)
	}

	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return SkillManageOutput{Success: false, Error: fmt.Sprintf("Failed to create skill directory: %v", err)}
	}

	// Write SKILL.md
	skillMdPath := filepath.Join(skillDir, "SKILL.md")
	if err := atomicWriteText(skillMdPath, content); err != nil {
		// Clean up directory on failure
		os.RemoveAll(skillDir)
		return SkillManageOutput{Success: false, Error: fmt.Sprintf("Failed to write SKILL.md: %v", err)}
	}

	// TODO: Security scan could be added here

	relPath, _ := filepath.Rel(t.skillsDir, skillDir)
	return SkillManageOutput{
		Success: true,
		Message: fmt.Sprintf("Skill '%s' created successfully", name),
		Path:    relPath,
	}
}

func (t *SkillManageTool) patchSkill(name, oldString, newString string, replaceAll bool) SkillManageOutput {
	if oldString == "" {
		return SkillManageOutput{Success: false, Error: "old_string is required for patch"}
	}
	if newString == "" && newString != "" { // newString can be empty string, but not nil
		// This condition is actually checking if newString was not provided at all
	}

	// Find skill
	skillPath := findSkillByName(name, t.skillsDir)
	if skillPath == "" {
		return SkillManageOutput{Success: false, Error: fmt.Sprintf("Skill '%s' not found", name)}
	}

	skillMdPath := filepath.Join(skillPath, "SKILL.md")
	content, err := os.ReadFile(skillMdPath)
	if err != nil {
		return SkillManageOutput{Success: false, Error: fmt.Sprintf("Failed to read SKILL.md: %v", err)}
	}

	// Fuzzy find and replace
	var newContent string
	var matchCount int
	if replaceAll {
		newContent = strings.ReplaceAll(string(content), oldString, newString)
		matchCount = strings.Count(string(content), oldString)
	} else {
		idx := strings.Index(string(content), oldString)
		if idx < 0 {
			return SkillManageOutput{Success: false, Error: "old_string not found in SKILL.md"}
		}
		newContent = string(content[:idx]) + newString + string(content[idx+len(oldString):])
		matchCount = 1
	}

	// Validate frontmatter still intact
	if err := validateFrontmatter(newContent); err != "" {
		return SkillManageOutput{Success: false, Error: fmt.Sprintf("Patch would break SKILL.md structure: %s", err)}
	}

	// Validate size
	if err := validateContentSize(newContent); err != "" {
		return SkillManageOutput{Success: false, Error: err}
	}

	// Backup original
	backupPath := skillMdPath + ".bak"
	os.WriteFile(backupPath, content, 0644)
	defer os.Remove(backupPath)

	// Write new content
	if err := atomicWriteText(skillMdPath, newContent); err != nil {
		return SkillManageOutput{Success: false, Error: fmt.Sprintf("Failed to write SKILL.md: %v", err)}
	}

	// TODO: Security scan could be added here

	return SkillManageOutput{
		Success: true,
		Message: fmt.Sprintf("Patched SKILL.md in skill '%s' (%d replacement%s)", name, matchCount, pluralize(matchCount)),
		Path:    skillPath,
	}
}

func (t *SkillManageTool) deleteSkill(name string) SkillManageOutput {
	// Find skill
	skillPath := findSkillByName(name, t.skillsDir)
	if skillPath == "" {
		return SkillManageOutput{Success: false, Error: fmt.Sprintf("Skill '%s' not found", name)}
	}

	// Delete skill directory
	if err := os.RemoveAll(skillPath); err != nil {
		return SkillManageOutput{Success: false, Error: fmt.Sprintf("Failed to delete skill: %v", err)}
	}

	// Clean up empty category directory
	categoryDir := filepath.Dir(skillPath)
	if categoryDir != t.skillsDir {
		entries, _ := os.ReadDir(categoryDir)
		if len(entries) == 0 {
			os.Remove(categoryDir)
		}
	}

	return SkillManageOutput{
		Success: true,
		Message: fmt.Sprintf("Skill '%s' deleted", name),
	}
}

// ===== Helper Functions =====

// findSkillByName searches for a skill by name across all skill directories
func findSkillByName(name, skillsDir string) string {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillDir := filepath.Join(skillsDir, entry.Name())

		// Check if skill.md exists directly in this directory (no category)
		skillMdPath := filepath.Join(skillDir, "SKILL.md")
		if entry.Name() == name {
			if _, err := os.Stat(skillMdPath); err == nil {
				return skillDir
			}
		}

		// Check subdirectories (with category)
		subEntries, err := os.ReadDir(skillDir)
		if err != nil {
			continue
		}
		for _, subEntry := range subEntries {
			if !subEntry.IsDir() {
				continue
			}
			if subEntry.Name() == name {
				subSkillMdPath := filepath.Join(skillDir, subEntry.Name(), "SKILL.md")
				if _, err := os.Stat(subSkillMdPath); err == nil {
					return filepath.Join(skillDir, subEntry.Name())
				}
			}
		}
	}
	return ""
}

// atomicWriteText atomically writes text content to a file
func atomicWriteText(filePath, content string) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Write to temp file first
	tmpFile, err := os.CreateTemp(dir, ".skill_tmp_")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return err
	}
	tmpFile.Close()

	// Atomic rename
	if err := os.Rename(tmpPath, filePath); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

func pluralize(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
