# 模型管理功能 (02-model.md)

## 1. 功能概述

完成模型管理页面功能，支持对 LLM 模型和 Embedding 模型进行 CRUD 操作，数据持久化到 PostgreSQL 数据库。

## 2. 功能清单

### 2.1 模型列表展示
- 显示所有已配置模型（LLM / Embedding）
- 支持 Tab 切换：LLM 模型 / Embedding 模型
- 显示模型关键信息：名称、Provider、状态、延迟、上下文窗口、使用率
- 支持点击卡片进入编辑模式

### 2.2 模型新增/编辑
- 点击"添加模型"按钮打开弹窗
- 填写模型配置：
  - 模型类型（LLM / Embedding）
  - 模型名称
  - Provider（OpenAI / Anthropic / Google / Custom）
  - Base URL（API 端点）
  - API Key（敏感信息，需加密存储）
  - 分类（LLM 专用：default / rewrite / skill / summarize）
- 保存到数据库

### 2.3 模型删除
- 在编辑弹窗中可删除模型
- 确认后从数据库删除

### 2.4 模型状态
- `active` - 正在使用
- `configured` - 已配置但未启用
- `error` - 连接错误

## 3. 数据库设计

### 3.1 表结构：sys_model

| 字段 | 类型 | 说明 |
|------|------|------|
| ulid | VARCHAR(32) | 主键 |
| name | VARCHAR(100) | 模型名称 |
| provider | VARCHAR(50) | 提供商 |
| base_url | VARCHAR(255) | API 地址 |
| api_key | VARCHAR(500) | API Key（加密存储） |
| model_type | VARCHAR(20) | llm / embedding |
| category | VARCHAR(20) | default / rewrite / skill / summarize（仅 LLM） |
| status | VARCHAR(20) | active / configured / error |
| latency | VARCHAR(20) | 平均延迟 |
| context_window | VARCHAR(20) | 上下文窗口大小 |
| usage | INT | 使用次数/百分比 |
| created_by | VARCHAR(32) | 创建人 |
| updated_by | VARCHAR(32) | 更新人 |
| created_at | TIMESTAMP | 创建时间 |
| updated_at | TIMESTAMP | 更新时间 |
| deleted_at | TIMESTAMP | 删除时间（软删除） |

### 3.2 索引
- `idx_sys_model_type` ON (model_type)
- `idx_sys_model_status` ON (status)
- `idx_sys_model_deleted_at` ON (deleted_at)

## 4. API 设计

### 4.1 路由前缀
`/api/xiaoqinglong/agent-frame/v1/model`

### 4.2 接口列表

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /model | 获取所有模型 |
| GET | /model/:ulid | 获取单个模型 |
| POST | /model | 创建模型 |
| PUT | /model/:ulid | 更新模型 |
| DELETE | /model/:ulid | 删除模型 |

### 4.3 请求/响应格式

**Create/Update Request:**
```json
{
  "name": "Gemini 3.1 Pro",
  "provider": "Google",
  "baseUrl": "https://generativelanguage.googleapis.com/v1",
  "apiKey": "xxx",
  "modelType": "llm",
  "category": "default"
}
```

**Response:**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "ulid": "01ARZ3NDEKTSV4RRFFQ69G5FAV",
    "name": "Gemini 3.1 Pro",
    "provider": "Google",
    "baseUrl": "https://generativelanguage.googleapis.com/v1",
    "apiKey": "***",
    "modelType": "llm",
    "category": "default",
    "status": "configured",
    "latency": null,
    "contextWindow": "128k",
    "usage": 0
  }
}
```

## 5. 前端页面

- 前端页面：`ModelManager.tsx`
- 现有代码已实现 UI 部分，需对接后端 API
- 数据存储从前端内存改为从后端数据库读取

## 6. 实现要点

1. **API Key 加密** - 使用 AES 或类似算法加密存储
2. **软删除** - deleted_at 标记，非真正删除
3. **分类仅 LLM** - embedding 模型不需要 category 字段
4. **状态联动** - 删除关联 Agent 时可自动标记模型为 configured

## 7. 落点清单

### Backend (agent-frame)

1. **infra/repository/po/model/**
   - `sys_model_po.go` - 模型 PO
   - `converter.go` - DTO/Entity 转换

2. **domain/entity/**
   - `sys_model_entity.go` - 模型领域实体

3. **domain/srv/**
   - `sys_model_srv.go` - 模型领域服务

4. **application/dto/model/**
   - `sys_model_dto.go` - DTO 定义

5. **application/service/model/**
   - `sys_model_svc.go` - 应用服务

6. **api/http/router/**
   - `sys_router.go` - 注册路由

7. **api/http/handler/public/model/**
   - `handler.go` - Handler 结构体
   - `sys_model_handler.go` - HTTP 处理函数

### Frontend (agent-ui)

1. **src/lib/api.ts**
   - 添加 modelApi 调用方法

2. **src/components/ModelManager.tsx**
   - 从后端加载模型数据
   - CRUD 操作对接后端 API

## 8. 参考

- agent-frame 规范文档：`backend/agent-frame/skills.md`
- DDD 分层架构示例：sys_user 模块
