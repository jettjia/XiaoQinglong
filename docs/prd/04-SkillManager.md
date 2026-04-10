# 技能管理功能 (04-SkillManager.md)

## 1. 功能概述

完成技能管理页面功能，支持对 Agent Skill 进行管理。用户上传 Skill 包（ZIP 格式）后，系统解压缩到 `skills/` 目录，Skill 的元信息（mcp、tool、a2a）存储到 PostgreSQL 数据库。

## 2. 功能清单

### 2.1 Skill 列表展示
- 显示所有已安装的 Skill
- 显示 Skill 关键信息：名称、描述、类型（MCP/Tool/A2A）、状态、版本、最后更新时间
- 支持搜索 Skill
- 按类型筛选（MCP / Tool / A2A）

### 2.2 Skill 上传/安装
- 点击"上传 Skill"按钮
- 选择 ZIP 包上传
- 系统解压缩到 `skills/{skill_name}/` 目录
- 解析 SKILL.md 提取元信息
- **同名检测**：如果 `skills/{skill_name}/` 已存在，提示用户是否覆盖
- 元信息入库（mcp、tool、a2a 配置）
- 支持启用/禁用 Skill

### 2.3 Skill 编辑
- 修改 Skill 基本信息（名称、描述）
- 启用/禁用 Skill

### 2.4 Skill 删除
- 在列表中点击删除按钮
- 确认后从数据库删除元信息
- **不删除** `skills/` 目录下的文件（保留原始文件）

### 2.5 Skill 包格式

Skill 包为 ZIP 文件，解压后结构如下：

```
{skill_name}/
├── SKILL.md          # 必须，元信息定义
├── scripts/          # 可选，执行脚本
├── assets/           # 可选，资源文件
└── ...
```

**SKILL.md 格式：**
```yaml
---
name: skill_name
description: Skill 描述
type: tool | mcp | a2a  # Skill 类型
version: 1.0.0          # 版本
---
# Skill 名称

## 描述
这里是详细描述...

## 使用方式
...
```

## 3. 数据库设计

### 3.1 表结构：sys_skill

| 字段          | 类型         | 说明                   |
| ------------- | ------------ | ---------------------- |
| ulid          | VARCHAR(32)  | 主键                   |
| name          | VARCHAR(100) | Skill 名称             |
| description   | VARCHAR(500) | 描述                   |
| skill_type    | VARCHAR(20)  | 类型：mcp/tool/a2a    |
| version       | VARCHAR(20)  | 版本号                 |
| path          | VARCHAR(255) | 存储路径（skills/{name}）|
| enabled       | BOOLEAN      | 是否启用               |
| config        | TEXT         | 扩展配置（JSON）       |
| created_by    | VARCHAR(32)  | 创建人                 |
| updated_by    | VARCHAR(32)  | 更新人                 |
| created_at    | TIMESTAMP    | 创建时间               |
| updated_at    | TIMESTAMP    | 更新时间               |
| deleted_at    | TIMESTAMP    | 删除时间（软删除）     |

### 3.2 索引
- `idx_sys_skill_name` ON (name)
- `idx_sys_skill_type` ON (skill_type)
- `idx_sys_skill_enabled` ON (enabled)
- `idx_sys_skill_deleted_at` ON (deleted_at)

## 4. API 设计

### 4.1 路由前缀
`/api/xiaoqinglong/agent-frame/v1/skill`

### 4.2 接口列表

| 方法   | 路径                | 说明           |
| ------ | ------------------- | -------------- |
| GET    | /skill              | 获取所有 Skill |
| GET    | /skill/:ulid        | 获取单个 Skill |
| POST   | /skill              | 创建 Skill     |
| PUT    | /skill/:ulid        | 更新 Skill     |
| DELETE | /skill/:ulid        | 删除 Skill     |
| POST   | /skill/upload       | 上传并安装 Skill|
| GET    | /skill/check-name   | 检查同名 Skill |

### 4.3 请求/响应格式

**Upload Request (multipart/form-data):**
```
file: ZIP 文件
```

**Upload Response:**
```json
{
  "ulid": "01ARZ3NDEKTSV4RRFFQ69G5FAV",
  "name": "pptx",
  "description": "处理 PPTX 文件的 Skill",
  "skillType": "tool",
  "version": "1.0.0",
  "path": "skills/pptx",
  "enabled": true,
  "created_at": 1711000000000
}
```

**Check Name Request:**
```json
{
  "name": "pptx"
}
```

**Check Name Response:**
```json
{
  "exists": true,
  "message": "Skill 'pptx' 已存在，是否覆盖？"
}
```

**Create Request:**
```json
{
  "name": "pptx",
  "description": "处理 PPTX 文件的 Skill",
  "skillType": "tool",
  "version": "1.0.0",
  "path": "skills/pptx",
  "enabled": true,
  "config": {}
}
```

**Create Response:**
```json
{
  "ulid": "01ARZ3NDEKTSV4RRFFQ69G5FAV",
  "name": "pptx",
  "description": "处理 PPTX 文件的 Skill",
  "skillType": "tool",
  "version": "1.0.0",
  "path": "skills/pptx",
  "enabled": true,
  "config": {},
  "created_at": 1711000000000,
  "updated_at": 1711000000000
}
```

## 5. 前端页面

- 前端页面：`SkillManager.tsx`
- 需要新增组件

## 6. 实现要点

1. **ZIP 解压缩** - 使用 Go 标准库 `archive/zip` 解压缩
2. **同名覆盖检测** - 上传前检查 `skills/` 目录是否已存在同名 Skill
3. **SKILL.md 解析** - 解析 YAML 头信息提取元数据
4. **软删除** - deleted_at 标记，非真正删除
5. **文件保留** - 删除时只删除数据库记录，保留 `skills/` 下原始文件

## 7. 落点清单

### Backend (agent-frame)

1. **infra/repository/po/skill/**
   - `sys_skill_po.go` - Skill PO

2. **infra/repository/converter/skill/**
   - `sys_skill_conv.go` - DTO/Entity 转换

3. **domain/entity/skill/**
   - `sys_skill_entity.go` - Skill 领域实体

4. **domain/irepository/skill/**
   - `i_sys_skill_repo.go` - Skill 仓库接口

5. **infra/repository/repo/skill/**
   - `sys_skill_impl.go` - Skill 仓库实现

6. **domain/srv/skill/**
   - `sys_skill_svc.go` - Skill 领域服务

7. **application/dto/skill/**
   - `sys_skill_dto.go` - DTO 定义

8. **application/assembler/skill/**
   - `sys_skill_dto.go` - Assembler 转换

9. **application/service/skill/**
   - `sys_skill_svc.go` - 应用服务

10. **api/http/router/**
    - `sys_router.go` - 注册路由

11. **api/http/handler/public/skill/**
    - `handler.go` - Handler 结构体
    - `sys_skill_handler.go` - HTTP 处理函数

### Frontend (agent-ui)

1. **src/lib/api.ts**
   - 添加 skillApi 调用方法

2. **src/components/SkillManager.tsx**
   - Skill 列表展示
   - 上传安装功能
   - 编辑/删除功能
   - 搜索/筛选功能

## 8. 参考

- Skill 示例：`skills/pptx/SKILL.md`
- DDD 分层架构示例：sys_model / sys_knowledge_base 模块
