# 知识库管理功能 (03-KnowledgeBaseManager.md)

## 1. 功能概述

完成知识库管理页面功能，支持对外部检索数据源进行 CRUD 操作，数据持久化到 PostgreSQL 数据库。Agent 可通过配置的检索 API 获取外部知识。

## 2. 功能清单

### 2.1 知识库列表展示
- 显示所有已配置的知识库
- 显示知识库关键信息：名称、描述、检索 URL、状态、最后更新时间
- 支持搜索知识库

### 2.2 知识库新增/编辑
- 点击"连接数据源"按钮打开弹窗
- 填写知识库配置：
  - 名称（必填）
  - 描述（可选）
  - 检索服务 URL（必填，外部检索 API 地址）
  - Token 认证（可选）
  - 是否启用
- 保存到数据库

### 2.3 知识库删除
- 在列表中点击删除按钮
- 确认后从数据库删除

### 2.4 召回测试
- 对单个知识库执行召回测试
- 输入查询语句，调用外部检索 API
- 显示召回结果（标题、内容、相关性分数）

### 2.5 API 规范说明
- 查看外部检索 API 的接口规范
- 显示输入/输出格式示例

## 3. 数据库设计

### 3.1 表结构：sys_knowledge_base

| 字段          | 类型         | 说明                   |
| ------------- | ------------ | ---------------------- |
| ulid          | VARCHAR(32)  | 主键                   |
| name          | VARCHAR(100) | 知识库名称             |
| description   | VARCHAR(500) | 描述                   |
| retrieval_url | VARCHAR(255) | 检索服务 URL           |
| token         | VARCHAR(500) | 认证 Token（加密存储） |
| enabled       | BOOLEAN      | 是否启用               |
| created_by    | VARCHAR(32)  | 创建人                 |
| updated_by    | VARCHAR(32)  | 更新人                 |
| created_at    | TIMESTAMP    | 创建时间               |
| updated_at    | TIMESTAMP    | 更新时间               |
| deleted_at    | TIMESTAMP    | 删除时间（软删除）     |

### 3.2 索引
- `idx_sys_kb_enabled` ON (enabled)
- `idx_sys_kb_deleted_at` ON (deleted_at)

### 3.3 外部检索 API 规范

系统调用外部知识检索 API，需符合以下规范：

**请求格式：**
```json
POST {retrieval_url}
Authorization: Bearer {token}
Content-Type: application/json

{
  "query": "搜索查询字符串",
  "top_k": 5
}
```

**响应格式：**
```json
[
  {
    "title": "文档标题",
    "content": "文档内容片段...",
    "score": 0.95
  },
  ...
]
```

## 4. API 设计

### 4.1 路由前缀
`/api/xiaoqinglong/agent-frame/v1/knowledge_base`

### 4.2 接口列表

| 方法   | 路径                         | 说明           |
| ------ | ---------------------------- | -------------- |
| GET    | /knowledge_base              | 获取所有知识库 |
| GET    | /knowledge_base/:ulid        | 获取单个知识库 |
| POST   | /knowledge_base              | 创建知识库     |
| PUT    | /knowledge_base/:ulid        | 更新知识库     |
| DELETE | /knowledge_base/:ulid        | 删除知识库     |
| POST   | /knowledge_base/:ulid/recall | 执行召回测试   |

### 4.3 请求/响应格式

**Create Request:**
```json
{
  "name": "External Search API",
  "description": "外部搜索 API 数据源",
  "retrievalUrl": "https://api.example.com/retrieve",
  "token": "Bearer xxx",
  "enabled": true
}
```

**Create Response:**
```json
{
  "ulid": "01ARZ3NDEKTSV4RRFFQ69G5FAV",
  "name": "External Search API",
  "description": "外部搜索 API 数据源",
  "retrievalUrl": "https://api.example.com/retrieve",
  "token": "***",
  "enabled": true,
  "created_at": 1711000000000,
  "updated_at": 1711000000000
}
```

**Recall Test Request:**
```json
{
  "query": "公司远程办公政策是什么？",
  "top_k": 5
}
```

**Recall Test Response:**
```json
[
  {
    "title": "远程办公政策",
    "content": "员工入职满3个月后可申请远程办公...",
    "score": 0.98
  }
]
```

## 5. 前端页面

- 前端页面：`KnowledgeBaseManager.tsx`
- 现有代码已实现 UI 部分，需对接后端 API
- 数据存储从前端内存改为从后端数据库读取
- 召回测试需调用实际检索 API

## 6. 实现要点

1. **Token 加密** - 使用 AES 或类似算法加密存储敏感 Token
2. **软删除** - deleted_at 标记，非真正删除
3. **召回测试** - 调用外部 API 时需处理超时和错误
4. **URL 格式校验** - 确保 retrieval_url 格式合法

## 7. 落点清单

### Backend (agent-frame)

1. **infra/repository/po/knowledge_base/**
   - `sys_knowledge_base_po.go` - 知识库 PO

2. **infra/repository/converter/knowledge_base/**
   - `sys_knowledge_base_conv.go` - DTO/Entity 转换

3. **domain/entity/knowledge_base/**
   - `sys_knowledge_base_entity.go` - 知识库领域实体

4. **domain/irepository/knowledge_base/**
   - `i_sys_knowledge_base_repo.go` - 知识库仓库接口

5. **infra/repository/repo/knowledge_base/**
   - `sys_knowledge_base_impl.go` - 知识库仓库实现

6. **domain/srv/knowledge_base/**
   - `sys_knowledge_base_svc.go` - 知识库领域服务

7. **application/dto/knowledge_base/**
   - `sys_knowledge_base_dto.go` - DTO 定义

8. **application/assembler/knowledge_base/**
   - `sys_knowledge_base_dto.go` - Assembler 转换

9. **application/service/knowledge_base/**
   - `sys_knowledge_base_svc.go` - 应用服务

10. **api/http/router/**
    - `sys_router.go` - 注册路由

11. **api/http/handler/public/knowledge_base/**
    - `handler.go` - Handler 结构体
    - `sys_knowledge_base_handler.go` - HTTP 处理函数

### Frontend (agent-ui)

1. **src/lib/api.ts**
   - 添加 knowledgeBaseApi 调用方法
   - 添加 recallTest 方法

2. **src/components/KnowledgeBaseManager.tsx**
   - 从后端加载知识库数据
   - CRUD 操作对接后端 API
   - 召回测试调用实际 API

## 8. Mock 外部检索服务

提供一个简易的 Mock 检索服务，便于测试和演示召回功能。

### 8.1 服务部署

```bash
cd /home/jett/aishu/XiaoQinglong/mock/kb-service
go run main.go
```

服务监听 `http://localhost:8081`

### 8.2 接口规范

**POST /retrieve**

请求：
```json
{
  "query": "搜索关键词",
  "top_k": 5
}
```

响应：
```json
[
  {
    "title": "文档标题1",
    "content": "文档内容片段1...",
    "score": 0.95
  },
  {
    "title": "文档标题2",
    "content": "文档内容片段2...",
    "score": 0.88
  }
]
```

### 8.3 内置测试数据

服务内置以下测试数据：

| 关键词   | 召回内容                                            |
| -------- | --------------------------------------------------- |
| 远程办公 | 员工入职满3个月后可申请远程办公，需部门经理批准     |
| 密码重置 | 访问自助门户 https://reset.company.com 重置密码     |
| 请假政策 | 员工每年享有15天带薪年假，需提前3天申请             |
| 报销流程 | 通过费控系统提交报销申请，平均处理时间3个工作日     |
| IT支持   | 联系 IT 支持邮箱 it-support@company.com 或拨打 8001 |

### 8.4 配置说明

在 KnowledgeBaseManager 中添加知识库时：
- Name: `Mock KB Service`
- Retrieval URL: `http://localhost:8081/retrieve`
- Token: （空）
- Enabled: true

## 9. 参考

- agent-frame 规范文档：`backend/agent-frame/skills.md`
- DDD 分层架构示例：sys_model 模块
- 前端现有实现：`KnowledgeBaseManager.tsx`
