package prompt

import (
	"fmt"
	"strings"

	"github.com/jettjia/XiaoQinglong/runner/types"
)

// ========== Prompt Section Types ==========

// SectionType represents the type of prompt section
type SectionType string

const (
	IntroSection           SectionType = "intro"
	SystemSection         SectionType = "system"
	DoingTasksSection     SectionType = "doing_tasks"
	ActionsSection        SectionType = "actions"
	UsingYourToolsSection SectionType = "using_your_tools"
	OutputEfficiencySection SectionType = "output_efficiency"
	ToneAndStyleSection   SectionType = "tone_and_style"
	SkillsSection         SectionType = "skills"
	McpSection            SectionType = "mcp"
	EnvironmentSection    SectionType = "environment"
	SessionSpecificSection SectionType = "session_specific"
	ContextSection        SectionType = "context"
	FilesSection          SectionType = "files"
	A2AAgentsSection      SectionType = "a2a_agents"
	InternalAgentsSection SectionType = "internal_agents"
)

// PromptSection represents a single section in the prompt
type PromptSection struct {
	Type    SectionType
	Content string
	Dynamic bool // 动态区块每次重新计算
}

// MaxSkillDescChars is the maximum characters for skill descriptions in listings
// Reference: Claude Code's MAX_LISTING_DESC_CHARS = 250
const MaxSkillDescChars = 250

// ========== Section Content Generators ==========

// GetIntroSection returns the identity definition section
func GetIntroSection() string {
	return `You are an interactive agent that helps users with software engineering tasks.
Use the instructions below and the tools available to you to assist the user.`
}

// GetSystemSection returns the system rules section
func GetSystemSection() string {
	return `# System
- All text you output outside of tool use is displayed to the user.
- Tools are executed in a user-selected permission mode.
- Tool results and user messages may include <system-reminder> tags.
- The system will automatically compress prior messages in your conversation.`
}

// GetDoingTasksSection returns the task execution rules section
func GetDoingTasksSection() string {
	return `# Doing Tasks
- Analyze requirements carefully before starting implementation.
- Follow existing code conventions and patterns in the project.
- Write clean, maintainable code with appropriate comments.
- Consider performance, security, and error handling.
- Prefer simple solutions over complex ones unless complexity adds clear value.`
}

// GetActionsSection returns the careful actions section (dangerous operations)
func GetActionsSection() string {
	return `# Executing actions with care
Carefully consider the reversibility and blast radius of actions.
Destructive operations: deleting files/branches, dropping database tables, killing processes.
Hard-to-reverse operations: force-pushing, git reset --hard, amending published commits.
Actions visible to others: pushing code, creating/closing PRs, sending messages.
When in doubt, ask the user to confirm before proceeding.`
}

// GetUsingYourToolsSection returns the tool usage guidance section
func GetUsingYourToolsSection(enabledTools []string) string {
	var toolsList string
	if len(enabledTools) > 0 {
		toolsList = strings.Join(enabledTools, ", ")
	} else {
		toolsList = "available tools"
	}
	return fmt.Sprintf(`# Using your tools
- Use %s instead of shell commands where possible.
- You can call multiple tools in a single response when they are independent.
- If tool calls depend on previous results, call them sequentially.
- When using tools, prefer the most specific tool for the task.`, toolsList)
}

// GetOutputEfficiencySection returns the output efficiency section
func GetOutputEfficiencySection() string {
	return `# Output efficiency
IMPORTANT: Go straight to the point. Try the simplest approach first without going in circles. Be extra concise.
Keep your text output brief and direct. Lead with the answer or action, not the reasoning.
Skip filler words, preamble, and unnecessary transitions. Do not restate what the user said — just do it.
When explaining, include only what is necessary for the user to understand.`
}

// GetToneAndStyleSection returns the tone and style section
func GetToneAndStyleSection() string {
	return `# Tone and style
- Only use emojis if the user explicitly requests it.
- Your responses should be short and concise.
- When referencing code, use markdown link syntax: [filename.ext](path/to/file.ext)
- Use markdown link syntax for file paths and line numbers: [filename.ts:42](path/to/file.ts#L42)
- Avoid using backticks or HTML tags for file references.`
}

// GetSkillsSection returns the available skills section (dynamic)
func GetSkillsSection(skills []types.Skill) string {
	if len(skills) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "# Available Skills")
	lines = append(lines, "Skills provide specialized capabilities. Use the load_skill tool to get full details.")

	for _, skill := range skills {
		desc := skill.Description
		if desc == "" {
			desc = "No description"
		}
		// Truncate description to MaxSkillDescChars
		runes := []rune(desc)
		if len(runes) > MaxSkillDescChars {
			desc = string(runes[:MaxSkillDescChars-1]) + "…"
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", skill.Name, desc))
	}

	lines = append(lines, "")
	lines = append(lines, "Use the load_skill tool to get full skill instructions.")

	return strings.Join(lines, "\n")
}

// GetMcpSection returns the MCP instructions section (dynamic)
func GetMcpSection(mcps []types.MCPConfig) string {
	if len(mcps) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "# MCP Server Instructions")
	lines = append(lines, "The following MCP servers have provided instructions:")

	for _, mcp := range mcps {
		lines = append(lines, fmt.Sprintf("\n## %s", mcp.Name))
		if mcp.Transport == "http" && mcp.Endpoint != "" {
			lines = append(lines, fmt.Sprintf("Endpoint: %s", mcp.Endpoint))
		} else if mcp.Transport == "stdio" && mcp.Command != "" {
			lines = append(lines, fmt.Sprintf("Command: %s", mcp.Command))
		}
	}

	return strings.Join(lines, "\n")
}

// GetContextSection returns the context variables section
func GetContextSection(context map[string]any) string {
	if len(context) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "# Context Information")

	for k, v := range context {
		lines = append(lines, fmt.Sprintf("- %s: %v", k, v))
	}

	return strings.Join(lines, "\n")
}

// GetFilesSection returns the uploaded files section
func GetFilesSection(files []types.FileConfig) string {
	if len(files) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "# User Uploaded Files")
	lines = append(lines, "The user has uploaded the following files:")

	for _, f := range files {
		lines = append(lines, fmt.Sprintf("- %s (Path: %s)", f.Name, f.VirtualPath))
	}

	return strings.Join(lines, "\n")
}

// GetA2AAgentsSection returns the A2A agents section
func GetA2AAgentsSection(a2a []types.A2AAgentConfig) string {
	if len(a2a) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "# Available External Agents")

	for _, agent := range a2a {
		lines = append(lines, fmt.Sprintf("- %s: %s", agent.Name, agent.Endpoint))
	}

	return strings.Join(lines, "\n")
}

// GetInternalAgentsSection returns the internal agents section
func GetInternalAgentsSection(agents []types.InternalAgentConfig) string {
	if len(agents) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "# Available Internal Agents")

	for _, agent := range agents {
		lines = append(lines, fmt.Sprintf("- %s (%s): %s", agent.Name, agent.ID, agent.Prompt))
	}

	return strings.Join(lines, "\n")
}
