# Claude Code 系统提示词分析

基于 Claude Code 源码的系统提示词、Skill 引导和 Tool 调用机制分析。

---

## 一、核心系统提示词结构 (src/buddy/prompt.ts, src/utils/systemPrompt.ts)

### 1.1 系统提示词优先级

```
0. Override system prompt (完全替换所有其他提示词)
1. Coordinator system prompt (协调者模式)
2. Agent system prompt (Agent 定义中的提示词)
3. Custom system prompt (--system-prompt 指定)
4. Default system prompt (标准 Claude Code 提示词)
```

### 1.2 默认系统提示词主要区块

```typescript
// src/buddy/prompt.ts 核心结构

// 1. Intro Section - 身份定义
function getSimpleIntroSection() {
  return `You are an interactive agent that helps users with software engineering tasks.
Use the instructions below and the tools available to you to assist the user.`
}

// 2. System Section - 系统级规则
function getSimpleSystemSection() {
  return `# System
- All text you output outside of tool use is displayed to the user.
- Tools are executed in a user-selected permission mode.
- Tool results and user messages may include <system-reminder> tags.
- The system will automatically compress prior messages in your conversation.`
}

// 3. Doing Tasks Section - 任务执行规则
function getSimpleDoingTasksSection() {
  // 包含：代码风格、执行效率、安全考虑等
}

// 4. Actions Section - 危险操作确认
function getActionsSection() {
  return `# Executing actions with care
Carefully consider the reversibility and blast radius of actions.
Destructive operations: deleting files/branches, dropping database tables...
Hard-to-reverse operations: force-pushing, git reset --hard...`
}

// 5. Using Your Tools Section - 工具使用引导
function getUsingYourToolsSection(enabledTools: Set<string>) {
  // 优先使用专用工具而非 Bash
  // 工具并行调用建议
}

// 6. Output Efficiency Section - 输出效率
function getOutputEfficiencySection() {
  return `# Output efficiency
IMPORTANT: Go straight to the point. Try the simplest approach first...
Keep your text output brief and direct. Lead with the answer or action...`
}

// 7. Tone and Style Section - 语气风格
function getSimpleToneAndStyleSection() {
  return `# Tone and style
- Only use emojis if the user explicitly requests it.
- Your responses should be short and concise.
- When referencing code, use file_path:line_number format.`
}
```

---

## 二、Skill 引导机制详解

### 2.1 Skill Tool 提示词 (src/tools/SkillTool/prompt.ts)

Skill 是 Claude Code 中非常重要的扩展机制，允许用户通过 slash command 调用预定义的技能。

```typescript
// Skill Tool 的核心提示词
export const getPrompt = memoize(async (_cwd: string): Promise<string> => {
  return `Execute a skill within the main conversation

When users ask you to perform tasks, check if any of the available skills match. Skills provide specialized capabilities and domain knowledge.

When users reference a "slash command" or "/<something>" (e.g., "/commit", "/review-pr"), they are referring to a skill. Use this tool to invoke it.

How to invoke:
- Use this tool with the skill name and optional arguments
- Examples:
  - \`skill: "pdf"\` - invoke the pdf skill
  - \`skill: "commit", args: "-m 'Fix bug'\` - invoke with arguments
  - \`skill: "review-pr", args: "123"\` - invoke with arguments
  - \`skill: "ms-office-suite:pdf"\` - invoke using fully qualified name

Important:
- Available skills are listed in system-reminder messages in the conversation
- When a skill matches the user's request, this is a BLOCKING REQUIREMENT: invoke the relevant Skill tool BEFORE generating any other response about the task
- NEVER mention a skill without actually calling this tool
- Do not invoke a skill that is already running
- Do not use this tool for built-in CLI commands (like /help, /clear, etc.)
- If you see a <COMMAND_NAME_TAG> tag in the current conversation turn, the skill has ALREADY been loaded - follow the instructions directly instead of calling this tool again
`
})
```

### 2.2 Skill 列表的 Token 预算控制

```typescript
// src/tools/SkillTool/prompt.ts

// Skill 列表只占用 1% 的 context window (按字符计算)
export const SKILL_BUDGET_CONTEXT_PERCENT = 0.01
export const CHARS_PER_TOKEN = 4
export const DEFAULT_CHAR_BUDGET = 8_000 // Fallback: 1% of 200k × 4

// 每个 skill 描述的最大字符数（硬上限）
// 原因：Skill 列表只是用于发现，真正的内容在调用时加载
export const MAX_LISTING_DESC_CHARS = 250
```

### 2.3 Skill 描述的截断策略

```typescript
// formatCommandsWithinBudget - 在预算内格式化 skill 列表
function formatCommandsWithinBudget(commands: Command[], contextWindowTokens?: number): string {
  const budget = getCharBudget(contextWindowTokens)

  // 1. 优先保留 bundled skills 的完整描述
  // 2. 非 bundled 描述截断到 maxDescLen
  // 3. 如果 budget 极其紧张，非 bundled 只显示名称

  // 截断示例：
  // - /commit: Commit staged changes or create...
  // - /review-pr: Review a pull request...
  // - /pdf:
  // (第三个 skill 的描述被完全移除以节省空间)
}
```

### 2.4 Skill 的 frontmatter 定义 (src/skills/loadSkillsDir.ts)

```typescript
// Skill frontmatter 字段解析
export function parseSkillFrontmatterFields(
  frontmatter: FrontmatterData,
  markdownContent: string,
  resolvedName: string,
): {
  // 显示名称（可选，默认使用目录名）
  displayName: string | undefined

  // 描述（从 frontmatter 或 markdown 内容提取）
  description: string
  hasUserSpecifiedDescription: boolean

  // 允许使用的工具列表
  allowedTools: string[]

  // 参数提示
  argumentHint: string | undefined
  argumentNames: string[]

  // 使用场景描述（帮助模型判断何时调用）
  whenToUse: string | undefined

  // 模型覆盖
  model: ReturnType<typeof parseUserSpecifiedModel> | undefined

  // 是否禁用模型调用（用于纯脚本类 skill）
  disableModelInvocation: boolean

  // 是否用户可调用（默认 true）
  userInvocable: boolean

  // Hooks 配置
  hooks: HooksSettings | undefined

  // 执行上下文：inline | fork
  executionContext: 'fork' | undefined

  // 指定 agent 类型
  agent: string | undefined

  // 努力程度
  effort: EffortValue | undefined

  // Shell 配置
  shell: FrontmatterShell | undefined

  // 条件激活路径
  paths: string[] | undefined
}
```

### 2.5 Skill 文件结构

```
skill-name/
├── SKILL.md          # 主 skill 文件（必需）
└── examples/         # 可选的示例目录
    └── example.md
```

### 2.6 Skill 调用流程

```
1. 用户输入 "/commit" 或类似 slash command
2. 模型识别这是一个 skill 调用
3. 调用 SkillTool，传入 skill name
4. SkillTool.validateInput() 验证 skill 存在
5. SkillTool.call() 执行：
   a. 查找 skill 定义（本地/MCP/bundled）
   b. 调用 skill.getPromptForCommand(args) 获取内容
   c. 内容被包装成 user message 注入对话
   d. 如果 skill 指定了 allowedTools，临时添加到权限
6. 模型收到 skill 内容，执行任务
```

### 2.7 Bundled Skill 注册模式

```typescript
// src/skills/bundled/index.ts
export function initBundledSkills(): void {
  registerUpdateConfigSkill()
  registerKeybindingsSkill()
  registerVerifySkill()
  // ...
}

// src/skills/bundled/verify.ts
export function registerVerifySkill(): void {
  if (process.env.USER_TYPE !== 'ant') {
    return  // 只在内部版本注册
  }

  registerBundledSkill({
    name: 'verify',
    description: DESCRIPTION,
    userInvocable: true,
    files: SKILL_FILES,
    async getPromptForCommand(args) {
      const parts: string[] = [SKILL_BODY.trimStart()]
      if (args) {
        parts.push(`## User Request\n\n${args}`)
      }
      return [{ type: 'text', text: parts.join('\n\n') }]
    },
  })
}
```

---

## 三、MCP (Model Context Protocol) 集成

### 3.1 MCP Server 提示词 (src/buddy/prompt.ts)

```typescript
function getMcpInstructionsSection(mcpClients: MCPServerConnection[]): string | null {
  if (!mcpClients || mcpClients.length === 0) return null

  const connectedClients = mcpClients.filter(
    (client): client is ConnectedMCPServer => client.type === 'connected'
  )

  const clientsWithInstructions = connectedClients.filter(
    client => client.instructions
  )

  if (clientsWithInstructions.length === 0) {
    return null
  }

  return `# MCP Server Instructions

The following MCP servers have provided instructions for how to use their tools and resources:

## ${client.name}
${client.instructions}`
}
```

### 3.2 MCP Skill Builder (src/skills/mcpSkillBuilders.ts)

```typescript
// MCP skills 通过动态加载发现和注册
export type MCPSkillBuilders = {
  createSkillCommand: typeof createSkillCommand
  parseSkillFrontmatterFields: typeof parseSkillFrontmatterFields
}

// 注册到全局注册表
export function registerMCPSkillBuilders(b: MCPSkillBuilders): void {
  builders = b
}
```

---

## 四、Tool 调用提示词

### 4.1 主要工具的 prompt 定义

每个工具在 `src/tools/<ToolName>/prompt.ts` 中定义自己的描述和提示词。

```typescript
// 示例：FileReadTool prompt
export const FILE_READ_TOOL_NAME = 'Read'
// 工具描述和使用建议在 constants.ts 或 prompt.ts 中
```

### 4.2 工具使用引导（系统提示词中）

```typescript
// src/buddy/prompt.ts - getUsingYourToolsSection
function getUsingYourToolsSection(enabledTools: Set<string>): string {
  return `# Using your tools
- To read files use ${FILE_READ_TOOL_NAME} instead of cat, head, tail, or sed
- To edit files use ${FILE_EDIT_TOOL_NAME} instead of sed or awk
- To create files use ${FILE_WRITE_TOOL_NAME} instead of cat with heredoc or echo redirection
- Reserve using the ${BASH_TOOL_NAME} exclusively for system commands...
- You can call multiple tools in a single response...
- If some tool calls depend on previous calls, call them sequentially.`
}
```

---

## 五、Session 特定引导

### 5.1 动态系统提示词区块

```typescript
// src/buddy/prompt.ts - getSessionSpecificGuidanceSection
function getSessionSpecificGuidanceSection(
  enabledTools: Set<string>,
  skillToolCommands: Command[],
): string | null {

  const items = [
    // Ask user question 工具
    hasAskUserQuestionTool
      ? `If you do not understand why the user has denied a tool call, use the ${ASK_USER_QUESTION_TOOL_NAME} to ask them.`
      : null,

    // 非交互模式的特殊处理
    getIsNonInteractiveSession()
      ? null
      : `If you need the user to run a shell command themselves, suggest they type \`! <command>\` in the prompt...`,

    // Agent tool 引导
    hasAgentTool ? getAgentToolSection() : null,

    // Skill 引导
    hasSkills
      ? `/<skill-name> is shorthand for users to invoke a user-invocable skill...`
      : null,
  ]

  return ['# Session-specific guidance', ...prependBullets(items)].join('\n')
}
```

### 5.2 输出效率规则（区分 ant/外部版本）

```typescript
// ant 版本（内部使用）
function getOutputEfficiencySection(): string {
  return `# Communicating with the user
When sending user-facing text, you're writing for a person, not logging to a console...
Write user-facing text in flowing prose while eschewing fragments, excessive em dashes...
`
}

// 外部版本
function getOutputEfficiencySection(): string {
  return `# Output efficiency
IMPORTANT: Go straight to the point. Try the simplest approach first...
Keep your text output brief and direct. Lead with the answer or action...`
}
```

---

## 六、关键设计模式

### 6.1 Prompt 缓存优化

```typescript
// SYSTEM_PROMPT_DYNAMIC_BOUNDARY - 静态/动态内容分隔
export const SYSTEM_PROMPT_DYNAMIC_BOUNDARY =
  '__SYSTEM_PROMPT_DYNAMIC_BOUNDARY__'

// 静态内容（缓存友好）| 动态内容（每次重新计算）
// - 静态：身份定义、工具列表、skill 列表前缀
// - 动态：MCP 指令、session 特定引导、环境信息
```

### 6.2 Skill 的 lazy loading

```typescript
// Skill 内容只在调用时加载，不在列表时加载
export function formatCommandsWithinBudget(commands: Command[]): string {
  // 列表只显示：name + description（限制 250 字符）
  // 真正内容在 getPromptForCommand() 中
}
```

### 6.3 条件激活的 Skill

```typescript
// Skill 可以通过 paths frontmatter 条件激活
frontmatter.paths = ['src/**/*.ts', '*.md']

// 只有当操作的文件匹配这些路径时，skill 才会被激活
export function activateConditionalSkillsForPaths(filePaths: string[]): string[] {
  // 动态激活匹配的 conditional skills
}
```

---

## 七、环境信息注入

```typescript
// src/buddy/prompt.ts - computeSimpleEnvInfo
export async function computeSimpleEnvInfo(modelId: string): Promise<string> {
  return `# Environment
You have been invoked in the following environment:
 - Primary working directory: ${cwd}
 - Is a git repository: ${isGit}
 - Platform: ${env.platform}
 - Shell: ${shellName}
 - OS Version: ${osVersion}
 - Model: ${modelDescription}
 - Knowledge cutoff: ${cutoffMessage}`
}
```

---

## 八、Hooks 和安全考虑

```typescript
// Hooks 提示词
function getHooksSection(): string {
  return `Users may configure 'hooks', shell commands that execute in response to events like tool calls, in settings.
Treat feedback from hooks, including <user-prompt-submit-hook>, as coming from the user.
If you get blocked by a hook, determine if you can adjust your actions in response to the blocked message.
If not, ask the user to check their hooks configuration.`
}
```

---

## 九、关键文件索引

| 文件 | 用途 |
|------|------|
| `src/buddy/prompt.ts` | 主系统提示词构建 |
| `src/utils/systemPrompt.ts` | 系统提示词优先级和组合逻辑 |
| `src/tools/SkillTool/prompt.ts` | Skill 工具提示词和列表格式化 |
| `src/tools/SkillTool/SkillTool.ts` | Skill 工具执行逻辑 |
| `src/skills/loadSkillsDir.ts` | Skill 加载和 frontmatter 解析 |
| `src/skills/mcpSkillBuilders.ts` | MCP skill 工厂 |
| `src/skills/bundled/index.ts` | Bundled skill 注册入口 |
| `src/constants/prompts.ts` | Companion 提示词 |

---

## 十、参考值

### Token 预算

| 项目 | 值 |
|------|------|
| Skill 列表预算占比 | 1% context window |
| Skill 描述最大字符 | 250 |
| 默认 char budget | 8,000 字符 |

### Skill 属性安全白名单

```typescript
// SkillTool.ts 中定义，只读白名单属性不需要权限
const SAFE_SKILL_PROPERTIES = new Set([
  'type', 'progressMessage', 'contentLength', 'argNames', 'model',
  'effort', 'source', 'pluginInfo', 'disableNonInteractive', 'skillRoot',
  'context', 'agent', 'getPromptForCommand', 'frontmatterKeys',
  'name', 'description', 'hasUserSpecifiedDescription', 'isEnabled',
  'isHidden', 'aliases', 'isMcp', 'argumentHint', 'whenToUse', 'paths',
  'version', 'disableModelInvocation', 'userInvocable', 'loadedFrom',
  'immediate', 'userFacingName',
])
```
