# Skill Evolution Pipeline — Go 实现

## Context

SkillClaw（Python）实现了 LLM Agent 技能的集体进化流水线：捕获会话 → 提炼技能 → 跨 Agent 共享。Runner 需要在 Go 中实现完整等价功能，存储遵循现有规范（本地文件）。

**核心思想**：Runner 自己完成"会话 → 技能进化"闭环，存储本地文件。

**与现有机制的关系**：Runner 已有 `create_skill` 工具（LLM 主动创建）和 `SkillGenerator`（自动检测模式）。Evolution Pipeline 与它们互补：
- 现有机制：**快速创建** skill（单次有价值的工作流）
- Evolution Pipeline：**累积进化** skill（基于多会话证据持续改进）
- 两者共享同一套 SKILL.md 格式和存储，可共存互补

---

## 技能进化流水线架构

```
会话存储 (SQLite)
    ↓
[Stage 1] Summarize（生成轨迹 + 分析摘要）
    ↓
[Stage 2] Aggregate（按 skill 分组）
    ↓
[Stage 3] Execute（LLM 决策：improve/optimize/skip）
    ↓
SKILL.md 更新本地 skills/ 目录
```

**新 skill 创建**仍由 `create_skill` 工具实时完成，不走此流水线。

---

## 新建目录结构

```
runner/
  pkg/xqldir/
    dir.go            # 扩展：新增 GetSkillEvolutionDir 等
  skill_evolution/
    evolution.go      # EvolverServer 主循环
    session.go        # Session/Turn/Skill 数据结构
    summarizer.go     # Stage 1: Summarize
    aggregator.go     # Stage 2: Aggregate
    evolver.go        # Stage 3: Execute
    skill_md.go       # SKILL.md 读写
    prompts.go        # LLM Prompt 模板
```

---

## 核心数据结构

### Session / Turn

```go
type Turn struct {
    TurnNum      int
    PromptText   string
    ResponseText string
    ReadSkills   []string  // Agent 读取的 skill
    ToolCalls    []ToolCall
    ToolResults  []ToolResult
    PRMScore     float64   // 0.0-1.0
}

type Session struct {
    SessionID   string
    TaskID      string
    Turns       []Turn
    AvgPRM      float64
    HasErrors   bool
    Trajectory  string  // Summarize 后生成
    Summary     string  // Summarize 后生成
}
```

### Skill / EvolveResult

```go
type Skill struct {
    Name        string
    Description string
    Category    string  // general/coding/reasoning/...
    Content     string  // Markdown body
}

type EvolveResult struct {
    Action    string  // improve_skill / optimize_description / skip
    Rationale string
    Skill     *Skill
}
```

### SKILL.md 格式（兼容现有 create_skill 工具格式）

```yaml
---
name: debug-systematically
description: "Systematically debug issues by isolating variables. NOT for syntax errors."
trigger: "debug, error, fix"
version: "0.1.0"
---

# Debug Systematically

## Overview
...

## When to Use
...

## Steps
1. ...
```

与现有 `create_skill` 工具格式完全兼容，Evolution Pipeline 只需添加 `source: evolve` 或递增 `version` 字段。

---

## Stage 1: Summarize

**输入**：原始 Session
**输出**：Session 附加 Trajectory + Summary

Trajectory 示例：
```
### Session abc123, avg PRM: 0.85
[Step 1] PRM=0.85 | read_skills=['debug-systematically']
  User: How do I fix the login bug?
  Tools:
    read({"path": "/auth.py"}) → ✓
    search({"query": "login"}) → ✗ (timeout)
```

Summary：LLM 生成 8-15 句因果分析。

---

## Stage 2: Aggregate

```go
func AggregateSessions(sessions []Session) map[string][]Session {
    // skill_name → sessions
    // 无 skill → "__no_skill__" 组
}
```

---

## Stage 3: Execute

对每个 skill 组调用 LLM，决策三种 action：

| Action                 | 触发条件                           |
| ---------------------- | ---------------------------------- |
| `improve_skill`        | 有现有 skill，需基于多会话证据改进 |
| `optimize_description` | 仅优化描述                         |
| `skip`                 | 证据不足以证明需要改动             |

**注意**：不包含 `create_skill` action。新 skill 的创建仍由 `create_skill` 工具完成（实时、LLM 判断）。

**编辑约束**：
- 保守编辑，保持原有结构
- 不改 API endpoint 除非证据明确
- 不添加泛型最佳实践
- Agent 用错 API ≠ skill 有问题

---

## 本地存储（遵循 xqldir 规范）

扩展 `xqldir/dir.go`，新增目录函数：

```go
// 新增在 xqldir/dir.go
func GetSkillEvolutionDir() string {
    return filepath.Join(GetBaseDir(), "skill_evolution")
}
func GetSkillEvolutionSessionsDir() string {
    return filepath.Join(GetSkillEvolutionDir(), "sessions")
}
func GetSkillEvolutionManifestPath() string {
    return filepath.Join(GetSkillEvolutionDir(), "manifest.jsonl")
}
```

实际路径：
```
~/.xiaoqinglong/
  skill_evolution/
    sessions/              # 待进化的会话 JSON
    manifest.jsonl          # skill 注册表（evolved skill 记录）
  skills/                  # 复用现有目录（create_skill + evolution 共享）
    {skill_name}/
      SKILL.md
```

**与现有 skill 存储的关系**：
- Evolution Pipeline 直接读写 `skills/` 目录，与 `create_skill` 工具共享存储
- 进化后的 skill 通过 `version` 字段递增区分
- 存储实现：复用 `xqldir.GetSkillsDir()` + 新增 `xqldir.GetSkillEvolutionSessionsDir()`

---

## 与 Runner 现有组件集成

| 现有组件                     | 集成点                                                                     |
| ---------------------------- | -------------------------------------------------------------------------- |
| `xqldir/dir.go`              | 新增 `GetSkillEvolutionDir/ SessionsDir/ ManifestPath`，扩展 EnsureBaseDir |
| `dispatcher.go`              | 会话结束后写入 Session 到 `skill_evolution/sessions/`                      |
| `runner/llm`                 | 用已有的 ModelFactory 获取 LLM                                             |
| `plugins/skill`              | 复用 skill 加载逻辑读取 SKILL.md                                           |
| `plugins/skill_generator.go` | 复用 SkillGenerator 的模式检测；Evolution Pipeline 进化其创建的 skill      |
| `create_skill` 工具          | 共享同一 `skills/` 目录存储，Evolution Pipeline 读取并改进其创建的 skill   |

---

## 关键文件

| 文件                                   | 作用                                                 |
| -------------------------------------- | ---------------------------------------------------- |
| `runner/pkg/xqldir/dir.go`             | 新增 GetSkillEvolutionDir 等函数，更新 EnsureBaseDir |
| `runner/skill_evolution/evolution.go`  | EvolverServer，定时触发进化                          |
| `runner/skill_evolution/session.go`    | 数据结构                                             |
| `runner/skill_evolution/summarizer.go` | Stage 1                                              |
| `runner/skill_evolution/aggregator.go` | Stage 2                                              |
| `runner/skill_evolution/evolver.go`    | Stage 3 + LLM 调用                                   |
| `runner/skill_evolution/skill_md.go`   | SKILL.md 序列化                                      |
| `runner/skill_evolution/prompts.go`    | Prompt 模板                                          |
| `runner/skill_evolution/storage.go`    | 本地文件存储                                         |

---

## PRM 评分

如暂无 PRM，所有 turn 的 PRM 默认为 1.0，不影响进化流程。

---

## 验证方案

1. 构造 3-5 个测试 Session JSON
2. 运行完整流水线
3. 检查输出的 SKILL.md 格式正确
4. 检查决策合理（improve/optimize/skip）