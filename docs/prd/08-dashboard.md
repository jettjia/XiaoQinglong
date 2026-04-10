# Dashboard 实现计划

## 概述

Dashboard 组件当前使用静态 Mock 数据，需要改造为从后端 API 获取真实数据。

---

## 前端改造计划

### 1. 数据来源分析

| 页面区块 | 当前数据 | 数据来源 |
|---------|---------|---------|
| Active Agents | 静态 '12' | `agentApi.findAll()` 统计 enabled=true |
| Periodic Agents | 静态 '4' | `agentApi.findAll()` 统计 isPeriodic=true |
| Tasks Completed | 静态 '1,428' | **后端需新增接口** |
| Total Tokens | 静态 '2.4M' | **后端需新增接口** |
| Active Sources | 静态 '1' | `knowledgeBaseApi.findAll()` 统计 enabled=true |
| Pending Approvals | 静态 Mock | `chatApi.getPendingApprovals()` ✅ 已有 |
| Recent Conversations | 静态 Mock | `chatApi.getSessionsByUserId()` ✅ 已有 |
| Token Usage Ranking | 静态 Mock | **后端需新增接口** |
| Channel Activity | 静态 Mock | `channelApi.findAll()` ✅ 已有 |

### 2. 前端文件改动

| 文件 | 改动内容 |
|-----|---------|
| `src/lib/api.ts` | 新增 `dashboardApi` |
| `src/components/Dashboard.tsx` | 改造数据获取逻辑，添加 loading/error 状态 |

### 3. 前端新增 API

```typescript
// src/lib/api.ts 新增
export const dashboardApi = {
  // Dashboard 统计概览
  async getOverview(): Promise<{
    activeAgents: number;
    periodicAgents: number;
    tasksCompleted: number;
    totalTokens: number;
    activeKnowledgeSources: number;
  }> { ... }

  // Token 使用排行
  async getTokenUsageRanking(limit: number = 10): Promise<{
    agentId: string;
    agentName: string;
    totalTokens: number;
  }[]> { ... }

  // 渠道活动统计
  async getChannelActivity(): Promise<{
    channelId: string;
    channelName: string;
    status: 'active' | 'inactive';
    messageCount: number;
  }[]> { ... }
}
```

### 4. Dashboard 组件改造

```typescript
const [stats, setStats] = useState<Stats | null>(null);
const [loading, setLoading] = useState(true);
const [error, setError] = useState<string | null>(null);

useEffect(() => {
  // 并行获取所有数据
  Promise.all([
    dashboardApi.getOverview(),
    dashboardApi.getTokenUsageRanking(),
    dashboardApi.getChannelActivity(),
    chatApi.getPendingApprovals(),
    chatApi.getSessionsByUserId(currentUserId),
  ]).then(([overview, tokenRanking, channelActivity, approvals, sessions]) => {
    // 处理数据
  }).catch(err => setError(err.message));
}, []);
```

---

## 后端改造计划

### 1. 现有可复用接口

| 接口 | 路径 | 用途 |
|-----|------|-----|
| Agent 列表 | `POST /agent/all` | 统计 active/periodic agents |
| KnowledgeBase 列表 | `POST /knowledge_base/all` | 统计 active sources |
| Pending Approvals | `POST /chat/approval/pending` | 待审批列表 |
| Session 列表 | `POST /chat/session/byUserId` | 最近会话 |
| Channel 列表 | `POST /channel/all` | 渠道列表 |

### 2. 后端需新增接口

#### 2.1 Dashboard 概览统计

**请求**
```
GET /dashboard/overview
```

**响应**
```json
{
  "active_agents": 12,
  "periodic_agents": 4,
  "tasks_completed": 1428,
  "total_tokens": 2400000,
  "active_knowledge_sources": 1
}
```

**实现文件**

| 层级 | 文件 | 改动 |
|-----|------|-----|
| DTO | `application/dto/dashboard/dashboard_dto.go` | 新增 |
| Entity | `domain/entity/dashboard/dashboard_entity.go` | 新增 |
| Service | `application/service/dashboard/dashboard_svc.go` | 新增 |
| Handler | `api/http/handler/public/dashboard/dashboard_handler.go` | 新增 |
| Router | `api/http/router/public/sys_router.go` | 新增路由 |
| Repo | `infra/repository/repo/chat/chat_token_stats_impl.go` | 新增统计查询方法 |

#### 2.2 Token 使用排行

**请求**
```
GET /dashboard/token-ranking?limit=10
```

**响应**
```json
{
  "rankings": [
    {"agent_id": "agent_001", "agent_name": "Research-Agent", "total_tokens": 1200000},
    {"agent_id": "agent_002", "agent_name": "Customer-Support", "total_tokens": 850000}
  ]
}
```

**实现说明**
- 基于 `chat_token_stats` 表按 agent_id 分组 SUM(total_tokens)
- 需要关联 `sys_agent` 表获取 agent_name

#### 2.3 渠道活动统计

**请求**
```
GET /dashboard/channel-activity
```

**响应**
```json
{
  "channels": [
    {"channel_id": "ch_001", "channel_name": "Feishu", "status": "active", "message_count": 1240},
    {"channel_id": "ch_002", "channel_name": "DingTalk", "status": "active", "message_count": 850}
  ]
}
```

**实现说明**
- 基于 `chat_session` 表按 channel 分组 COUNT
- 需要关联 `sys_channel` 表获取 channel_name

### 3. 后端新增文件清单

```
backend/agent-frame/
├── application/
│   ├── dto/
│   │   └── dashboard/
│   │       └── dashboard_dto.go          # [新增]
│   └── service/
│       └── dashboard/
│           └── dashboard_svc.go          # [新增]
├── api/http/
│   └── handler/public/
│       └── dashboard/
│           └── dashboard_handler.go      # [新增]
└── infra/repository/
    └── repo/
        └── chat/
            └── chat_stats_impl.go       # [新增统计查询方法]
```

### 4. 数据库查询示例

```sql
-- Token 使用排行
SELECT
  a.agent_id,
  a.name as agent_name,
  SUM(s.total_tokens) as total_tokens
FROM chat_token_stats s
LEFT JOIN sys_agent a ON s.agent_id = a.ulid
GROUP BY s.agent_id, a.name
ORDER BY total_tokens DESC
LIMIT 10;

-- 渠道活动统计
SELECT
  c.channel_id,
  c.name as channel_name,
  CASE WHEN COUNT(m.ulid) > 0 THEN 'active' ELSE 'inactive' END as status,
  COUNT(m.ulid) as message_count
FROM sys_channel c
LEFT JOIN chat_session s ON c.code = s.channel
LEFT JOIN chat_message m ON s.ulid = m.session_id
GROUP BY c.channel_id, c.name;
```

---

## 实施顺序

### Phase 1: 已有数据改造 (前端为主)
1. 改造 `Dashboard.tsx` 使用现有 API 统计 Agent/KnowledgeBase 数量
2. 接入 `chatApi.getPendingApprovals()` 获取待审批列表
3. 接入 `chatApi.getSessionsByUserId()` 获取最近会话
4. 接入 `channelApi.findAll()` 显示渠道信息

### Phase 2: 后端统计接口 (后端为主)
1. 新增 `GET /dashboard/overview` 接口
2. 新增 `GET /dashboard/token-ranking` 接口
3. 新增 `GET /dashboard/channel-activity` 接口

### Phase 3: 前端对接后端
1. 对接 `dashboardApi.getOverview()`
2. 对接 `dashboardApi.getTokenUsageRanking()`
3. 对接 `dashboardApi.getChannelActivity()`

### Phase 4: 优化
1. 添加手动刷新功能
2. 添加自动刷新 (可选)
3. 添加 Error Boundary

---

## 注意事项

1. **Tasks Completed 统计**: 需定义"任务完成"的计数规则 (按 session 结束? 按 message 发送?)
2. **Token 统计**: 已有 `chat_token_stats` 表，可直接聚合查询
3. **性能考虑**: 统计类接口建议添加缓存 (Redis/Cache)
4. **时间范围**: Token ranking 等统计需明确时间范围 (当天/本周/本月/全部)
