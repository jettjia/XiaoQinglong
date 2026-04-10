# Agent 编排功能 (05-AgentOrchestrator.md)

## 1. 功能概述

智能体（Agent）编排功能，支持对 Agent 进行管理。智能体由以下组件组成：
- **模型（Model）**：LLM 配置
- **技能（Skill）**：用户编写的技能脚本
- **工具（Tool/MCP/A2A）**：HTTP 工具、MCP 服务、A2A 代理
- **知识库（Knowledge）**：向量检索知识

最终入库存储格式为一大段 JSON，参考 `backend/runner/example/test-all.json` 格式。

## 2. 功能清单

### 2.1 Agent 列表展示
- 显示所有已创建的 Agent
- 显示 Agent 关键信息：名称、描述、模型、状态、类型（内置/用户创建）
- 支持搜索 Agent
- 按类型筛选（内置 / 用户创建）

### 2.2 Agent 创建
- 输入 Agent 基本信息（名称、描述、图标）
- 选择默认模型
- 选择绑定的 Skills / Tools / MCPs / A2As / KnowledgeBases
- 配置系统提示词
- 配置 Sandbox 环境
- 配置响应格式（response_schema）
- 配置重试策略、限流策略等

### 2.3 Agent 编辑
- 修改 Agent 基本信息
- 启用/禁用 Agent

### 2.4 Agent 删除
- 在列表中点击删除按钮
- 确认后从数据库删除
- **系统内置 Agent 不能删除**

### 2.5 Agent JSON 导入/导出
- **导出**：将 Agent 的 config JSON 导出，用户可拷贝到 `backend/runner/example/` 目录调试
- **导入**：用户可直接粘贴 JSON 数据入库，支持直接拷贝 `test-all.json` 格式调试

## 3. 数据库设计

### 3.1 表结构：sys_agent

| 字段        | 类型         | 说明                                 |
| ----------- | ------------ | ------------------------------------ |
| ulid        | VARCHAR(32)  | 主键                                 |
| name        | VARCHAR(100) | Agent 名称                           |
| description | TEXT         | 描述                                 |
| icon        | VARCHAR(50)  | 图标名称                             |
| model       | VARCHAR(100) | 默认模型                             |
| config      | TEXT         | 完整 JSON 配置（参考 test-all.json） |
| is_system   | BOOLEAN      | 是否系统内置                         |
| enabled     | BOOLEAN      | 是否启用                             |
| created_by  | VARCHAR(32)  | 创建人                               |
| updated_by  | VARCHAR(32)  | 更新人                               |
| created_at  | TIMESTAMP    | 创建时间                             |
| updated_at  | TIMESTAMP    | 更新时间                             |
| deleted_at  | TIMESTAMP    | 删除时间（软删除）                   |

### 3.2 索引
- `idx_sys_agent_name` ON (name)
- `idx_sys_agent_enabled` ON (enabled)
- `idx_sys_agent_is_system` ON (is_system)
- `idx_sys_agent_deleted_at` ON (deleted_at)

## 4. config JSON 格式（参考 test-all.json）

```json
{
  "endpoint": "http://localhost:18080/run",
  "models": {
    "default": { "provider": "openai", "name": "${OPENAI_MODEL}", "api_key": "${OPENAI_API_KEY}", "api_base": "${OPENAI_BASE_URL}" },
    "rewrite": { "provider": "openai", "name": "${OPENAI_MODEL_MINI}", "api_key": "${OPENAI_API_KEY}", "api_base": "${OPENAI_BASE_URL}" },
    "skill": { "provider": "openai", "name": "${OPENAI_MODEL}", "api_key": "${OPENAI_API_KEY}", "api_base": "${OPENAI_BASE_URL}" },
    "summarize": { "provider": "openai", "name": "${OPENAI_MODEL_MINI}", "api_key": "${OPENAI_API_KEY}", "api_base": "${OPENAI_BASE_URL}" }
  },
  "system_prompt": "你是一个智能助手...",
  "user_message": "",
  "tools": [
    { "type": "http", "name": "get_order_detail", "description": "...", "endpoint": "http://localhost:28081/v1/orders/{order_no}", "method": "GET", "headers": {}, "risk_level": "high" }
  ],
  "a2a": [
    { "name": "payment_agent", "endpoint": "http://localhost:28080/a2a", "headers": {}, "risk_level": "medium" }
  ],
  "mcps": [
    { "name": "weather", "transport": "stdio", "command": "go", "args": ["run", "/path/to/mcp"], "env": {}, "risk_level": "low" }
  ],
  "sandbox": {
    "enabled": true,
    "mode": "docker",
    "image": "sandbox-code-interpreter:v1.0.3",
    "workdir": "/workspace",
    "timeout_ms": 120000,
    "network": "bridge",
    "env": { "PATH": "/usr/local/bin:/usr/bin:/bin" },
    "limits": { "cpu": "0.5", "memory": "512m" }
  },
  "options": {
    "temperature": 0.7,
    "max_tokens": 2000,
    "max_iterations": 10,
    "max_tool_calls": 20,
    "max_a2a_calls": 5,
    "max_total_tokens": 50000,
    "stream": true,
    "retry": { "max_attempts": 3, "initial_delay_ms": 1000, "max_delay_ms": 10000, "backoff_multiplier": 2.0 },
    "routing": { "default_model": "default", "rewrite_prompt": "请优化以下用户Query...", "summarize_prompt": "请总结以下内容..." },
    "approval_policy": { "enabled": true, "risk_threshold": "medium", "auto_approve": ["get_product_info"] }
  },
  "skills": [
    { "id": "s3-upload", "name": "S3上传下载", "description": "...", "instruction": "你是一个S3操作助手...", "scope": "both", "trigger": "manual", "entry_script": "python3 scripts/s3_upload.py", "file_path": "s3-upload", "risk_level": "medium" }
  ],
  "context": {
    "session_id": "",
    "user_id": "",
    "channel_id": "feishu",
    "skills_dir": "skills",
    "variables": {},
    "trace_id": "",
    "parent_span_id": ""
  },
  "knowledge": [
    { "id": "product_info", "name": "产品信息", "content": "...", "score": 0.95, "metadata": { "source": "internal_wiki" } }
  ]
}
```

## 5. API 设计

### 5.1 路由前缀
`/api/xiaoqinglong/agent-frame/v1/agent`

### 5.2 接口列表

| 方法   | 路径                | 说明                    |
| ------ | ------------------- | ----------------------- |
| GET    | /agent              | 获取所有 Agent          |
| GET    | /agent/:ulid        | 获取单个 Agent          |
| POST   | /agent              | 创建 Agent              |
| PUT    | /agent/:ulid        | 更新 Agent              |
| DELETE | /agent/:ulid        | 删除 Agent              |
| POST   | /agent/upload       | 导入 Agent（JSON 格式） |
| GET    | /agent/export/:ulid | 导出 Agent（JSON 格式） |

### 5.3 请求/响应格式

**Create Request:**
```json
{
  "name": "订单助手",
  "description": "帮助用户查询和处理订单",
  "icon": "Bot",
  "model": "openai-gpt-4",
  "config": { ... },
  "enabled": true,
  "is_system": false
}
```

**Upload Request (multipart/form-data):**
```
file: JSON 文件 或 直接粘贴 JSON 到 body
```

**Upload Response:**
```json
{
  "ulid": "01ARZ3NDEKTSV4RRFFQ69G5FAV",
  "name": "订单助手",
  "description": "帮助用户查询和处理订单",
  "icon": "Bot",
  "model": "openai-gpt-4",
  "config": { ... },
  "enabled": true,
  "is_system": false,
  "created_at": 1711000000000
}
```

## 6. 前端页面

- 页面文件：`AgentManager.tsx` 为列表页面，`AgentOrchestrator.tsx`为编排页面
- JSON 导入：提供 textarea 或文件上传两种方式
- JSON 导出：提供下载或一键拷贝功能
- 遵循当前的 AgentOrchestrator.tsx的设计规则，不能乱改

## 7. 实现要点

1. **config TEXT 字段** - 存储完整 JSON，与 test-all.json 格式完全兼容
2. **软删除** - deleted_at 标记，非真正删除
3. **系统内置保护** - is_system=true 的 Agent 不能删除
4. **JSON 导入/导出** - 用户可直接粘贴 test-all.json 格式的 JSON 调试
5. **与 SkillManager 联动** - 创建 Agent 时可选择已注册的 Skills/Tools/MCPs/A2As

## 8. 落点清单

### Backend (agent-frame)

1. **infra/repository/po/agent/**
   - `sys_agent_po.go` - Agent PO

2. **infra/repository/converter/agent/**
   - `sys_agent_conv.go` - DTO/Entity/PO 转换

3. **domain/entity/agent/**
   - `sys_agent_entity.go` - Agent 领域实体

4. **domain/irepository/agent/**
   - `i_sys_agent_repo.go` - Agent 仓库接口

5. **infra/repository/repo/agent/**
   - `sys_agent_impl.go` - Agent 仓库实现

6. **domain/srv/agent/**
   - `sys_agent_svc.go` - Agent 领域服务

7. **application/dto/agent/**
   - `sys_agent_dto.go` - DTO 定义

8. **application/assembler/agent/**
   - `sys_agent_assembler.go` - Assembler 转换

9. **application/service/agent/**
   - `sys_agent_svc.go` - 应用服务

10. **api/http/handler/public/agent/**
    - `agent_handler.go` - Handler 结构体
    - `sys_agent_handler.go` - HTTP 处理函数

11. **api/http/router/**
    - `agent_router.go` - 注册路由

12. **boot/migrate.go**
    - 添加 `sys_agent` 表的 AutoMigrate

### Frontend (agent-ui)

1. **src/lib/api.ts**
   - 添加 `agentApi`: create, update, delete, findAll, findById, upload, export

2. **src/types.ts**
   - 扩展 `Agent` 接口，添加 `config` 字段

3. **src/components/AgentManager.tsx**
   - Agent 列表展示
   - 创建/编辑/删除功能
   - JSON 导入/导出功能
   - 搜索/筛选功能

## 9. 参考

- SkillManager 实现参考：`04-SkillManager.md`
- AgentOrchestrator 组件：`frontend/agent-ui/src/components/AgentOrchestrator.tsx`
- JSON 格式参考：`backend/runner/example/test-all.json`
- DDD 分层架构示例：sys_skill / sys_model 模块
