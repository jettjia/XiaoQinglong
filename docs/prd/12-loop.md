# Claude Code Loop/定时任务分析

基于 Claude Code 源码的定时循环任务机制分析。

---

## 一、概述

Claude Code 的 Loop 模式是通过 `/loop` skill 和 Cron 调度系统实现的，允许用户设置**定时循环执行**的任务。

### 1.1 核心组件

| 组件 | 文件 | 用途 |
|------|------|------|
| `/loop` skill | `src/skills/bundled/loop.ts` | 用户入口，解析参数 |
| CronCreateTool | `src/tools/ScheduleCronTool/CronCreateTool.ts` | 创建定时任务 |
| CronDeleteTool | `src/tools/ScheduleCronTool/CronDeleteTool.ts` | 删除定时任务 |
| CronListTool | `src/tools/ScheduleCronTool/CronListTool.ts` | 列出定时任务 |
| cronTasks.ts | `src/utils/cronTasks.ts` | 任务持久化存储 |
| cronScheduler.ts | `src/utils/cronScheduler.ts` | 调度器实现 |
| cron.ts | `src/utils/cron.ts` | Cron 表达式解析 |

---

## 二、/loop Skill (src/skills/bundled/loop.ts)

### 2.1 Skill 定义

```typescript
// src/skills/bundled/loop.ts
export function registerLoopSkill(): void {
  registerBundledSkill({
    name: 'loop',
    description: 'Run a prompt or slash command on a recurring interval',
    whenToUse: 'When the user wants to set up a recurring task, poll for status...',
    argumentHint: '[interval] <prompt>',
    userInvocable: true,
    isEnabled: isKairosCronEnabled,  // Feature gate
    async getPromptForCommand(args) {
      // 解析参数并构建 prompt
    }
  })
}
```

### 2.2 使用方式

```
/loop 5m /babysit-prs        # 每5分钟执行 /babysit-prs
/loop 30m check the deploy   # 每30分钟执行 "check the deploy"
/loop 1h /standup 1          # 每小时执行 /standup 1
/loop check the deploy       # 默认10分钟执行
/loop check every 20m        # 同上
```

### 2.3 参数解析规则

```typescript
// 优先级顺序：
1. Leading token 规则
   - 输入 "5m /babysit-prs"
   - 匹配 ^\d+[smhd]$ → interval = "5m"
   - 剩余为 prompt = "/babysit-prs"

2. Trailing "every" 规则
   - 输入 "check the deploy every 20m"
   - 匹配 "every <N><unit>" → interval = "20m"
   - prompt = "check the deploy"

3. Default 规则
   - 无时间修饰 → interval = "10m", prompt = 完整输入
```

### 2.4 Interval → Cron 转换

| 输入格式 | Cron 表达式 | 说明 |
|----------|-------------|------|
| `Nm` (N ≤ 59) | `*/N * * * *` | 每 N 分钟 |
| `Nm` (N ≥ 60) | `0 */H * * *` | 转为小时（H = N/60） |
| `Nh` (N ≤ 23) | `0 */N * * *` | 每 N 小时 |
| `Nd` | `0 0 */N * *` | 每 N 天午夜 |
| `Ns` | `ceil(N/60)m` | 秒转分钟（最小1分钟） |

---

## 三、Cron 调度系统

### 3.1 任务类型

```typescript
// src/utils/cronTasks.ts
export type CronTask = {
  id: string
  cron: string              // 5字段 cron 表达式（本地时间）
  prompt: string            // 触发时执行的 prompt
  createdAt: number          // 创建时间戳
  lastFiredAt?: number       // 上次触发时间
  recurring?: boolean        // 是否循环（false = 一次性）
  permanent?: boolean        // 永不过期（系统任务）
  durable?: boolean         // 是否持久化到磁盘
  agentId?: string          // 关联的 agent ID
}
```

### 3.2 持久化机制

```typescript
// 存储位置: <project>/.claude/scheduled_tasks.json
// 格式:
{
  "tasks": [
    { id, cron, prompt, createdAt, lastFiredAt?, recurring?, permanent? }
  ]
}

// 两种模式：
// 1. durable: true → 写入 .claude/scheduled_tasks.json，跨会话持久化
// 2. durable: false → 仅内存存储，会话结束丢失
```

### 3.3 CronCreateTool

```typescript
// src/tools/ScheduleCronTool/CronCreateTool.ts
const inputSchema = z.strictObject({
  cron: z.string().describe('5字段 cron 表达式'),
  prompt: z.string().describe('触发时执行的 prompt'),
  recurring: z.boolean().optional().describe('true=循环, false=一次性'),
  durable: z.boolean().optional().describe('true=持久化, false=会话级'),
})

export const CronCreateTool = buildTool({
  name: 'CronCreate',
  shouldDefer: true,  // 延迟加载，需要 ToolSearch
  isEnabled() {
    return isKairosCronEnabled()  // Feature gate
  },
  async validateInput(input) {
    // 验证 cron 表达式
    // 检查任务数量限制（MAX_JOBS = 50）
  },
  async call({ cron, prompt, recurring, durable }) {
    const task = await addCronTask({
      id: randomUUID(),
      cron,
      prompt,
      createdAt: Date.now(),
      recurring: recurring ?? true,
      durable: durable ?? false,
    })
    return { data: { id: task.id, humanSchedule: cronToHuman(cron), ... }}
  }
})
```

### 3.4 调度器行为

```typescript
// CronCreatePrompt 中的关键规则：

// 1. 时间分散策略（避免惊群效应）
"every morning around 9" → "57 8 * * *" 或 "3 9 * * *" (不是 "0 9")
"hourly" → "7 * * * *" (不是 "0 *")

// 2. 循环任务自动过期
// 循环任务在 DEFAULT_MAX_AGE_DAYS (7天) 后自动过期
// - 触发一次最终执行
// - 然后删除

// 3. 触发时机
// - 只在 REPL 空闲时触发（mid-query 不触发）
// - 添加确定性 jitter：循环任务最多延迟 10% 周期（最多15分钟）
// - 一次性任务在 :00/:30 可最多提前 90 秒
```

---

## 四、Feature Gate 机制

### 4.1 特性开关

```typescript
// src/tools/ScheduleCronTool/prompt.ts
export function isKairosCronEnabled(): boolean {
  return feature('AGENT_TRIGGERS')
    ? !isEnvTruthy(process.env.CLAUDE_CODE_DISABLE_CRON) &&
        getFeatureValue_CACHED_WITH_REFRESH('tengu_kairos_cron', true, 5 * 60 * 1000)
    : false
}

// 独立于 KAIROS 模式：
// - AGENT_TRIGGERS 是构建时开关
// - tengu_kairos_cron 是运行时 GrowthBook 开关
// - CLAUDE_CODE_DISABLE_CRON 是环境变量覆盖
```

### 4.2 为什么这样设计

```
AGENT_TRIGGERS (build-time)
    │
    ├──= off → cron 完全禁用（tree-shaking 去除代码）
    │
    └──= on
          │
          ├── CLAUDE_CODE_DISABLE_CRON=true → 禁用
          │
          └── GrowthBook 'tengu_kairos_cron'=false → 禁用（kill switch）
                    │
                    └── 以上都通过 → 启用
```

---

## 五、完整执行流程

```
用户输入 "/loop 5m check the deploy"
       ↓
SkillTool 识别为 /loop skill
       ↓
processPromptSlashCommand() 处理
       ↓
buildPrompt() 构建解析 prompt：
  "Parse interval '5m' → cron '*/5 * * * *'
   Parse prompt 'check the deploy'
   Call CronCreateTool with cron, prompt, recurring: true"
       ↓
CronCreateTool.call():
  ├─ validateInput() - 验证 cron 表达式
  ├─ addCronTask() - 添加到任务列表
  └─ 返回 { id, humanSchedule: "every 5 minutes", ... }
       ↓
/loop skill 确认并立即执行一次
       ↓
后台 cronScheduler 监控任务触发
       ↓
到达触发时间：
  ├─ cronScheduler 唤醒
  ├─ 检查任务是否存活（isKilled）
  ├─ 将 prompt 注入 REPL 队列
  └─ 更新 lastFiredAt
       ↓
REPL 空闲时执行 prompt
```

---

## 六、关键文件索引

| 文件 | 用途 |
|------|------|
| `src/skills/bundled/loop.ts` | /loop skill 实现 |
| `src/tools/ScheduleCronTool/CronCreateTool.ts` | 创建定时任务 |
| `src/tools/ScheduleCronTool/CronDeleteTool.ts` | 删除定时任务 |
| `src/tools/ScheduleCronTool/CronListTool.ts` | 列出定时任务 |
| `src/tools/ScheduleCronTool/prompt.ts` | Tool prompt 和 feature gate |
| `src/utils/cronTasks.ts` | 任务存储（read/write cron_tasks.json） |
| `src/utils/cronScheduler.ts` | 调度器主循环 |
| `src/utils/cron.ts` | Cron 表达式解析 |
| `src/hooks/useScheduledTasks.ts` | React hook for UI |

---

## 七、设计亮点

### 7.1 延迟加载

```typescript
// CronCreateTool.shouldDefer = true
// 工具在首次使用时才加载完整 schema
// 减少启动时的工具列表大小
```

### 7.2 内存/持久化双模式

```typescript
// 会话级（默认）
durable: false → 任务仅在内存中，会话结束丢失

// 持久化
durable: true → 写入 .claude/scheduled_tasks.json
           → 重启后自动恢复
           → 支持错过触发检测（catch-up）
```

### 7.3 安全的默认行为

```typescript
// 默认 interval 是 10m（不是更短）
// 默认 recurring 是 true（不是 false）
// 默认 durable 是 false（不是 true）

// 用户必须明确要求才会持久化
```

### 7.4 防止系统过载

```typescript
// MAX_JOBS = 50 限制任务数量
// 避免用户创建过多定时任务

// 自动过期（7天）
// bounds session lifetime
```
