# Sub-Agent 改进计划

## 1. 背景与目标

### DeerFlow 模式参考
- **Lead Agent**: 负责任务分解、结果聚合
- **动态拉起**: Lead Agent 按需动态创建 Sub-Agent
- **独立上下文**: 每个 Sub-Agent 有自己独立的上下文、工具集、终止条件
- **并行执行**: 多个 Sub-Agent 可同时运行
- **结构化结果**: Sub-Agent 返回结构化结果（JSON）
- **结果聚合**: Lead Agent 汇总各 Sub-Agent 结果生成最终输出

### 当前 Runner 问题
| 问题 | 说明 |
|------|------|
| internal_agents 不可用 | 创建后存储在 map，但主 Agent 无法调用 |
| 缺少委托机制 | 没有 transfer/delegate 工具 |
| 无并行执行 | Sub-Agent 串行执行 |
| 无结果聚合 | 缺少结构化输出和聚合逻辑 |

---

## 2. 改进方案

### Phase 1: 基础架构 - Sub-Agent 生命周期管理

#### 2.1 Sub-Agent 配置结构

```json
"sub_agents": [
    {
        "id": "researcher",
        "name": "研究Agent",
        "description": "负责信息检索和研究",
        "prompt": "你是一个研究助手，负责...",

        // 独立的模型配置（可选，默认使用主模型）
        "model": {
            "provider": "openai",
            "name": "gpt-4o-mini",
            "api_key": "sk-xxx"
        },

        // Sub-Agent 专用工具（独立于主 Agent）
        "tools": ["web_search", "file_read"],
        "mcps": ["arxiv_mcp"],
        "skills": ["data_analysis"],

        // 执行配置
        "max_iterations": 5,
        "timeout_ms": 60000,

        // 终止条件
        "termination_condition": {
            "type": "max_iterations" | "output_length" | "custom",
            "threshold": 1000
        }
    }
]
```

#### 2.2 Sub-Agent 运行时结构

```go
// sub_agent.go
type SubAgent struct {
    ID       string
    Name     string
    Agent    adk.Agent
    Config   *SubAgentConfig
    Status   AgentStatus // running, completed, failed
    Result   *SubAgentResult
    CancelFn context.CancelFunc
}

type SubAgentResult struct {
    AgentID    string
    Status     string
    Output     string
    StructuredOutput map[string]any  // 结构化输出
    TokensUsed int
    LatencyMs  int64
    Error      string
}
```

#### 2.3 Sub-Agent 生命周期管理

```go
// sub_agent_manager.go
type SubAgentManager struct {
    agents    map[string]*SubAgent      // id -> SubAgent
    results   map[string]*SubAgentResult // id -> result
    mu        sync.RWMutex
}

func (m *SubAgentManager) Create(ctx context.Context, cfg *SubAgentConfig) (*SubAgent, error)
func (m *SubAgentManager) Run(subAgent *SubAgent, task string) error  // 异步执行
func (m *SubAgentManager) Wait(subAgent *SubAgent) (*SubAgentResult, error)
func (m *SubAgentManager) Cancel(subAgentID string) error
func (m *SubAgentManager) GetResult(subAgentID string) (*SubAgentResult, error)
func (m *SubAgentManager) WaitAll() ([]*SubAgentResult, error)  // 并行等待
```

---

### Phase 2: 委托机制 - Lead Agent 调用 Sub-Agent

#### 2.4 Delegate 工具

让 Lead Agent 可以动态委托任务给 Sub-Agent：

```json
// 在 tools 中添加 delegate 工具
{
    "type": "delegate",
    "name": "delegate_to_agent",
    "description": "委托任务给子Agent，Sub-Agent会独立执行并返回结构化结果",

    // 预定义的 Sub-Agent 列表（运行时从配置填充）
    "available_agents": ["researcher", "coder", "reviewer"],

    "risk_level": "low"
}
```

**Delegate 工具 schema**:
```go
type DelegateInput struct {
    AgentID    string  `json:"agent_id"`     // 委托给哪个 agent
    Task       string  `json:"task"`         // 任务描述
    Context    string  `json:"context"`      // 额外上下文
    WaitResult bool    `json:"wait"`         // 是否等待结果（false 则异步）
}
```

**Delegate 工具响应**:
```go
type DelegateOutput struct {
    SubAgentID string            `json:"sub_agent_id"`  // 分配的 Sub-Agent ID
    Status     string            `json:"status"`        // "running", "completed", "failed"
    Result     *SubAgentResult   `json:"result"`        // 同步模式时直接返回
    AsyncHandle string          `json:"async_handle"` // 异步模式时返回 handle，用于后续查询
}
```

#### 2.5 Delegate 工具实现

```go
// delegate_tool.go
func (t *DelegateTool) InvokableRun(ctx context.Context, argumentsInJSON string) (string, error) {
    var input DelegateInput
    json.Unmarshal(argumentsInJSON, &input)

    // 1. 创建/获取 Sub-Agent
    subAgent, err := t.manager.GetOrCreate(ctx, input.AgentID)
    if err != nil {
        return "", err
    }

    // 2. 判断同步/异步模式
    if input.WaitResult {
        // 同步模式：等待执行完成
        err := subAgent.Run(ctx, input.Task)
        result, _ := subAgent.GetResult()
        return json.Marshal(result)
    } else {
        // 异步模式：立即返回 handle
        go subAgent.Run(context.Background(), input.Task)
        return json.Marshal(map[string]string{
            "sub_agent_id": subAgent.ID,
            "status": "running",
            "async_handle": subAgent.ID,
        })
    }
}
```

---

### Phase 3: 并行执行 - 多 Sub-Agent 协作

#### 2.6 Spawn 工具（并行拉起）

```json
{
    "type": "spawn",
    "name": "spawn_agents",
    "description": "并行拉起多个Sub-Agent同时执行任务",
    "risk_level": "medium"
}
```

**Spawn 工具 schema**:
```go
type SpawnInput struct {
    Tasks []struct {
        AgentID string `json:"agent_id"`
        Task    string `json:"task"`
    } `json:"tasks"`
    WaitAll bool `json:"wait_all"` // 是否等待所有完成
}
```

**Spawn 执行流程**:
```go
func (t *SpawnTool) InvokableRun(ctx context.Context, argumentsInJSON string) (string, error) {
    var input SpawnInput
    json.Unmarshal(argumentsInJSON, &input)

    // 并行启动所有 Sub-Agent
    var wg sync.WaitGroup
    results := make(map[string]*SubAgentResult)

    for _, task := range input.Tasks {
        wg.Add(1)
        go func(t struct{AgentID, Task}) {
            defer wg.Done()
            subAgent, _ := t.manager.GetOrCreate(ctx, t.AgentID)
            subAgent.Run(ctx, t.Task)
            results[t.AgentID] = subAgent.GetResult()
        }(task)
    }

    // 等待所有完成
    wg.Wait()

    return json.Marshal(results)
}
```

#### 2.7 结果聚合工具

```json
{
    "type": "aggregate",
    "name": "aggregate_results",
    "description": "聚合多个Sub-Agent的结果，生成最终报告",
    "risk_level": "low"
}
```

---

### Phase 4: 结构化输出

#### 2.8 Sub-Agent 结果格式

```go
type SubAgentResult struct {
    AgentID         string            `json:"agent_id"`
    AgentName      string            `json:"agent_name"`
    Status         string            `json:"status"` // running, completed, failed, timeout
    Output         string            `json:"output"` // 原始文本输出

    // 结构化输出（关键！）
    StructuredOutput map[string]any  `json:"structured_output"`

    // 执行统计
    TokensUsed      int              `json:"tokens_used"`
    LatencyMs       int64            `json:"latency_ms"`
    Iterations      int              `json:"iterations"`
    ToolCallsCount  int              `json:"tool_calls_count"`

    // 错误信息
    Error           string            `json:"error,omitempty"`
    ErrorCode       string            `json:"error_code,omitempty"`
}
```

**StructuredOutput 示例（研究 Agent）**:
```json
{
    "findings": [
        {
            "topic": "AI发展历史",
            "summary": "...",
            "sources": ["source1", "source2"],
            "confidence": 0.9
        }
    ],
    "conclusion": "...",
    "next_steps": ["..."]
}
```

---

### Phase 5: 状态管理与中断恢复

#### 2.9 Sub-Agent 状态持久化

```go
// 与 CheckpointStore 集成
type SubAgentState struct {
    ID           string
    Config       *SubAgentConfig
    Status       AgentStatus
    Messages     []adk.Message
    Result       *SubAgentResult
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

---

## 3. 配置示例

### 3.1 完整配置（test-all.json 改进版）

```json
{
    "endpoint": "http://localhost:18080/run",
    "models": {
        "default": {
            "provider": "openai",
            "name": "gpt-4o",
            "api_key": "${OPENAI_API_KEY}"
        }
    },
    "system_prompt": "你是一个任务分解专家。当面对复杂任务时，你需要分解任务并委托给合适的Sub-Agent。",


    // ========== Sub-Agents ==========
    "sub_agents": [
        {
            "id": "researcher",
            "name": "研究Agent",
            "description": "负责信息检索、文献研究",
            "prompt": "你是一个研究助手，擅长搜索和分析信息。",
            "tools": ["web_search", "file_read"],
            "max_iterations": 5,
            "timeout_ms": 120000
        },
        {
            "id": "coder",
            "name": "编码Agent",
            "description": "负责代码编写和调试",
            "prompt": "你是一个编程专家，擅长代码开发。",
            "tools": ["code_interpreter", "bash"],
            "max_iterations": 10,
            "timeout_ms": 300000
        },
        {
            "id": "reviewer",
            "name": "审核Agent",
            "description": "负责代码和文档审核",
            "prompt": "你是一个审核专家，负责检查质量和一致性。",
            "tools": ["code_interpreter"],
            "max_iterations": 3,
            "timeout_ms": 60000
        }
    ],

    // ========== Tools ==========
    "tools": [
        {
            "type": "delegate",
            "name": "delegate_to_agent",
            "description": "委托任务给指定的Sub-Agent",
            "risk_level": "low"
        },
        {
            "type": "spawn",
            "name": "spawn_agents",
            "description": "并行启动多个Sub-Agent",
            "risk_level": "medium"
        },
        {
            "type": "http",
            "name": "web_search",
            "description": "网络搜索",
            "endpoint": "...",
            "method": "GET",
            "risk_level": "low"
        }
    ],

    "options": {
        "max_sub_agents": 5,
        "sub_agent_timeout_ms": 300000,
        "parallel_sub_agents": true
    }
}
```

---

## 4. 实施计划

### Phase 1: 核心框架（1-2周）
- [ ] SubAgent 结构体和 SubAgentManager
- [ ] Sub-Agent 生命周期管理（创建、运行、取消）
- [ ] 基础 Delegate 工具

### Phase 2: 委托机制（1周）
- [ ] Delegate 工具完整实现
- [ ] 与主 Agent 集成
- [ ] 单元测试

### Phase 3: 并行执行（1周）
- [ ] Spawn 工具实现
- [ ] 并行执行管理
- [ ] 结果聚合

### Phase 4: 结构化输出（1周）
- [ ] StructuredOutput 格式定义
- [ ] Sub-Agent 输出标准化
- [ ] 结果聚合工具

### Phase 5: 状态持久化（1周）
- [ ] CheckpointStore 集成
- [ ] 中断恢复
- [ ] 端到端测试

---

## 5. 风险与注意事项

1. **并发控制**: 最多同时运行的 Sub-Agent 数量需要限制
2. **资源隔离**: 每个 Sub-Agent 的工具和上下文需要隔离
3. **超时处理**: Sub-Agent 执行超时需要优雅处理
4. **结果一致性**: 并行执行时结果聚合的顺序问题
5. **上下文泄露**: Sub-Agent 之间不应有上下文共享（除非明确配置）
