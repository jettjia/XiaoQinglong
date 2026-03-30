# 聊天界面设计

> **前置参考**：[05-AgentOrchestrator.md](./05-AgentOrchestrator.md) - 智能体编排设计，定义了 Agent 的配置结构、工具调用和审批策略

## 1. 概述

聊天界面为用户提供与 AI 智能体对话的交互界面。用户可以选择不同的智能体进行对话，智能体根据用户输入调用工具/技能并返回结果。

## 2. 架构设计

```
┌─────────────┐     ┌──────────────┐     ┌───────────┐
│   前端       │────▶│  Agent-Frame  │────▶│   Runner   │
│  (Agent-UI) │◀────│  (Agent-API)  │◀────│  (Agent)   │
└─────────────┘     └──────────────┘     └───────────┘
                           │
                           ▼
                    ┌──────────────┐
                    │   数据库      │
                    │ (聊天历史)    │
                    └──────────────┘
```

**核心要点：**
- 前端只与 `agent-frame` 对接
- `agent-frame` 负责与 `runner` 通信
- 所有聊天内容存入数据库，支持用户查看历史

## 3. 数据模型

### 3.1 聊天会话 (Chat Session)

| 字段 | 类型 | 描述 |
|-------|------|------|
| ulid | string | 主键 |
| created_at | int64 | 创建时间戳 |
| updated_at | int64 | 更新时间戳 |
| deleted_at | int64 | 删除时间戳 |
| user_id | string | 用户 ID |
| agent_id | string | 智能体 ULID（关联 sys_agent） |
| title | string | 会话标题（首条消息摘要） |
| status | string | active/archived |
| channel | string | 渠道编码 (web/api/feishu/dingtalk) |

### 3.2 聊天消息 (Chat Message)

| 字段 | 类型 | 描述 |
|-------|------|------|
| ulid | string | 主键 |
| session_id | string | 会话 ULID |
| created_at | int64 | 创建时间戳 |
| role | string | user/assistant/system |
| content | text | 消息内容 |
| model | string | 使用的模型 |
| input_tokens | int | 输入 Token 数量 |
| output_tokens | int | 输出 Token 数量 |
| total_tokens | int | 总 Token 数量 |
| latency_ms | int | 响应延迟 |
| trace | json | 执行轨迹（工具调用、推理等） |
| status | string | sending/success/failed/pending_approval |
| error_msg | string | 错误信息 |
| metadata | json | 附加元数据（智能体配置等） |

### 3.4 Token 统计 (Chat Token Stats)

| 字段 | 类型 | 描述 |
|-------|------|------|
| ulid | string | 主键 |
| session_id | string | 会话 ULID |
| agent_id | string | 智能体 ULID |
| user_id | string | 用户 ID |
| date | string | 统计日期 (YYYY-MM-DD) |
| model | string | 模型标识 |
| input_tokens | int | 当日输入 Token 累计 |
| output_tokens | int | 当日输出 Token 累计 |
| total_tokens | int | 当日总 Token 累计 |
| request_count | int | 当日请求次数 |
| cost_amount | decimal | 预估费用 |
| created_at | int64 | 创建时间 |
| updated_at | int64 | 更新时间 |

### 3.3 执行轨迹 (Chat Trace)

| 字段 | 类型 | 描述 |
|-------|------|------|
| id | string | 步骤 ID |
| type | string | thought/tool/retrieval/approval |
| label | string | 显示标签 |
| content | text | 轨迹内容 |
| status | string | pending/success/failed |
| timestamp | int64 | 步骤时间戳 |
| duration_ms | int | 步骤耗时 |

## 4. 响应格式类型

编排智能体时，可配置以下响应格式类型：

| 格式类型 | 描述 | Content-Type |
|-------------|-------------|--------------|
| text | 纯文本 | text/plain |
| markdown | Markdown 格式 | text/markdown |
| a2ui | A2UI 协议 | application/json |
| audio | 音频响应 | audio/* |
| image | 图片响应 | image/* |
| video | 视频响应 | video/* |
| mixed | 混合内容 | multipart/mixed |

### 4.1 A2UI 响应格式

```json
{
  "type": "a2ui",
  "version": "1.0",
  "content": {
    "text": "响应内容",
    "audio_url": "可选音频地址",
    "images": ["可选图片地址"],
    "actions": [
      {
        "type": "tool_call",
        "tool": "tool_name",
        "params": {}
      }
    ]
  },
  "trace": {
    "steps": [
      {"type": "thought", "content": "推理过程..."},
      {"type": "tool", "content": "调用工具..."}
    ]
  }
}
```

## 5. UI 布局

```
┌─────────────────────────────────────────────────────────────┐
│  侧边栏 (240px)           │  聊天区域                        │
│  ┌─────────────────────┐  │  ┌─────────────────────────────┐│
│  │ 会话列表             │  │  │ 智能体头部                   ││
│  │ - 今天               │  │  │ 智能体名称 | 状态            ││
│  │   - 会话 1           │  │  └─────────────────────────────┘│
│  │   - 会话 2           │  │  ┌─────────────────────────────┐│
│  │ - 昨天               │  │  │                             ││
│  │   - 会话 3           │  │  │  消息列表                    ││
│  │                      │  │  │  (可滚动)                    ││
│  │                      │  │  │                             ││
│  │                      │  │  │                             ││
│  │                      │  │  └─────────────────────────────┘│
│  │                      │  │  ┌─────────────────────────────┐│
│  │                      │  │  │ 审批卡片 (待审批时显示)       ││
│  │                      │  │  │ 工具: xxx | 风险: 高        ││
│  └─────────────────────┘  │  │ [批准] [拒绝]                ││
│  ┌─────────────────────┐  │  └─────────────────────────────┘│
│  │ 智能体选择器          │  │  ┌─────────────────────────────┐│
│  │ ┌─────────────────┐  │  │ 输入区域                     ││
│  │ │ [Bot] 智能体1 ▼ │  │  │ [智能体下拉]                 ││
│  │ └─────────────────┘  │  │ [文本输入框]            [➤] ││
│  │ ○ 系统智能体 1      │  │  └─────────────────────────────┘│
│  │ ○ 系统智能体 2      │  │                                │
│  │ ─────────────────── │  │                                │
│  │ ○ 用户智能体 1        │  │                                │
│  │ ○ 用户智能体 2        │  │                                │
│  └─────────────────────┘  │                                │
└─────────────────────────────────────────────────────────────┘
```

## 6. 智能体选择器

### 6.1 智能体列表规则

1. 系统默认智能体显示在前
2. 用户自定义智能体显示在后
3. 根据 `enabled = true` 和匹配的 `channel` 过滤
4. 显示格式：`智能体名称 (模型提供商)`

### 6.2 切换智能体

- 切换智能体将开始新的聊天会话
- 之前的会话自动归档
- 新会话继承智能体配置

## 7. 消息交互

### 7.1 用户消息

- 通过文本框输入（Enter 发送，Shift+Enter 换行）
- 支持文件附件（拖拽或点击上传）
- 显示字符计数

### 7.2 助手消息

- 支持流式响应
- 显示执行轨迹：
  - 思考步骤（推理过程）
  - 工具调用（带参数）
  - 检索结果（从知识库）
- 显示 Token 使用量和延迟

### 7.3 轨迹展示

```
┌────────────────────────────────────────┐
│ 💭 推理                               │
│ 分析用户问题：如何获取股票信息...        │
│                                        │
│ 🔧 工具: get_stock_price             │
│ 调用参数: {symbol: "AAPL"}            │
│ 结果: $150.25                        │
│                                        │
│ 📚 知识检索                           │
│ 从知识库中找到 3 个相关片段            │
└────────────────────────────────────────┘
```

## 8. 审批流程（人工介入）

### 8.1 流程图

```
用户消息
    │
    ▼
智能体处理
    │
    ├─── 调用工具（高风险）
    │         │
    │         ▼
    │    ┌────────────────┐
    │    │ 待审批请求      │◀── 收件箱通知
    │    │ (存入数据库)    │
    │    └────────────────┘
    │              │
    │    ┌────────┴────────┐
    │    ▼                 ▼
    │ 批准              拒绝
    │    │                 │
    │    ▼                 ▼
    │ 执行工具          返回错误
    │    │                 │
    └────┴─────────────────┘
              │
              ▼
         最终响应
```

### 8.2 收件箱集成

当智能体需要审批时：

1. **收件箱通知徽章**
   - 红色徽章显示待审批数量
   - 通过轮询或 WebSocket 实时更新

2. **收件箱页面 (`/inbox`)**
   - 列出所有待审批请求
   - 按智能体和时间分组
   - 按状态筛选：待审批/已批准/已拒绝

3. **审批卡片**

```
┌─────────────────────────────────────────────┐
│ ⚠️ 待审批请求                               │
│                                             │
│ 智能体: order_agent                          │
│ 工具: payment.execute                       │
│ 风险等级: HIGH                              │
│                                             │
│ 参数:                                        │
│ {                                            │
│   "order_no": "12345",                     │
│   "amount": 999.00                         │
│ }                                            │
│                                             │
│ 请求时间: 2024-01-15 10:30:00              │
│                                             │
│ [✅ 批准]  [❌ 拒绝]                       │
└─────────────────────────────────────────────┘
```

4. **聊天集成**
   - 待审批内容在聊天中显示为特殊消息卡片
   - 用户可直接在聊天中批准/拒绝
   - 批准后继续执行
   - 拒绝后返回错误给智能体

## 9. 聊天历史

### 9.1 会话管理

- 按日期分组：今天、昨天、本周、本月、更早
- 支持按标题或内容搜索会话
- 删除/归档会话
- 导出会话为 JSON/Markdown

### 9.2 消息持久化

对话结束后：
1. 所有消息连同完整轨迹入库
2. Token 使用统计
3. 智能体配置快照
4. 渠道信息

## 10. API 接口

### 10.1 聊天会话

| 方法 | 端点 | 描述 |
|--------|----------|-------------|
| POST | /chat/session | 创建新会话 |
| GET | /chat/session/{ulid} | 获取会话详情 |
| PUT | /chat/session/{ulid} | 更新会话 |
| DELETE | /chat/session/{ulid} | 删除会话 |
| GET | /chat/sessions | 列出用户会话 |
| PUT | /chat/session/{ulid}/archive | 归档会话 |

### 10.2 聊天消息

| 方法 | 端点 | 描述 |
|--------|----------|-------------|
| POST | /chat/message | 发送消息 |
| GET | /chat/message/{ulid} | 获取消息详情 |
| GET | /chat/session/{ulid}/messages | 列出会话消息 |

### 10.3 审批

| 方法 | 端点 | 描述 |
|--------|----------|-------------|
| POST | /chat/approval | 创建审批请求 |
| PUT | /chat/approval/{ulid} | 更新审批状态 |
| GET | /chat/approvals | 列出待审批请求 |

## 11. 组件列表

| 组件 | 描述 |
|-----------|-------------|
| ChatSidebar | 左侧边栏（会话列表 + 智能体选择器） |
| SessionList | 按日期分组的会话列表 |
| SessionItem | 单个会话项 |
| AgentSelector | 智能体下拉选择器和列表 |
| ChatHeader | 智能体信息头部显示 |
| MessageList | 可滚动的消息容器 |
| MessageBubble | 单条消息展示 |
| UserMessage | 用户消息气泡 |
| AssistantMessage | 助手消息（带轨迹） |
| TraceCard | 执行轨迹展示 |
| ApprovalCard | 待审批消息展示 |
| ChatInput | 文本输入和发送按钮 |
| InboxNotification | 徽章通知组件 |

## 12. 状态管理

```typescript
interface ChatState {
  // 当前会话
  currentSession: Session | null;
  messages: Message[];

  // 智能体选择器
  agents: Agent[];
  selectedAgent: Agent | null;

  // 输入
  inputValue: string;
  isLoading: boolean;

  // 审批
  pendingApprovals: Approval[];
}
```

## 13. 事件流程

### 13.1 发送消息

1. 用户输入消息并点击发送
2. 前端 POST 到 `/chat/message`
3. 后端创建消息，状态为 `sending`
4. 后端转发给 Runner
5. Runner 流式返回响应
6. 前端展示流式内容
7. Runner 完成，后端更新消息状态为 `success`
8. 后端存储完整消息和轨迹

### 13.2 工具审批

1. 智能体调用高风险工具
2. Runner 暂停并返回 `pending_approval`
3. 后端将审批记录存入数据库
4. 后端返回 approval_id 给前端
5. 前端展示审批卡片
6. 后端同时发送通知到收件箱
7. 用户通过聊天或收件箱批准
8. 后端更新审批状态
9. 后端通知 Runner 继续执行
10. Runner 继续执行

## 14. 数据库 Schema

```sql
-- 聊天会话表
CREATE TABLE chat_session (
    ulid VARCHAR(128) PRIMARY KEY,
    user_id VARCHAR(128),
    agent_id VARCHAR(128),
    title VARCHAR(256),
    status VARCHAR(32), -- active, archived
    channel VARCHAR(32),
    created_at BIGINT,
    updated_at BIGINT,
    deleted_at BIGINT
);

-- 聊天消息表
CREATE TABLE chat_message (
    ulid VARCHAR(128) PRIMARY KEY,
    session_id VARCHAR(128),
    role VARCHAR(32), -- user, assistant, system
    content TEXT,
    model VARCHAR(128),
    input_tokens INT DEFAULT 0,
    output_tokens INT DEFAULT 0,
    total_tokens INT DEFAULT 0,
    latency_ms INT,
    trace JSON,
    status VARCHAR(32), -- sending, success, failed, pending_approval
    error_msg TEXT,
    metadata JSON,
    created_at BIGINT
);

-- 聊天审批表
CREATE TABLE chat_approval (
    ulid VARCHAR(128) PRIMARY KEY,
    message_id VARCHAR(128),
    agent_id VARCHAR(128),
    tool_name VARCHAR(128),
    risk_level VARCHAR(32),
    parameters JSON,
    status VARCHAR(32), -- pending, approved, rejected
    approved_by VARCHAR(128),
    approved_at BIGINT,
    created_at BIGINT
);

-- Token 消耗统计表（按日期、模型聚合）
CREATE TABLE chat_token_stats (
    ulid VARCHAR(128) PRIMARY KEY,
    session_id VARCHAR(128),
    agent_id VARCHAR(128),
    user_id VARCHAR(128),
    date VARCHAR(16), -- YYYY-MM-DD
    model VARCHAR(128),
    input_tokens INT DEFAULT 0,
    output_tokens INT DEFAULT 0,
    total_tokens INT DEFAULT 0,
    request_count INT DEFAULT 0,
    cost_amount DECIMAL(10, 4) DEFAULT 0,
    created_at BIGINT,
    updated_at BIGINT,
    UNIQUE KEY uk_stats (agent_id, user_id, date, model)
);
```

## 15. 实现要点

1. **流式响应**：使用 Server-Sent Events (SSE) 实现
2. **实时性**：每 5 秒轮询待审批请求
3. **离线支持**：离线时队列缓存消息，重连后同步
4. **性能优化**：消息列表分页，长对话使用虚拟滚动
5. **安全**：验证智能体权限，清理用户输入
