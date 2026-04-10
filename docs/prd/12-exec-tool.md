# Claude Code 执行工具分析

基于 Claude Code 源码的 Tool 和 Skill 执行机制分析。

---

## 一、Tool 架构 (src/Tool.ts)

### 1.1 Tool 接口定义

```typescript
// src/Tool.ts - 核心 Tool 类型
export type Tool<
  Input extends AnyObject = AnyObject,
  Output = unknown,
  P extends ToolProgressData = ToolProgressData,
> = {
  name: string                    // Tool 唯一名称
  aliases?: string[]              // 向后兼容的别名
  searchHint?: string             // ToolSearch 关键词提示

  // 核心方法
  call(
    args: z.infer<Input>,
    context: ToolUseContext,
    canUseTool: CanUseToolFn,
    parentMessage: AssistantMessage,
    onProgress?: ToolCallProgress<P>,
  ): Promise<ToolResult<Output>>

  description(
    input: z.infer<Input>,
    options: {...}
  ): Promise<string>

  prompt(options: {...}): Promise<string>

  // Schema 定义
  readonly inputSchema: Input
  readonly inputJSONSchema?: ToolInputJSONSchema
  outputSchema?: z.ZodType<unknown>

  // 工具属性方法
  isConcurrencySafe(input: z.infer<Input>): boolean
  isEnabled(): boolean
  isReadOnly(input: z.infer<Input>): boolean
  isDestructive?(input: z.infer<Input>): boolean

  // 权限检查
  validateInput?(input, context): Promise<ValidationResult>
  checkPermissions(input, context): Promise<PermissionResult>

  // 渲染方法
  renderToolResultMessage?(content: Output, ...): React.ReactNode
  renderToolUseMessage?(input: Partial<Input>, ...): React.ReactNode
  renderToolUseProgressMessage?(progressMessages, ...): React.ReactNode
}
```

### 1.2 buildTool 工厂函数

```typescript
// src/Tool.ts - Tool 构建器，自动填充默认值
const TOOL_DEFAULTS = {
  isEnabled: () => true,
  isConcurrencySafe: (_input) => false,   // 默认不安全
  isReadOnly: (_input) false,              // 默认可写
  isDestructive: (_input) => false,
  checkPermissions: () => ({ behavior: 'allow' }),  // 默认允许
  toAutoClassifierInput: (_input) => '',   // 跳过分类器
  userFacingName: () => '',                // 默认使用 name
}

export function buildTool<D extends AnyToolDef>(def: D): BuiltTool<D> {
  return {
    ...TOOL_DEFAULTS,
    userFacingName: () => def.name,
    ...def,
  } as BuiltTool<D>
}
```

### 1.3 ToolUseContext - 执行上下文

```typescript
// src/Tool.ts - Tool 执行时的完整上下文
export type ToolUseContext = {
  options: {
    commands: Command[]
    tools: Tools                    // 可用工具列表
    mcpClients: MCPServerConnection[]
    maxBudgetUsd?: number
    customSystemPrompt?: string
    // ...
  }
  abortController: AbortController
  messages: Message[]               // 当前对话消息
  getAppState(): AppState
  setAppState(f: (prev: AppState) => AppState): void
  // ... 大量可选字段
}
```

---

## 二、Tool 执行流程

### 2.1 执行链路 (src/services/tools/toolExecution.ts)

```
用户/模型调用 Tool
       ↓
validateInput()    ← 验证输入格式、Tool 是否存在
       ↓
checkPermissions() ← 检查权限模式、alwaysAllow/alwaysDeny 规则
       ↓
    call()         ← 真正执行 Tool 逻辑
       ↓
   返回结果
```

### 2.2 validateInput - 输入验证

```typescript
// SkillTool.validateInput 示例
async validateInput({ skill }, context): Promise<ValidationResult> {
  const trimmed = skill.trim()
  if (!trimmed) {
    return { result: false, message: `Invalid skill format: ${skill}`, errorCode: 1 }
  }

  // 规范化命令名（去除前导 /）
  const commandName = trimmed.startsWith('/')
    ? trimmed.substring(1)
    : trimmed

  // 获取可用命令（包括 MCP skills）
  const commands = await getAllCommands(context)

  // 检查命令是否存在
  const foundCommand = findCommand(commandName, commands)
  if (!foundCommand) {
    return { result: false, message: `Unknown skill: ${commandName}`, errorCode: 2 }
  }

  // 检查是否禁用模型调用
  if (foundCommand.disableModelInvocation) {
    return { result: false, message: `Skill cannot be used...`, errorCode: 4 }
  }

  return { result: true }
}
```

### 2.3 checkPermissions - 权限检查

```typescript
// 权限检查核心逻辑
async checkPermissions({ skill, args }, context): Promise<PermissionDecision> {
  const permissionContext = context.getAppState().toolPermissionContext

  // 1. 检查 deny 规则
  const denyRules = getRuleByContentsForTool(permissionContext, Tool, 'deny')
  for (const [ruleContent, rule] of denyRules.entries()) {
    if (ruleMatches(ruleContent)) {
      return { behavior: 'deny', message: `Blocked by permission rules`, decisionReason: { type: 'rule', rule } }
    }
  }

  // 2. 检查 allow 规则
  const allowRules = getRuleByContentsForTool(permissionContext, Tool, 'allow')
  for (const [ruleContent, rule] of allowRules.entries()) {
    if (ruleMatches(ruleContent)) {
      return { behavior: 'allow', updatedInput: { skill, args }, decisionReason: { type: 'rule', rule } }
    }
  }

  // 3. 安全属性自动放行
  if (commandObj?.type === 'prompt' && skillHasOnlySafeProperties(commandObj)) {
    return { behavior: 'allow' }
  }

  // 4. 默认：询问用户
  return {
    behavior: 'ask',
    message: `Execute skill: ${commandName}`,
    suggestions: [...],
  }
}
```

### 2.4 ToolResult - 返回结构

```typescript
// src/Tool.ts
export type ToolResult<T> = {
  data: T                          // Tool 输出数据
  newMessages?: (UserMessage | AssistantMessage | AttachmentMessage | SystemMessage)[]
  // contextModifier 允许 Tool 修改后续 Tool 的执行上下文
  contextModifier?: (context: ToolUseContext) => ToolUseContext
  mcpMeta?: { _meta?: Record<string, unknown>; structuredContent?: Record<string, unknown> }
}
```

---

## 三、SkillTool 详解 (src/tools/SkillTool/SkillTool.ts)

### 3.1 Skill 调用流程

```
用户输入 "/commit" 或 "Use the /review-pr skill"
       ↓
模型识别为 Skill 调用 → 调用 SkillTool(skill: "commit")
       ↓
SkillTool.validateInput()  ← 验证 skill 存在、可调用
       ↓
SkillTool.checkPermissions()  ← 检查权限规则
       ↓
SkillTool.call() 执行：
  ├─ Inline Skill: processPromptSlashCommand() → 返回 newMessages
  └─ Forked Skill: executeForkedSkill() → 启动子 Agent
       ↓
   返回 ToolResult，newMessages 注入对话
```

### 3.2 Inline Skill 执行

```typescript
// SkillTool.call() - inline skill 路径
async call({ skill, args }, context, canUseTool, parentMessage, onProgress) {
  // 1. 获取所有可用命令（包括 MCP skills）
  const commands = await getAllCommands(context)
  const command = findCommand(commandName, commands)

  // 2. 检查是否为 fork 类型
  if (command?.type === 'prompt' && command.context === 'fork') {
    return executeForkedSkill(command, commandName, args, context, canUseTool, parentMessage, onProgress)
  }

  // 3. 处理 inline skill
  const { processPromptSlashCommand } = await import('src/utils/processUserInput/processSlashCommand.js')
  const processedCommand = await processPromptSlashCommand(commandName, args || '', commands, context)

  // 4. 返回 newMessages 和 contextModifier
  return {
    data: { success: true, commandName, allowedTools, model },
    newMessages: processedCommand.messages,  // Skill 内容注入对话
    contextModifier(ctx) {
      // 更新 allowedTools
      // 处理 model override
      // 处理 effort override
      return modifiedContext
    }
  }
}
```

### 3.3 Forked Skill 执行

```typescript
// src/tools/SkillTool/SkillTool.ts - executeForkedSkill
async function executeForkedSkill(
  command, commandName, args, context, canUseTool, parentMessage, onProgress
) {
  const agentId = createAgentId()

  // 准备 fork 上下文
  const { modifiedGetAppState, baseAgent, promptMessages, skillContent } =
    await prepareForkedCommandContext(command, args || '', context)

  // 使用 runAgent 在独立 Agent 中执行
  for await (const message of runAgent({
    agentDefinition: { ...baseAgent, effort: command.effort },
    promptMessages,
    toolUseContext: { ...context, getAppState: modifiedGetAppState },
    canUseTool,
    isAsync: false,
    querySource: 'agent:custom',
    model: command.model,
    availableTools: context.options.tools,
    override: { agentId },
  })) {
    agentMessages.push(message)
    // 报告 progress
    if (onProgress && hasToolContent(message)) {
      onProgress({ toolUseID: `skill_${parentMessage.message.id}`, data: { message, type: 'skill_progress', ... } })
    }
  }

  const resultText = extractResultText(agentMessages, 'Skill execution completed')
  return {
    data: { success: true, commandName, status: 'forked', agentId, result: resultText }
  }
}
```

### 3.4 Skill 发现机制

```typescript
// src/tools/SkillTool/SkillTool.ts - getAllCommands
async function getAllCommands(context: ToolUseContext): Promise<Command[]> {
  // 获取 MCP skills（只包含 loadedFrom === 'mcp'）
  const mcpSkills = context.getAppState().mcp.commands.filter(
    cmd => cmd.type === 'prompt' && cmd.loadedFrom === 'mcp'
  )
  if (mcpSkills.length === 0) {
    return getCommands(getProjectRoot())  // 本地 skills
  }
  // 合并本地和 MCP skills
  const localCommands = await getCommands(getProjectRoot())
  return uniqBy([...localCommands, ...mcpSkills], 'name')
}
```

---

## 四、Skill 注册机制

### 4.1 Skill 来源

| 来源 | 位置 | loadedFrom |
|------|------|-----------|
| 本地 Skills | `.claude/skills/` 或 `.claude/commands/` | `skills` |
| Bundled Skills | `src/skills/bundled/` | `bundled` |
| Plugin Skills | 插件目录 | `plugin` |
| MCP Skills | MCP 服务器 | `mcp` |
| Managed Skills | 托管配置 | `managed` |

### 4.2 Skill 加载流程

```typescript
// src/skills/loadSkillsDir.ts - createSkillCommand
export function createSkillCommand({
  skillName, displayName, description, markdownContent,
  allowedTools, argumentHint, argumentNames, whenToUse,
  model, disableModelInvocation, userInvocable,
  source, baseDir, loadedFrom, hooks,
  executionContext, agent, effort, paths, shell,
}): Command {
  return {
    type: 'prompt',
    name: skillName,
    description,
    contentLength: markdownContent.length,
    progressMessage: 'running',
    source,  // 'bundled' | 'builtin' | 'userSettings' | 'projectSettings' | 'plugin'
    loadedFrom,
    async getPromptForCommand(args, toolUseContext) {
      let finalContent = baseDir
        ? `Base directory for this skill: ${baseDir}\n\n${markdownContent}`
        : markdownContent

      // 替换参数：$ARGUMENTS, $1, $2, ...
      finalContent = substituteArguments(finalContent, argumentNames, args)

      // 替换环境变量：${CLAUDE_SKILL_DIR}, ${CLAUDE_SESSION_ID}
      finalContent = finalContent.replace(/\$\{CLAUDE_SKILL_DIR\}/g, baseDir)
      finalContent = finalContent.replace(/\$\{CLAUDE_SESSION_ID\}/g, getSessionId())

      return [{ type: 'text', text: finalContent }]
    }
  }
}
```

### 4.3 Bundled Skill 注册

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
  if (process.env.USER_TYPE !== 'ant') return  // 只在内部版本注册

  registerBundledSkill({
    name: 'verify',
    description: DESCRIPTION,
    userInvocable: true,
    files: SKILL_FILES,
    async getPromptForCommand(args) {
      const parts: string[] = [SKILL_BODY.trimStart()]
      if (args) parts.push(`## User Request\n\n${args}`)
      return [{ type: 'text', text: parts.join('\n\n') }]
    },
  })
}
```

### 4.4 Skill Frontmatter 解析

```typescript
// src/skills/loadSkillsDir.ts - parseSkillFrontmatterFields
export function parseSkillFrontmatterFields(frontmatter, markdownContent, resolvedName): {
  displayName: string | undefined
  description: string
  hasUserSpecifiedDescription: boolean
  allowedTools: string[]
  argumentHint: string | undefined
  argumentNames: string[]
  whenToUse: string | undefined
  model: ReturnType<typeof parseUserSpecifiedModel> | undefined
  disableModelInvocation: boolean
  userInvocable: boolean
  hooks: HooksSettings | undefined
  executionContext: 'fork' | undefined
  agent: string | undefined
  effort: EffortValue | undefined
  shell: FrontmatterShell | undefined
  paths: string[] | undefined
}
```

---

## 五、Skill 列表 Token 预算控制

### 5.1 预算计算

```typescript
// src/tools/SkillTool/prompt.ts
export const SKILL_BUDGET_CONTEXT_PERCENT = 0.01  // 1% context window
export const CHARS_PER_TOKEN = 4
export const DEFAULT_CHAR_BUDGET = 8_000  // Fallback: 1% of 200k × 4
export const MAX_LISTING_DESC_CHARS = 250  // 每个描述最大字符数

export function getCharBudget(contextWindowTokens?: number): number {
  if (Number(process.env.SLASH_COMMAND_TOOL_CHAR_BUDGET)) {
    return Number(process.env.SLASH_COMMAND_TOOL_CHAR_BUDGET)
  }
  if (contextWindowTokens) {
    return Math.floor(contextWindowTokens * CHARS_PER_TOKEN * SKILL_BUDGET_CONTEXT_PERCENT)
  }
  return DEFAULT_CHAR_BUDGET
}
```

### 5.2 截断策略

```typescript
// src/tools/SkillTool/prompt.ts - formatCommandsWithinBudget
export function formatCommandsWithinBudget(commands: Command[], contextWindowTokens?): string {
  const budget = getCharBudget(contextWindowTokens)

  // 1. 先尝试完整描述
  const fullTotal = fullEntries.reduce((sum, e) => sum + stringWidth(e.full), 0) + (N-1)
  if (fullTotal <= budget) return fullEntries.map(e => e.full).join('\n')

  // 2. 分区：bundled (不截断) vs rest
  const bundledIndices = new Set<number>()
  const restCommands: Command[] = []
  for (const cmd of commands) {
    if (cmd.type === 'prompt' && cmd.source === 'bundled') {
      bundledIndices.add(i)  // 保留完整描述
    } else {
      restCommands.push(cmd)  // 可能截断
    }
  }

  // 3. 计算剩余预算
  const bundledChars = ...
  const remainingBudget = budget - bundledChars
  const maxDescLen = Math.floor(remainingBudget / restCommands.length)

  // 4. 极端情况：只剩名称
  if (maxDescLen < MIN_DESC_LENGTH) {
    // bundled 保留描述，rest 只显示名称
    return commands.map((cmd, i) =>
      bundledIndices.has(i) ? fullEntries[i]!.full : `- ${cmd.name}`
    ).join('\n')
  }

  // 5. 截断非 bundled 描述
  return commands.map((cmd, i) => {
    if (bundledIndices.has(i)) return fullEntries[i]!.full
    const description = getCommandDescription(cmd)
    return `- ${cmd.name}: ${truncate(description, maxDescLen)}`
  }).join('\n')
}
```

### 5.3 Skill 内容加载时机

```
Skill 列表显示（system-reminder）     ← 只加载 name + description（≤250字）
Skill 调用时（SkillTool.call）         ← 完整加载 SKILL.md 内容
```

---

## 六、Tool 与 Skill 的区别

| 特性 | Tool | Skill |
|------|------|-------|
| 调用方式 | 模型自主调用或按名称调用 | slash command (`/name`) 或 SkillTool |
| 内容 | Tool 定义 + prompt | SKILL.md 文件内容 |
| 加载时机 | 启动时注册，按需实例化 | 调用时加载完整内容 |
| 预算控制 | Tool prompt 固定大小 | 1% context window 限制列表大小 |
| 权限控制 | alwaysAllow/alwaysDeny/ask | 继承 Tool 权限机制 + safe properties |
| 上下文修改 | contextModifier | allowedTools, model, effort |

---

## 七、Tool Prompt 引导机制

### 7.1 Tool 使用引导（不列举每个 Tool）

```typescript
// src/buddy/prompt.ts - getUsingYourToolsSection
function getUsingYourToolsSection(enabledTools: Set<string>): string {
  return `# Using your tools
- To read files use ${FILE_READ_TOOL_NAME} instead of cat, head, tail, or sed
- To edit files use ${FILE_EDIT_TOOL_NAME} instead of sed or awk
- To create files use ${FILE_WRITE_TOOL_NAME} instead of cat with heredoc or echo redirection
- Reserve using the ${BASH_TOOL_NAME} exclusively for system commands...
- You can call multiple tools in a single response...`
}
```

### 7.2 每个 Tool 的 prompt

```typescript
// src/tools/FileReadTool/prompt.ts
export const FILE_READ_TOOL_NAME = 'Read'
export const getPrompt = memoize(async (_cwd: string): Promise<string> => {
  return `Read files from the filesystem...`
})
```

---

## 八、关键文件索引

| 文件 | 用途 |
|------|------|
| `src/Tool.ts` | Tool 接口定义、buildTool 工厂 |
| `src/tools/SkillTool/SkillTool.ts` | SkillTool 实现 |
| `src/tools/SkillTool/prompt.ts` | Skill 列表格式化和预算控制 |
| `src/services/tools/toolExecution.ts` | Tool 执行引擎 |
| `src/skills/loadSkillsDir.ts` | Skill 加载和 frontmatter 解析 |
| `src/skills/bundled/index.ts` | Bundled skill 注册入口 |
| `src/utils/processUserInput/processSlashCommand.tsx` | Slash command 处理 |
| `src/commands.ts` | 命令注册入口 |

---

## 九、执行流程图

### 9.1 Tool 执行完整流程

```
API 返回 tool_use 块
       ↓
toolExecution.ts 处理
       ↓
┌─────────────────────────────────────┐
│ 1. findToolByName() 查找 Tool       │
└─────────────────────────────────────┘
       ↓
┌─────────────────────────────────────┐
│ 2. tool.validateInput() 验证输入     │
│    - 格式检查                        │
│    - Tool 存在性                     │
│    - 工具特定验证                    │
└─────────────────────────────────────┘
       ↓
┌─────────────────────────────────────┐
│ 3. tool.checkPermissions() 权限检查  │
│    - alwaysDeny 规则                 │
│    - alwaysAllow 规则                │
│    - safe properties 自动放行       │
│    - 默认 ask                        │
└─────────────────────────────────────┘
       ↓
┌─────────────────────────────────────┐
│ 4. 权限被拒绝？                      │
│    - 是 → renderToolUseRejectedMessage │
│    - 否 → 继续                       │
└─────────────────────────────────────┘
       ↓
┌─────────────────────────────────────┐
│ 5. tool.call() 执行核心逻辑          │
│    - 工具特定操作                    │
│    - 返回 ToolResult                │
└─────────────────────────────────────┘
       ↓
┌─────────────────────────────────────┐
│ 6. result.contextModifier() 修改上下文│
│    - 更新 allowedTools              │
│    - 更新 model/effort              │
└─────────────────────────────────────┘
       ↓
┌─────────────────────────────────────┐
│ 7. renderToolResultMessage() 渲染结果│
└─────────────────────────────────────┘
```

### 9.2 Skill 调用完整流程

```
用户输入 "/commit" 或模型调用 SkillTool
       ↓
validateInput() - 验证 skill 存在
       ↓
checkPermissions() - 检查权限
       ↓
call() 执行：
  ├─ Fork Skill (context: 'fork')？
  │    ├─ prepareForkedCommandContext()
  │    ├─ runAgent() 启动子 Agent
  │    └─ 返回 forked result
  │
  └─ Inline Skill
       ├─ processPromptSlashCommand()
       │    ├─ command.getPromptForCommand(args)
       │    ├─ substituteArguments() 替换参数
       │    ├─ addInvokedSkill() 注册到压缩保留
       │    └─ registerSkillHooks()
       └─ 返回 newMessages
       ↓
newMessages 注入对话 → 模型继续执行
```

---

## 十、参考值

| 项目 | 值 | 说明 |
|------|-----|------|
| Skill 列表预算 | 1% context window | ~8,000 字符（200k window） |
| Skill 描述最大 | 250 字符 | 硬上限 |
| Tool 结果最大 | 100,000 字符 | SkillTool.maxResultSizeChars |
| 安全属性自动放行 | 28 个属性 | SAFE_SKILL_PROPERTIES |
