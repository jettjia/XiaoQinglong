# 记忆系统实现计划

## Context

用户希望在现有系统中实现类似 Claude Code 的记忆系统，要求：
- **长期记忆**: 支持 user/feedback/project/reference 四种类型
- **会话记忆**: 自动提取当前对话的关键信息
- **存储**: 直接使用数据库，不再用文件
- **触发**: 自动触发

现有 `agent-frame` 已有基础的 `AgentMemory` 实体和仓库，但缺少：
- 四种类型的分类体系
- 自动提取机制（Session Memory）
- 记忆索引结构
- 与 Prompt 的集成

---

## 方案设计

### 1. 扩展记忆类型体系

修改 `agent-frame/domain/entity/memory/agent_memory_entity.go`:

```go
// 记忆类型枚举（与 Claude Code 一致）
const (
    MemoryTypeUser      = "user"      // 用户角色、偏好、知识
    MemoryTypeFeedback  = "feedback"  // 用户指导（避免什么、保持什么）
    MemoryTypeProject   = "project"   // 项目上下文（谁在做什么、为什么）
    MemoryTypeReference = "reference" // 外部系统指针
)

// AgentMemory 扩展字段
type AgentMemory struct {
    // ... 现有字段 ...
    MemoryType  string `json:"memory_type"` // user/feedback/project/reference
    Name         string `json:"name"`        // 记忆名称，如 "user_role"
    Description  string `json:"description"` // 一句话描述（用于索引）
    Source       string `json:"source"`      // private/team
    ExpiresAt    int64  `json:"expires_at"`   // 过期时间，0=永不过期
}
```

### 2. 新建记忆索引表

创建 `agent-frame/infra/repository/po/memory/memory_index_po.go`:

```go
type MemoryIndex struct {
    Ulid       string `gorm:"column:ulid;primaryKey"`
    MemoryID   string `gorm:"column:memory_id;index"`  // 关联 agent_memory.id
    HookLine   string `gorm:"column:hook_line"`         // "- [Title](file.md) — one-line hook"
    MemoryType string `gorm:"column:memory_type"`       // 索引类型，用于快速筛选
    AgentId    string `gorm:"column:agent_id;index"`
    UserId     string `gorm:"column:user_id;index"`
}
```

### 3. 实现记忆仓库接口

修改 `agent-frame/domain/irepository/memory/i_agent_memory_repo.go`:

```go
type IAgentMemoryRepo interface {
    // 现有方法...

    // 新增：按类型查询记忆
    FindByType(ctx context.Context, agentId, userId, memoryType string) ([]*AgentMemory, error)

    // 新增：保存记忆并更新索引（原子操作）
    CreateWithIndex(ctx context.Context, memory *AgentMemory) error

    // 新增：删除记忆并删除索引
    DeleteWithIndex(ctx context.Context, ulid string) error

    // 新增：获取记忆索引（用于构建 prompt）
    GetMemoryIndex(ctx context.Context, agentId, userId string) ([]*MemoryIndex, error)
}
```

### 4. 实现自动记忆提取（Session Memory）

创建 `agent-frame/domain/svc/memory/memory_svc.go`:

```go
// MemoryService 记忆业务服务
type MemoryService struct {
    repo irepository.IAgentMemoryRepo
}

// 新增：获取记忆用于 prompt 构建
func (s *MemoryService) GetMemoriesForPrompt(ctx context.Context, agentId, userId string) ([]*entity.AgentMemory, error) {
    return s.repo.FindByAgentAndUser(ctx, agentId, userId)
}

// 新增：获取记忆索引
func (s *MemoryService) GetMemoryIndex(ctx context.Context, agentId, userId string) ([]*po.MemoryIndex, error) {
    return s.repo.GetMemoryIndex(ctx, agentId, userId)
}

// 自动提取触发条件（与 Claude Code 一致）
func (s *MemoryService) ShouldExtract(messages []*types.Message) bool {
    // 1. Token 阈值：每 10000 tokens 触发一次
    // 2. Tool call 阈值：每 50 次 tool call 触发一次
    // 3. 最后一次 assistant turn 不能有 tool call（自然断点）
}

// 提取提示词（来自 Claude Code 的设计）
func (s *MemoryService) BuildExtractionPrompt(currentMemory string) string {
    // 提示 AI 从对话中提取：
    // - 用户偏好和反馈
    // - 项目上下文
    // - 需要记住的事实
    // - 避免重复的错误
}

// 核心提取逻辑：调用 LLM 执行提取
func (s *MemoryService) ExtractSessionMemory(ctx context.Context, agentId, userId, sessionId string, messages []*types.Message) error {
    // 1. 判断是否需要提取
    // 2. 读取当前记忆（用于增量更新）
    // 3. 构建提取 prompt
    // 4. 调用 LLM 提取关键信息
    // 5. 保存到数据库
}
```

### 5. 修改 Prompt 构建（集成记忆）

修改 `runner/prompt/sections.go`:

新增 `GetMemorySection()` 函数，构建记忆相关的 prompt section：

```go
func GetMemorySection(memories []*entity.AgentMemory, index []*po.MemoryIndex) string {
    if len(index) == 0 {
        return ""
    }

    var lines []string
    lines = append(lines, "# Memory")
    lines = append(lines, "You have a persistent, file-based memory system...")

    // 添加记忆类型说明
    lines = append(lines, "## Types of memory", ...)

    // 添加索引内容（截断到 200 行）
    lines = append(lines, "## MEMORY.md", ...)
    for _, idx := range index {
        lines = append(lines, idx.HookLine)
    }

    return strings.Join(lines, "\n")
}
```

### 6. 修改 Dispatcher（集成记忆）

修改 `runner/dispatcher.go`:

在 `buildSystemPrompt()` 阶段通过 Context 获取记忆服务并加载：

```go
func (d *Dispatcher) loadMemories() (string, error) {
    svc, ok := d.request.Context["memory_svc"].(*memorysvc.MemoryService)
    if !ok {
        return "", nil // 未配置记忆服务，优雅降级
    }

    memories, err := svc.GetMemoriesForPrompt(ctx, agentId, userId)
    if err != nil {
        return "", err
    }

    index, err := svc.GetMemoryIndex(ctx, agentId, userId)
    if err != nil {
        return "", err
    }

    return prompt.GetMemorySection(memories, index), nil
}

func (d *Dispatcher) buildSystemPrompt() string {
    // ... 现有逻辑 ...

    // 加载记忆（通过 Context 传入的 memory service）
    if memorySection, err := d.loadMemories(); err == nil && memorySection != "" {
        // 将记忆 section 插入到 system prompt 末尾
    }
}
```

**Context 传递方式**：

```go
// 启动入口（如 agent-frame 调用 runner 时）
memoryRepo := repo.NewAgentMemoryRepo()
memorySvc := memorysvc.NewMemoryService(memoryRepo)

req := &types.RunRequest{
    Context: map[string]any{
        "memory_svc": memorySvc,
        // ... 其他 context
    },
}
```

### 7. 触发时机

在 `Dispatcher.runAgent()` 的事件循环中定期检查：

```go
func (d *Dispatcher) runAgent(ctx context.Context, messages []adk.Message) {
    // ... 主循环 ...

    // 检查是否需要提取会话记忆
    if d.sessionMemoryExtractor.ShouldExtract(messages) {
        go func() {
            d.sessionMemoryExtractor.Extract(ctx, messages)
        }()
    }
}
```

---

## 关键文件修改

| 文件                                                            | 操作                         |
| --------------------------------------------------------------- | ---------------------------- |
| `agent-frame/domain/entity/memory/agent_memory_entity.go`       | 修改 - 添加类型体系字段      |
| `agent-frame/infra/repository/po/memory/memory_index_po.go`     | 新建 - 记忆索引 PO           |
| `agent-frame/domain/irepository/memory/i_agent_memory_repo.go`  | 修改 - 添加新方法            |
| `agent-frame/infra/repository/repo/memory/agent_memory_impl.go` | 修改 - 实现新方法            |
| `runner/prompt/sections.go`                                     | 修改 - 添加 GetMemorySection |
| `runner/dispatcher.go`                                          | 修改 - 集成记忆加载          |
| `runner/memory/session_memory.go`                               | 新建 - 会话记忆提取器        |

---

## 验证方式

1. **编译测试**: `go build` 无错误
2. **数据库验证**: 确认 `agent_memory` 和 `memory_index` 表结构正确
3. **Prompt 验证**: 检查构建的 prompt 包含记忆 section
4. **功能测试**:
   - 创建一条记忆，验证索引正确更新
   - 发送多次对话，验证自动提取触发
   - 查询记忆，验证类型筛选正确

---

## 与 Claude Code 方案对比

| 特性 | Claude Code                            | 本方案               |
| ---- | -------------------------------------- | -------------------- |
| 存储 | 文件系统 (.md)                         | 数据库               |
| 索引 | MEMORY.md 文本索引                     | memory_index 表      |
| 提取 | fork 子 agent                          | 后台 goroutine + LLM |
| 触发 | token + tool call 阈值                 | 相同                 |
| 类型 | 4 种 (user/feedback/project/reference) | 4 种 (一致)          |
| 验证 | 引用时检查文件存在                     | 引用时查数据库       |