# Runner Executor 改进计划

## Context

基于 Claude Code 源码分析 (`tmp/frontend/claude-clode-sourcecode`) 和当前 Runner Executor 实现 (`backend/runner/`)，识别出以下可改进的领域。

当前 Runner 已有：基础 Tool 系统、Skill 执行器（沙箱+adk）、MCP 集成、A2A 代理、上下文压缩、审批机制。

缺少：validateInput 分离、contextModifier、shouldDefer 延迟加载、Skill 权限白名单、`/loop` 定时任务、Skill token 预算控制等。

---

## 一、Tool 系统增强

### 1.1 添加 `validateInput()` 分离

**现状**：Tool 的输入验证和权限检查混在一起。

**目标**：将验证逻辑分为两部分：
- `validateInput()` - 验证输入格式、Tool 存在性
- `checkPermissions()` - 检查权限规则

**参考**：`tmp/frontend/claude-clode-sourcecode/src/Tool.ts` 的 `validateInput` 和 `SkillTool.ts` 的 `checkPermissions`

**文件**：`backend/runner/plugins/tool.go`

```go
// 添加到 tool.BaseTool 接口
type ToolWithValidation interface {
    ValidateInput(ctx context.Context, args string) error  // 返回具体错误信息
    CheckPermissions(ctx context.Context, args string) (bool, string)  // 是否允许，拒绝原因
}
```

### 1.2 添加 `contextModifier` 支持

**现状**：Tool 执行后无法修改后续执行上下文。

**目标**：Tool 可以返回 context modifier 来更新 `allowedTools`、`model`、`effort` 等。

**参考**：`tmp/frontend/claude-clode-sourcecode/src/Tool.ts` 的 `contextModifier` 字段

**文件**：`backend/runner/plugins/tool.go`

```go
type ToolContextModifier func(ctx *ToolContext) *ToolContext

type ToolResult struct {
    Output   string
    NewMessages []Message
    ContextModifier ToolContextModifier  // 执行后修改上下文
}
```

### 1.3 添加 `shouldDefer` 延迟加载

**现状**：所有 Tool 在启动时全部加载。

**目标**：支持 `shouldDefer=true` 的 Tool 在首次使用时才加载。

**参考**：`tmp/frontend/claude-clode-sourcecode/src/Tool.ts` 的 `shouldDefer` 字段

**文件**：`backend/runner/dispatcher.go`

```go
type ToolConfig struct {
    ShouldDefer bool  `json:"should_defer"`
    // ...
}

// initTools 修改为按需加载
func (d *Dispatcher) getTool(name string) (tool.BaseTool, error) {
    if t, ok := d.toolsMap[name]; ok {
        return t, nil
    }
    if d.deferredTools[name] != nil {
        // 加载 deferred tool
        t := d.deferredTools[name].Load()
        d.toolsMap[name] = t
        return t, nil
    }
    return nil, fmt.Errorf("tool not found: %s", name)
}
```

---

## 二、Skill 系统增强

### 2.1 添加 Skill `whenToUse` 和 `allowedTools` frontmatter

**现状**：Skill 配置简单，只有 `Instruction`。

**目标**：支持丰富的 frontmatter 元数据。

**参考**：`tmp/frontend/claude-clode-sourcecode/src/skills/loadSkillsDir.ts` 的 `parseSkillFrontmatterFields`

**文件**：`backend/runner/types/types.go`

```go
type Skill struct {
    ID          string   `json:"id"`
    Name        string   `json:"name"`
    Description string   `json:"description"`
    Instruction string   `json:"instruction"`

    // 新增字段
    WhenToUse   string   `json:"when_to_use"`    // 帮助模型判断何时调用
    AllowedTools []string `json:"allowed_tools"`  // 临时授权的工具列表
    Model       string   `json:"model"`          // 模型覆盖
    Effort      string   `json:"effort"`          // effort 级别
    Paths       []string `json:"paths"`           // 条件激活路径
}
```

### 2.2 添加 Skill 权限安全白名单

**现状**：Skill 执行时需要用户显式授权。

**目标**：只有非安全属性的 Skill 才需要授权。

**参考**：`tmp/frontend/claude-clode-sourcecode/src/tools/SkillTool/SkillTool.ts` 的 `SAFE_SKILL_PROPERTIES`

**文件**：`backend/runner/plugins/skill.go`

```go
var SafeSkillProperties = map[string]bool{
    "type": true, "progressMessage": true, "model": true,
    "effort": true, "source": true, "context": true,
    "agent": true, "name": true, "description": true,
    "instruction": true, "whenToUse": true, "paths": true,
}

func isSkillSafe(skill *Skill) bool {
    // 检查 skill 是否只使用安全属性
    // 只有安全 skill 才自动授权
}
```

### 2.3 Skill 内容懒加载

**现状**：Skill 列表时加载完整内容。

**目标**：Skill 列表只显示 name + description（限制 250 字符），内容在调用时才加载。

**参考**：`tmp/frontend/claude-clode-sourcecode/src/tools/SkillTool/prompt.ts` 的 `MAX_LISTING_DESC_CHARS = 250`

**文件**：`backend/runner/plugins/skill.go`

```go
const MaxSkillDescChars = 250

func (r *SkillRunner) SkillListText() string {
    // 列表只显示 name + description（限制 250 字符）
    // 完整内容在 BuildLoadSkillTool 中按需加载
}
```

---

## 三、添加 `/loop` 定时任务功能

### 3.1 Cron 调度系统

**现状**：没有定时任务功能。

**目标**：实现类似 Claude Code 的 `/loop` skill。

**参考**：`tmp/frontend/claude-clode-sourcecode/src/tools/ScheduleCronTool/` 和 `src/skills/bundled/loop.ts`

**文件**：`backend/runner/cron/`

```
cron/
  scheduler.go    # 调度器主循环
  tasks.go        # 任务存储（内存 + 磁盘持久化）
  cron.go         # Cron 表达式解析
  prompt.go       # /loop skill 的 prompt 模板
```

### 3.2 核心功能

```go
type CronTask struct {
    ID          string    // 任务 ID
    Cron        string    // 5字段 cron 表达式
    Prompt      string    // 触发时执行的 prompt
    Recurring   bool      // 是否循环
    Durable     bool      // 是否持久化到磁盘
    CreatedAt   int64     // 创建时间
    LastFiredAt int64     // 上次触发时间
    AgentId     string    // 关联的 agent ID
}

const (
    MaxJobs = 50              // 最大任务数限制
    DefaultMaxAgeDays = 7     // 循环任务 7 天后自动过期
)
```

### 3.3 /loop Skill 解析

```go
// Interval → Cron 转换规则
// Nm (N ≤ 59)  → */N * * * *
// Nm (N ≥ 60)  → 0 */H * * *  (转为小时)
// Nh (N ≤ 23)  → 0 */N * * *
// Nd           → 0 0 */N * *
```

---

## 四、Command 系统增强

### 4.1 添加 `availability` 过滤

**现状**：所有 command 都对所有用户可见。

**目标**：根据用户类型过滤 command。

**参考**：`tmp/frontend/claude-clode-sourcecode/src/commands.ts` 的 `meetsAvailabilityRequirement`

**文件**：`backend/runner/types/types.go`

```go
type CommandAvailability string
const (
    AvailabilityAll       CommandAvailability = ""
    AvailabilityClaudeAI  CommandAvailability = "claude-ai"
    AvailabilityConsole   CommandAvailability = "console"
)

type Command struct {
    ID           string
    Name         string
    Availability CommandAvailability  // 可见性过滤
    // ...
}
```

### 4.2 添加远程/桥接安全标记

**目标**：某些 command 只能在本地或特定渠道使用。

**参考**：`tmp/frontend/claude-clode-sourcecode/src/commands.ts` 的 `REMOTE_SAFE_COMMANDS` 和 `BRIDGE_SAFE_COMMANDS`

**文件**：`backend/runner/types/types.go`

```go
type Command struct {
    // ...
    RemoteSafe bool  // 是否在 --remote 模式安全
    BridgeSafe bool  // 是否在桥接协议中安全
}
```

---

## 五、Prompt 系统增强

### 5.1 结构化 Prompt 构建

**现状**：`buildSystemPrompt()` 方法简单拼接。

**目标**：分层构建 prompt，支持动态部分按需更新。

**参考**：`tmp/frontend/claude-clode-sourcecode/src/buddy/prompt.ts` 的分段结构

**文件**：`backend/runner/prompt/`

```
prompt/
  builder.go      # Prompt 构建器
  sections.go     # 各区块定义
  templates.go    # 模板字符串
```

### 5.2 Prompt 区块

```go
type PromptBuilder struct {
    sections []PromptSection
}

type PromptSection struct {
    Name    string
    Content string
    Dynamic bool  // 动态区块每次重新计算
}

// 区块列表
// - IntroSection: 身份定义
// - SystemSection: 系统规则
// - DoingTasksSection: 任务执行规则
// - ActionsSection: 危险操作确认
// - UsingYourToolsSection: 工具使用引导
// - OutputEfficiencySection: 输出效率
// - ToneAndStyleSection: 语气风格
// - SkillsSection: 可用技能列表（动态）
// - McpSection: MCP 指令（动态）
```

---

## 六、实现顺序

### Phase 1: Tool 系统增强（1-2 周）
1. 添加 `validateInput()` 和 `checkPermissions()` 分离
2. 添加 `contextModifier` 支持
3. 添加 `shouldDefer` 延迟加载

### Phase 2: Skill 系统增强（1 周）
1. 添加 `whenToUse` 和 `allowedTools` frontmatter
2. 添加 Skill 权限安全白名单
3. Skill 内容懒加载

### Phase 3: Cron 定时任务（2 周）
1. 实现 cron 表达式解析
2. 实现任务存储（内存 + 磁盘）
3. 实现调度器
4. 实现 `/loop` skill

### Phase 4: Command/Prompt 增强（1 周）
1. 添加 `availability` 过滤
2. 添加远程/桥接安全标记
3. 结构化 Prompt 构建

---

## 七、关键文件

| 文件 | 修改类型 |
|------|----------|
| `backend/runner/plugins/tool.go` | 修改 |
| `backend/runner/types/types.go` | 修改 |
| `backend/runner/dispatcher.go` | 修改 |
| `backend/runner/plugins/skill.go` | 修改 |
| `backend/runner/cron/scheduler.go` | 新增 |
| `backend/runner/cron/tasks.go` | 新增 |
| `backend/runner/cron/cron.go` | 新增 |
| `backend/runner/prompt/builder.go` | 新增 |
| `backend/runner/prompt/sections.go` | 新增 |

---

## 八、验证方式

1. **Unit Tests**：为新增功能编写单元测试
2. **集成测试**：创建测试用例验证 Tool 验证链、Skill 权限、Cron 调度
3. **手动测试**：
   - 使用 `/loop 5m check the deploy` 验证定时任务
   - 使用高风险 Tool 验证审批流程
   - 验证 Skill 懒加载和 token 预算
