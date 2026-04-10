# Runner 改进报告

**基于 Hermes-Agent 和 DB-GPT 分析**
**日期**: 2026/04/10

---

## 一、现状分析

### 1.1 Runner 当前能力

| 模块 | 状态 | 说明 |
|------|------|------|
| HTTP API | ✅ | /run, /agent, /resume, /stop |
| 多模型路由 | ✅ | default/rewrite/skill/summarize |
| Skills | ✅ | Python/Shell 脚本执行 |
| MCP | ✅ | SSE/stdio/HTTP |
| A2A | ✅ | Agent 间通信 |
| 沙箱执行 | ✅ | Docker/Local |
| 记忆系统 | ⚠️ | 有但不持久（依赖数据库） |
| 自动技能创建 | ❌ | 缺失 |
| 子代理并行 | ⚠️ | 有但简陋 |
| 浏览器控制 | ❌ | 缺失 |

### 1.2 对标 Hermes-Agent 差距

| Hermes-Agent | Runner | 差距分析 |
|-------------|--------|----------|
| 记忆存储 MD 文件 | 数据库 | **Runner 记忆依赖数据库，迁移复杂** |
| 自动生成 SKILL.md | 手动创建 | **Runner 缺失自动创建能力** |
| delegate_tool + batch_runner | spawn 工具 | **Runner 子代理能力较弱** |
| 浏览器 Playwright MCP | web_fetch | **Runner 缺失完整浏览器控制** |

---

## 二、改进方案

### 2.1 持久记忆增强 ⭐⭐

**目标**: 记忆跨会话持久化，不依赖数据库，支持多级隔离

**原有架构问题**：
```
Runner → 回调 agent-frame → 存入数据库
         ↑
         需要网络调用，增加延迟和复杂度
```

**改进后架构**：
```
Runner
├── ~/.xiaoqinglong/memory/        ← 直接读写，无需回调
│   ├── sessions/{session_id}/
│   ├── users/{user_id}/
│   └── agents/{agent_id}/
│
└── 可选 callback → agent-frame   ← 仅同步展示，不做存储
```

**记忆分层设计**:

| 级别 | 存储路径 | 说明 |
|------|----------|------|
| **Session** | `~/.xiaoqinglong/memory/sessions/{session_id}/` | 单次对话内的记忆 |
| **User** | `~/.xiaoqinglong/memory/users/{user_id}/` | 跨会话的用户记忆 |
| **Agent** | `~/.xiaoqinglong/memory/agents/{agent_id}/` | 跨会话的智能体记忆 |

**文件结构**:
```
~/.xiaoqinglong/memory/
├── sessions/
│   └── {session_id}/
│       ├── MEMORY.md      # Session 笔记
│       └── CONTEXT.md     # Session 上下文
├── users/
│   └── {user_id}/
│       ├── MEMORY.md      # 用户偏好、习惯
│       └── USER.md        # 用户画像
└── agents/
    └── {agent_id}/
        ├── MEMORY.md      # 智能体学到的技能
        └── SKILLS.md     # 自动创建的技能
```

**核心机制**:
- **冻结快照模式**: 会话开始时读取快照，修改立即持久化
- **文件锁**: 防止并发写入冲突（参考 Hermes）
- **安全扫描**: 检测提示注入和凭据窃取模式

**改动**:
- 新文件: `runner/memory/memstore.go`
- 新文件: `runner/memory/snapshot.go`
- 修改: `runner/dispatcher.go` - 集成 MemStore
- 修改: `runner/dispatcher_init.go` - 添加 initMemStore 方法

**工作量**: 中（2-3天）

**相关退役**：
- `memory` 字段从 RunRequest 中移除
- `memory callback URL` 机制保留但仅用于同步展示
- agent-frame 的数据库记忆表可退役
- agent-frame 记忆查询接口改为读文件

**Runner 内部自动加载**：
```go
// Runner 根据 session_id/user_id/agent_id 自动加载对应层级记忆
sessionMemory := LoadSessionMemory(sessionID)  // ~/.xiaoqinglong/memory/sessions/{session_id}/
userMemory := LoadUserMemory(userID)            // ~/.xiaoqinglong/memory/users/{user_id}/
agentMemory := LoadAgentMemory(agentID)         // ~/.xiaoqinglong/memory/agents/{agent_id}/
```

---

### 2.2 子代理并行能力 ⭐⭐

**目标**: 支持真正的并行执行，每个子代理独立运行

**改动**:
- 新文件: `runner/subagent/agent_pool.go`
  - 子代理池管理
  - 结果聚合
  - 信号量控制并发
- 修改: `runner/subagent/spawn_tool.go`
  - 添加 `spawn_parallel` 工具

**新工具: spawn_parallel**
```json
{
  "tasks": {
    "agent1": "任务1",
    "agent2": "任务2",
    "agent3": "任务3"
  },
  "max_concurrent": 3,
  "timeout": 60,
  "pool_timeout": 300
}
```

**返回结果**:
```json
{
  "total_tasks": 3,
  "completed": 2,
  "failed": 1,
  "results": [
    {"task_id": "task_1", "agent_id": "agent1", "output": "...", "duration_ms": 1234},
    {"task_id": "task_2", "agent_id": "agent2", "output": "...", "duration_ms": 2345},
    {"task_id": "task_3", "agent_id": "agent3", "error": "timeout"}
  ]
}
```

**复用现有**:
- `runner/subagent/sub_agent_manager.go` 已有基础

**工作量**: 小（2-3天）

---

### 2.3 自动技能创建 ⭐⭐⭐

**目标**: Agent 解决难题后自动生成可复用技能

**核心流程**:
```
1. Agent 完成任务
2. 检测可复用模式（工具组合、重复步骤）
3. 调用 skill-creator 生成 SKILL.md
4. 保存到 skills/ 目录
5. 下次同类问题自动触发
```

**改动**:
- 新文件: `runner/plugins/skill_generator.go`
  - 分析对话历史提取模式
  - 生成 SKILL.md 草稿
  - 检测重复工具调用序列

**技能生成逻辑**:
1. 分析工具调用序列，检测重复模式
2. 如果频率 >= 2，生成技能草稿
3. 草稿保存到 `~/.xiaoqinglong/skills/auto_*/`
4. 人工审核后可启用

**复用现有**:
- `skills/skill-creator/` 已有创建流程
- `runner/plugins/skill.go` SkillRunner

**工作量**: 中（3-5天）

---

## 三、实施计划

### 阶段一：持久记忆（2-3天）

```
Day 1:
- 创建 runner/memory/memstore.go ✓
- 创建 runner/memory/snapshot.go ✓
- 设计 Session/User/Agent 分层结构 ✓
- 实现文件读写逻辑 ✓

Day 2:
- 实现冻结快照模式 ✓
- 实现文件锁防止并发冲突 ✓
- 实现安全扫描（提示注入检测）✓

Day 3:
- 测试各层级记忆持久化
- 与 agent-frame 联调
- 文档更新
```

### 阶段二：子代理并行（2-3天）

```
Day 3-4:
- 创建 runner/subagent/agent_pool.go ✓
- 修改 spawn_tool.go 添加 spawn_parallel ✓

Day 5:
- 测试并行执行
- 性能验证
```

### 阶段三：自动技能创建（3-5天）

```
Day 6-7:
- 创建 runner/plugins/skill_generator.go ✓
- 设计模式检测算法 ✓

Day 8-9:
- 集成 skill-creator
- 实现 SKILL.md 自动生成

Day 10:
- 测试完整流程
- 优化触发时机
```

---

## 四、关键文件清单

| 文件 | 操作 | 状态 |
|------|------|------|
| `runner/memory/memstore.go` | 新建 | ✅ 已完成 |
| `runner/memory/snapshot.go` | 新建 | ✅ 已完成 |
| `runner/subagent/agent_pool.go` | 新建 | ✅ 已完成 |
| `runner/plugins/skill_generator.go` | 新建 | ✅ 已完成 |
| `runner/dispatcher.go` | 修改 | ✅ 已完成 |
| `runner/dispatcher_init.go` | 修改 | ✅ 已完成 |
| `runner/subagent/spawn_tool.go` | 修改 | ✅ 已完成 |

---

## 五、验证方式

### 5.1 持久记忆
```bash
# 1. 写入用户级记忆
curl -X POST http://localhost:18080/run \
  -d '{"user_id": "user001", "prompt": "记住我喜欢喝美式咖啡"}'

# 2. 写入 Session 级记忆
curl -X POST http://localhost:18080/run \
  -d '{"session_id": "sess001", "user_id": "user001", "prompt": "这个任务我先做A再做B"}'

# 3. 重启 Runner
docker compose down && docker compose up -d

# 4. 验证用户记忆仍在
curl -X POST http://localhost:18080/run \
  -d '{"user_id": "user001", "prompt": "我喜欢喝什么咖啡？"}'
# 期望: 回复提到美式咖啡

# 5. 验证文件结构
ls -la ~/.xiaoqinglong/memory/users/user001/
ls -la ~/.xiaoqinglong/memory/sessions/sess001/
```

### 5.2 子代理并行
```bash
# 发送多个并行任务
curl -X POST http://localhost:18080/run \
  -d '{
    "prompt": "并行执行三个任务",
    "tools": [{"name": "spawn_parallel", "args": {
      "tasks": {
        "agent-a": "任务A",
        "agent-b": "任务B",
        "agent-c": "任务C"
      }
    }}]
  }'
# 验证结果同时返回，耗时接近单个任务而非累加
```

### 5.3 自动技能创建
```bash
# 1. 让 Agent 解决一个复杂问题（涉及重复的工具调用）
# 2. 检查 skills/ 目录是否生成新技能
ls -la ~/.xiaoqinglong/skills/auto_*/

# 3. 查看生成的 SKILL.md
cat ~/.xiaoqinglong/skills/auto_xxx/SKILL.md

# 4. 复现同类问题时验证自动触发
```

---

## 六、风险与注意事项

| 风险 | 缓解措施 |
|------|----------|
| 记忆文件并发写入冲突 | 使用文件锁（参考 Hermes） |
| 自动生成的 SKILL.md 质量差 | 先生成草稿，人工审核后启用 |
| 子代理资源耗尽 | 实现池化 + 超时控制 |

---

## 七、不纳入本次改进

| 功能 | 原因 |
|------|------|
| 浏览器控制 | 用户后续单独安排 |
| 健康检查端点 | 用户表示不需要 |
| Prometheus Metrics | 用户表示不需要 |
| Graceful Shutdown | 用户表示不需要 |
| Rate Limiting | 非本次重点 |
| API Key 认证 | 非本次重点 |

---

## 八、已完成的代码变更

### 新建文件

```
backend/runner/memory/memstore.go       # 记忆文件存储
backend/runner/memory/snapshot.go          # 冻结快照模式
backend/runner/subagent/agent_pool.go     # 子代理池
backend/runner/plugins/skill_generator.go  # 自动技能创建
```

### 修改文件

```
backend/runner/pkg/xqldir/dir.go           # 添加 GetMemoryDir 等方法
backend/runner/dispatcher.go               # 添加 memStore 集成
backend/runner/dispatcher_init.go          # 添加 initMemStore 方法
backend/runner/subagent/spawn_tool.go      # 添加 spawn_parallel 工具
```

---

**报告生成时间**: 2026/04/10
**预计总工期**: 7-11 天
**实际完成情况**: 核心代码已完成（测试待验证）