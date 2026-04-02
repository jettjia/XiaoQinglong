# Runner 使用指南

## 目录

- [快速开始](#快速开始)
- [内置工具](#内置工具)
- [CLI 扩展](#cli-扩展)
- [定时任务](#定时任务)
- [Sub-Agent](#sub-agent)
- [审批策略](#审批策略)

---

## 快速开始

### 启动服务

```bash
cd backend/runner
go build -o runner .
./runner
```

服务默认监听 `http://localhost:18080`

### 基本请求

```bash
curl -X POST http://localhost:18080/run \
  -H "Content-Type: application/json" \
  -d @example/test-all.json
```

---

## 内置工具

Runner 内置了 17 个常用工具，Agent 可以直接调用：

| 工具名              | 类型    | 说明           | 风险   |
| ------------------- | ------- | -------------- | ------ |
| `glob`              | builtin | 文件模式匹配   | low    |
| `grep`              | builtin | 内容搜索       | low    |
| `file_read`         | builtin | 文件读取       | low    |
| `file_edit`         | builtin | 字符串替换编辑 | low    |
| `file_write`        | builtin | 文件写入       | medium |
| `bash`              | builtin | 命令执行       | high   |
| `sleep`             | builtin | 延迟等待       | low    |
| `web_fetch`         | builtin | 网页抓取       | medium |
| `web_search`        | builtin | 网络搜索       | medium |
| `task_create`       | builtin | 创建任务       | low    |
| `task_get`          | builtin | 获取任务       | low    |
| `task_list`         | builtin | 列出任务       | low    |
| `task_update`       | builtin | 更新任务       | low    |
| `todo_write`        | builtin | 待办列表       | low    |
| `enter_plan_mode`   | builtin | 进入计划模式   | medium |
| `exit_plan_mode`    | builtin | 退出计划模式   | low    |
| `ask_user_question` | builtin | 向用户提问     | low    |

---

## CLI 扩展

### 概述

CLI 扩展允许 Agent 通过标准 CLI 工具（如 `lark-cli`）操作第三方服务。支持：
- 多租户隔离
- OAuth 授权流程
- 结构化命令执行

### 支持的 CLI

| CLI             | 安装命令                        | 授权类型      |
| --------------- | ------------------------------- | ------------- |
| lark-cli (飞书) | `npm install -g @larksuite/cli` | oauth2_device |

### 请求配置

```json
{
  "clis": [
    {
      "name": "lark",
      "command": "lark-cli",
      "config_dir": "/root/.config/lark-cli",
      "skills_dir": "/root/.skills/larksuite",
      "risk_level": "medium",
      "auth_type": "oauth2_device"
    }
  ]
}
```

| 字段         | 必填 | 说明            |
| ------------ | ---- | --------------- |
| `name`       | 是   | CLI 标识名      |
| `command`    | 是   | CLI 命令        |
| `config_dir` | 否   | Token 配置目录  |
| `skills_dir` | 否   | Skills 文件目录 |
| `risk_level` | 否   | low/medium/high |
| `auth_type`  | 否   | 授权类型        |

### 可用工具

Agent 可以调用以下工具：

#### cli_lark

执行飞书 CLI 命令。

```json
{
  "tool": "cli_lark",
  "input": {
    "command": "calendar +agenda",
    "args": "--format json",
    "format": "json"
  }
}
```

#### cli_auth

管理 CLI 授权。

```json
// 查看授权状态
{
  "tool": "cli_auth",
  "input": {
    "action": "status"
  }
}

// 开始授权
{
  "tool": "cli_auth",
  "input": {
    "action": "start",
    "cli": "lark"
  }
}

// 完成授权
{
  "tool": "cli_auth",
  "input": {
    "action": "complete",
    "cli": "lark",
    "device_code": "xxxx"
  }
}

// 登出
{
  "tool": "cli_auth",
  "input": {
    "action": "logout",
    "cli": "lark"
  }
}
```

### 完整使用示例

#### 1. 安装 CLI

```bash
npm install -g @larksuite/cli
npx skills add larksuite/cli -y -g
```

#### 2. 配置飞书应用

```
1. 打开 https://open.feishu.cn/app
2. 创建企业自建应用
3. 获取 App ID 和 App Secret
```

#### 3. 初始化配置

```bash
# 在终端执行（不是容器内）
lark-cli config init
lark-cli auth login --recommend
```

#### 4. 挂载配置到 Runner

如果 Runner 在 Docker 中运行：

```json
{
  "sandbox": {
    "enabled": true,
    "volumes": [
      {
        "host_path": "/home/user/.config/lark-cli",
        "container_path": "/root/.config/lark-cli",
        "read_only": true
      },
      {
        "host_path": "/home/user/.skills",
        "container_path": "/root/.skills",
        "read_only": true
      }
    ]
  }
}
```

#### 5. 发送请求

```json
{
  "prompt": "帮我看看今天的飞书日程",
  "models": {
    "default": {
      "provider": "openai",
      "name": "gpt-4o",
      "api_key": "sk-xxx"
    }
  },
  "clis": [
    {
      "name": "lark",
      "command": "lark-cli",
      "config_dir": "/root/.config/lark-cli",
      "skills_dir": "/root/.skills/larksuite",
      "risk_level": "medium",
      "auth_type": "oauth2_device"
    }
  ]
}
```

### OAuth 授权流程（多用户）

对于需要 OAuth 的 CLI（如飞书），Runner 支持设备码授权流程：

```
1. Agent 调用 cli_auth {action: "start", cli: "lark"}
2. Runner 返回 {verification_url, device_code}
3. 用户在浏览器打开 URL 完成授权
4. Agent 调用 cli_auth {action: "complete", cli: "lark", device_code: "xxx"}
5. 授权完成，Token 保存到 config_dir
```

---

## 定时任务

### 概述

通过 `/loop` 端点创建定时任务，支持 cron 表达式。

### 创建定时任务

```bash
curl -X POST http://localhost:18080/loop \
  -H "Content-Type: application/json" \
  -d '{
    "cron": "0 9 * * *",
    "prompt": "帮我看看今天的日程"
  }'
```

### 定时任务工具

Agent 可以使用以下内置工具管理定时任务：

| 工具名        | 说明             |
| ------------- | ---------------- |
| `cron_create` | 创建定时任务     |
| `cron_delete` | 删除定时任务     |
| `cron_list`   | 列出所有定时任务 |

### cron_create 用法

```json
{
  "tool": "cron_create",
  "input": {
    "name": "daily_agenda",
    "cron": "0 9 * * *",
    "prompt": "帮我看看今天的日程"
  }
}
```

### cron_delete 用法

```json
{
  "tool": "cron_delete",
  "input": {
    "name": "daily_agenda"
  }
}
```

### cron_list 用法

```json
{
  "tool": "cron_list",
  "input": {}
}
```

---

## Sub-Agent

### 概述

Sub-Agent 允许并行执行多个任务，通过 `spawn` 工具启动。

### 配置

```json
{
  "sub_agents": [
    {
      "id": "researcher",
      "name": "研究Agent",
      "description": "负责信息检索和文献研究",
      "prompt": "你是一个研究助手...",
      "max_iterations": 5,
      "timeout_ms": 120000
    }
  ]
}
```

### 可用工具

| 工具名         | 说明                   |
| -------------- | ---------------------- |
| `spawn`        | 异步并行执行多个 agent |
| `collect_task` | 获取异步任务结果       |
| `list_tasks`   | 列出所有任务           |
| `cancel_task`  | 取消任务               |
| `delegate`     | 同步执行单个 agent     |

### spawn 用法

```json
{
  "tool": "spawn",
  "input": {
    "tasks": [
      {"agent": "researcher", "prompt": "研究量子计算"},
      {"agent": "researcher", "prompt": "研究人工智能"}
    ]
  }
}
```

### collect_task 用法

```json
{
  "tool": "collect_task",
  "input": {
    "task_ids": ["task_001", "task_002"]
  }
}
```

---

## 审批策略

### 概述

对于高风险操作，可以配置审批策略。启用后，高风险工具调用会中断等待审批。

### 配置

```json
{
  "options": {
    "approval_policy": {
      "enabled": true,
      "risk_threshold": "medium",
      "auto_approve": ["task_create", "task_list"]
    }
  }
}
```

| 字段             | 说明               |
| ---------------- | ------------------ |
| `enabled`        | 是否启用审批       |
| `risk_threshold` | 需要审批的风险级别 |
| `auto_approve`   | 自动批准的工具列表 |

### 风险级别

| 级别     | 说明             |
| -------- | ---------------- |
| `low`    | 低风险，自动批准 |
| `medium` | 中风险，需要审批 |
| `high`   | 高风险，需要审批 |

### 审批流程

1. Agent 调用高风险工具
2. 请求中断，等待审批
3. 前端展示待审批项
4. 用户批准或拒绝
5. Agent 继续执行

### Resume 请求

```bash
curl -X POST http://localhost:18080/resume \
  -H "Content-Type: application/json" \
  -d '{
    "checkpoint_id": "xxx",
    "approvals": [
      {
        "interrupt_id": "int_001",
        "approved": true
      }
    ]
  }'
```

---

## 常见问题

### Q: 如何调试工具调用？

开启 trace 日志：

```bash
LOG_LEVEL=debug ./runner
```

### Q: 工具调用超时怎么办？

增加 `timeout_ms` 配置：

```json
{
  "options": {
    "timeout_ms": 120000
  }
}
```

### Q: 如何禁用某个工具？

不配置该工具即可。内置工具默认全部启用，可通过 `approval_policy.auto_approve` 控制。

### Q: CLI 授权过期了怎么办？

调用 `cli_auth {action: "logout"}` 登出，然后重新授权。
