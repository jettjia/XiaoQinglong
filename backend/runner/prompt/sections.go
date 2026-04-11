package prompt

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jettjia/XiaoQinglong/runner/types"
)

// ========== Prompt Section Types ==========

// SectionType represents the type of prompt section
type SectionType string

const (
	IntroSection             SectionType = "intro"
	SystemSection           SectionType = "system"
	DoingTasksSection       SectionType = "doing_tasks"
	ActionsSection          SectionType = "actions"
	UsingYourToolsSection   SectionType = "using_your_tools"
	OutputEfficiencySection SectionType = "output_efficiency"
	ToneAndStyleSection     SectionType = "tone_and_style"
	SkillsSection           SectionType = "skills"
	SkillsSystemSection     SectionType = "skills_system"
	SkillUsageSection       SectionType = "skill_usage"
	McpSection              SectionType = "mcp"
	EnvironmentSection      SectionType = "environment"
	SessionSpecificSection   SectionType = "session_specific"
	ContextSection          SectionType = "context"
	FilesSection            SectionType = "files"
	A2AAgentsSection        SectionType = "a2a_agents"
	InternalAgentsSection   SectionType = "internal_agents"
	MemorySection           SectionType = "memory"
	ResponseSchemaSection   SectionType = "response_schema"
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

// GetSkillsSection returns the available skills section with metadata (dynamic)
// Reference: DB-GPT's progressive disclosure pattern
func GetSkillsSection(skills []types.Skill) string {
	if len(skills) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "# Available Skills")
	lines = append(lines, "")
	lines = append(lines, "Skills provide specialized capabilities. Available skills are listed below with their triggers and I/O.")
	lines = append(lines, "Use the `load_skill` tool to get full skill instructions when needed.")

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
		lines = append(lines, fmt.Sprintf("- **%s**: %s", skill.Name, desc))

		// Add trigger keywords if available
		if skill.Trigger != "" {
			lines = append(lines, fmt.Sprintf("  - triggers: %s", skill.Trigger))
		}

		// Add inputs if available
		if len(skill.Inputs) > 0 {
			lines = append(lines, fmt.Sprintf("  - inputs: %s", strings.Join(skill.Inputs, ", ")))
		}

		// Add outputs if available
		if len(skill.Outputs) > 0 {
			lines = append(lines, fmt.Sprintf("  - outputs: %s", strings.Join(skill.Outputs, ", ")))
		}

		lines = append(lines, "")
	}

	lines = append(lines, "Use the `load_skill` tool to get full skill instructions.")
	lines = append(lines, "Use the `run_skill` tool to execute a skill directly.")
	lines = append(lines, "Use the `orchestrate_skills` tool for complex multi-step tasks.")

	// 检查是否有生成 HTML 的 skill（如 csv-data-analysis）
	hasHtmlSkills := false
	for _, skill := range skills {
		// 检查 skill 名称或描述中是否包含分析/报告相关关键词
		lowerName := strings.ToLower(skill.Name)
		lowerDesc := strings.ToLower(skill.Description)
		if strings.Contains(lowerName, "analysis") || strings.Contains(lowerName, "report") ||
			strings.Contains(lowerDesc, "analysis") || strings.Contains(lowerDesc, "report") ||
			strings.Contains(lowerName, "csv") || strings.Contains(lowerDesc, "csv") {
			hasHtmlSkills = true
			break
		}
	}

	// 添加 HTML 输出指导
	if hasHtmlSkills {
		lines = append(lines, "")
		lines = append(lines, "# HTML Report Output Handling")
		lines = append(lines, "CRITICAL: When a skill returns a message containing a report link (like \"/reports/report_*.html\" or \"📊 查看数据分析报告\"),")
		lines = append(lines, "treat it as the FINAL response. Output the link text EXACTLY as-is without calling ANY tools.")
		lines = append(lines, "Do NOT call parse_file, read_file, or any other tool to read or process the HTML report.")
		lines = append(lines, "The report link should be displayed directly to the user - they can click it to view in browser.")
		lines = append(lines, "Stop processing immediately after receiving a report link. Do not summarize or analyze further.")
	}

	return strings.Join(lines, "\n")
}

// GetSkillsSystemSection returns the skills system guidance (Reference: DB-GPT SKILLS_SYSTEM_PROMPT)
// This instructs the LLM on how to use skills
func GetSkillsSystemSection() string {
	return `# Skills System

You have access to a skills library that provides specialized capabilities and domain knowledge.

## How to Use Skills (Progressive Disclosure)

Skills follow a **progressive disclosure** pattern:

1. **Recognize when a skill applies**: Check if the user's task matches a skill's description or triggers
2. **Load skill details**: Use the 'load_skill' tool to get full instructions for the matched skill
3. **Execute the skill**: Use the 'run_skill' tool to execute the skill, or follow the instructions in the skill
4. **For complex tasks**: Use 'orchestrate_skills' to plan and execute multiple skills

## When to Use Skills

- User's request matches a skill's domain (e.g., "research X" -> web-research skill)
- You need specialized knowledge or structured workflows
- A skill provides proven patterns for complex tasks
- The task requires specific tool sequences that the skill encapsulates

## Skill Selection Guidance

- **Match by name/keywords**: If user mentions skill name or description keywords, use that skill
- **Match by trigger**: Skills have trigger phrases - use them when user input matches triggers
- **Multiple skills**: For complex tasks requiring multiple capabilities, use 'orchestrate_skills'
- **When unsure**: Use 'load_skill' to read full skill instructions and decide

## Skill Execution Flow

1. Identify potential skill(s) from Available Skills list
2. Use 'load_skill' to get full instructions (optional but recommended)
3. Use 'run_skill' with appropriate input, or follow the skill's instructions
4. Interpret the result - if it contains a report link, treat it as final output`
}

// GetSkillUsageSection returns practical examples of skill usage
func GetSkillUsageSection() string {
	return `# Skill Usage Examples

## Direct Skill Execution
- User: "帮我分析这个CSV文件" -> Use 'run_skill' with csv-data-analysis
- User: "帮我创建PPT" -> Use 'run_skill' with pptx
- User: "上传文件到S3" -> Use 'run_skill' with s3-upload

## Browser/Search Tasks
- User: "帮我搜索北京的天气" -> Use agent-browser skill
  1. 'load_skill' to get browser CLI instructions
  2. 'run_skill' or follow instructions to open browser and search
- User: "帮我抓取这个网页的数据" -> Use agent-browser skill to navigate and extract

## Multi-Step Tasks
- User: "分析销售数据并生成报告" -> Use 'orchestrate_skills'
  - First skill: csv-data-analysis
  - Second skill: pptx or report generation

## When to Load vs Run
- **Use 'load_skill' first** when: You need to understand the full workflow before executing
- **Use 'run_skill' directly** when: Task clearly matches skill's description and you know the required input

## Common Pitfalls to Avoid
- Don't call multiple skills unnecessarily - one skill may be enough
- Don't re-implement what a skill already does - use the skill instead
- When a skill returns a report link, STOP - don't try to read/process the report file`
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

// GetMemorySection returns the memory section for the prompt
// memories 是记忆内容列表，index 是索引列表（用于展示）
func GetMemorySection(indexLines []string) string {
	if len(indexLines) == 0 {
		return ""
	}

	const MAX_INDEX_LINES = 200

	var lines []string
	lines = append(lines, "# Memory")
	lines = append(lines, "")
	lines = append(lines, "You have a persistent, file-based memory system at a database. Future conversations can have a complete picture of who the user is, how they'd like to collaborate with you, what behaviors to avoid or repeat, and the context behind the work the user gives you.")
	lines = append(lines, "")
	lines = append(lines, "If the user explicitly asks you to remember something, save it immediately as whichever type fits best. If they ask you to forget something, find and remove the relevant entry.")
	lines = append(lines, "")
	lines = append(lines, "## Types of memory")
	lines = append(lines, "")
	lines = append(lines, "There are several discrete types of memory that you can store in your memory system:")
	lines = append(lines, "")
	lines = append(lines, "<types>")
	lines = append(lines, "<type>")
	lines = append(lines, "    <name>user</name>")
	lines = append(lines, "    <description>Contain information about the user's role, goals, responsibilities, and knowledge. Great user memories help you tailor your future behavior to the user's preferences and perspective.</description>")
	lines = append(lines, "    <when_to_save>When you learn any details about the user's role, preferences, responsibilities, or knowledge</when_to_save>")
	lines = append(lines, "    <how_to_use>When your work should be informed by the user's profile or perspective.</how_to_use>")
	lines = append(lines, "</type>")
	lines = append(lines, "<type>")
	lines = append(lines, "    <name>feedback</name>")
	lines = append(lines, "    <description>Guidance the user has given you about how to approach work — both what to avoid and what to keep doing. These are a very important type of memory to read and write as they allow you to remain coherent and responsive to the way you should approach work in the project.</description>")
	lines = append(lines, "    <when_to_save>Any time the user corrects your approach (\"no not that\", \"don't\", \"stop doing X\") OR confirms a non-obvious approach worked (\"yes exactly\", \"perfect, keep doing that\").</when_to_save>")
	lines = append(lines, "    <how_to_use>Let these memories guide your behavior so that the user does not need to offer the same guidance twice.</how_to_use>")
	lines = append(lines, "</type>")
	lines = append(lines, "<type>")
	lines = append(lines, "    <name>project</name>")
	lines = append(lines, "    <description>Information that you learn about ongoing work, goals, initiatives, bugs, or incidents within the project that is not otherwise derivable from the code or git history.</description>")
	lines = append(lines, "    <when_to_save>When you learn who is doing what, why, or by when. Always convert relative dates to absolute dates when saving.</when_to_save>")
	lines = append(lines, "    <how_to_use>Use these memories to more fully understand the details and nuance behind the user's request.</how_to_use>")
	lines = append(lines, "</type>")
	lines = append(lines, "<type>")
	lines = append(lines, "    <name>reference</name>")
	lines = append(lines, "    <description>Stores pointers to where information can be found in external systems. These memories allow you to remember where to look to find up-to-date information outside of the project directory.</description>")
	lines = append(lines, "    <when_to_save>When you learn about resources in external systems and their purpose.</when_to_save>")
	lines = append(lines, "    <how_to_use>When the user references an external system or information that may be in an external system.</how_to_use>")
	lines = append(lines, "</type>")
	lines = append(lines, "</types>")
	lines = append(lines, "")
	lines = append(lines, "## What NOT to save in memory")
	lines = append(lines, "")
	lines = append(lines, "- Code patterns, conventions, architecture, file paths, or project structure — these can be derived from reading the current project state.")
	lines = append(lines, "- Git history, recent changes, or who-changed-what — `git log` / `git blame` are authoritative.")
	lines = append(lines, "- Debugging solutions or fix recipes — the fix is in the code; the commit message has the context.")
	lines = append(lines, "- Anything already documented in CLAUDE.md files.")
	lines = append(lines, "- Ephemeral task details: in-progress work, temporary state, current conversation context.")
	lines = append(lines, "")
	lines = append(lines, "## Memory and other forms of persistence")
	lines = append(lines, "Memory is one of several persistence mechanisms available to you as you assist the user in a given conversation. The distinction is often that memory can be recalled in future conversations and should not be used for persisting information that is only useful within the scope of the current conversation.")
	lines = append(lines, "- When to use or update a plan instead of memory: If you are about to start a non-trivial implementation task and you would like to reach alignment with the user on your approach you should use a Plan rather than saving this information to memory.")
	lines = append(lines, "- When to use or update tasks instead of memory: When you need to break your work in current conversation into discrete steps or keep track of your progress use tasks instead of saving to memory.")
	lines = append(lines, "")

	// 添加索引
	lines = append(lines, "## MEMORY.md")
	lines = append(lines, "")

	// 截断到 MAX_INDEX_LINES
	if len(indexLines) > MAX_INDEX_LINES {
		indexLines = indexLines[:MAX_INDEX_LINES]
	}

	for _, line := range indexLines {
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// GetResponseSchemaSection returns the response schema section for structured output
func GetResponseSchemaSection(schema *types.ResponseSchemaConfig) string {
	if schema == nil || schema.Type == "" || schema.Schema == nil {
		return ""
	}

	var lines []string

	switch schema.Type {
	case "a2ui":
		lines = append(lines, "# Response Format")
		lines = append(lines, "")
		lines = append(lines, "You must respond in A2UI format (JSON). Follow the schema below exactly:")
		lines = append(lines, "")
		lines = append(lines, "## Response Schema")
		schemaJSON, err := json.MarshalIndent(schema.Schema, "", "  ")
		if err != nil {
			return ""
		}
		lines = append(lines, "```json")
		lines = append(lines, string(schemaJSON))
		lines = append(lines, "```")
		lines = append(lines, "")
		lines = append(lines, "Important: Output ONLY valid JSON that conforms to the schema above. Do not include any other text, markdown formatting, or explanation.")

	case "json":
		lines = append(lines, "# Response Format")
		lines = append(lines, "")
		lines = append(lines, "You must respond in JSON format. Follow the schema below exactly:")
		lines = append(lines, "")
		lines = append(lines, "## Response Schema")
		schemaJSON, err := json.MarshalIndent(schema.Schema, "", "  ")
		if err != nil {
			return ""
		}
		lines = append(lines, "```json")
		lines = append(lines, string(schemaJSON))
		lines = append(lines, "```")
		lines = append(lines, "")
		lines = append(lines, "Important: Output ONLY valid JSON that conforms to the schema above.")

	case "audio":
		lines = append(lines, "# Response Format")
		lines = append(lines, "")
		lines = append(lines, "You must respond in audio format. Provide either a URL or base64 encoded audio data.")
		lines = append(lines, "")
		lines = append(lines, "## Response Schema")
		audioSchema := map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "URL to the audio file (mp3, wav, etc.)",
				},
				"base64": map[string]any{
					"type":        "string",
					"description": "Base64 encoded audio data (use when no URL available)",
				},
				"format": map[string]any{
					"type":        "string",
					"description": "Audio format (mp3, wav, ogg, etc.)",
				},
				"duration": map[string]any{
					"type":        "number",
					"description": "Duration in seconds",
				},
			},
		}
		schemaJSON, err := json.MarshalIndent(audioSchema, "", "  ")
		if err != nil {
			return ""
		}
		lines = append(lines, "```json")
		lines = append(lines, string(schemaJSON))
		lines = append(lines, "```")
		lines = append(lines, "")
		lines = append(lines, "Important: Output ONLY valid JSON that conforms to the schema above.")

	case "image", "video":
		lines = append(lines, "# Response Format")
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("You must respond in %s format. Provide URL or base64 encoded data.", schema.Type))
		lines = append(lines, "")
		lines = append(lines, "## Response Schema")
		mediaSchema := map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": fmt.Sprintf("URL to the %s file", schema.Type),
				},
				"base64": map[string]any{
					"type":        "string",
					"description": fmt.Sprintf("Base64 encoded %s data (use when no URL available)", schema.Type),
				},
			},
		}
		schemaJSON, err := json.MarshalIndent(mediaSchema, "", "  ")
		if err != nil {
			return ""
		}
		lines = append(lines, "```json")
		lines = append(lines, string(schemaJSON))
		lines = append(lines, "```")
		lines = append(lines, "")
		lines = append(lines, "Important: Output ONLY valid JSON that conforms to the schema above.")

	case "markdown", "text":
		// markdown/text 类型不需要特殊指导，LLM 会直接输出
		return ""

	default:
		return ""
	}

	return strings.Join(lines, "\n")
}
