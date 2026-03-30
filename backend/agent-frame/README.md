# Agent-Frame 服务说明

智能体编排框架服务，负责 AI 智能体的配置、管理、编排和执行。支持 Skills、MCP、Tools、A2A 等多种工具类型，提供审批流、渠道管理、对话历史和 Token 统计等功能。

## 服务架构

```
┌─────────────┐     ┌──────────────┐     ┌───────────┐
│   前端       │────▶│  Agent-Frame  │────▶│   Runner   │
│  (Agent-UI) │◀────│  (Agent-API)  │◀────│  (Agent)   │
└─────────────┘     └──────────────┘     └───────────┘
                           │
                           ▼
                    ┌──────────────┐
                    │   数据库      │
                    │ (GORM)       │
                    └──────────────┘
```

## 核心功能

### 1. 智能体管理 (Agent)
- 创建、编辑、删除智能体
- 配置智能体名称、描述、图标
- 设置渠道（web/api/feishu/dingtalk）
- 部署和上线管理

### 2. 技能与工具 (Skills)
| 类型 | 说明 |
|------|------|
| Skill | 自定义技能，支持 ZIP 包上传 |
| MCP | Model Context Protocol 连接器 |
| Tool | 外部工具服务 |
| A2A | Agent-to-Agent 协议服务 |

**风险等级**：每种工具可配置 Low/Medium/High 三个风险等级，用于审批流控制。

### 3. 模型管理 (Model)
- 配置 AI 模型（OpenAI、Claude 等）
- 设置 API Key 和接口地址
- 模型优先级排序

### 4. 知识库 (Knowledge Base)
- 向量数据库集成
- 知识库搜索和召回

### 5. 渠道管理 (Channel)
- API（接口调用）
- Web（网页端）
- Feishu（飞书）
- Dingtalk（钉钉）

### 6. 审批流 (Approval)
- 高风险操作需要人工审批
- 审批阈值可配置
- 待审批消息自动进入收件箱

### 7. 对话与 Token 统计
- 聊天会话管理
- 消息历史记录
- Token 消耗统计（input_tokens、output_tokens、total_tokens）
- 按日期、模型聚合统计

## 技术特性

- **DDD 架构**：Entity、PO、Repository、Service、Handler 分层清晰
- **定时任务**：支持周期性任务配置（Cron Expression）
- **审批流**：人工审批中断执行流程
- **流式响应**：SSE (Server-Sent Events) 支持

---

# 服务基础配置信息

端口：

| 外部端口 | 内部端口 | GRPC端口 | 接口前缀                         |
| -------- | -------- | -------- | -------------------------------- |
| 6666     | 6667     | 6668     | /api/v1/group-name/service-name/ |

存储配置：

| 存储类型  | 数据库                           | 表前缀/索引等举例                        |
| --------- | -------------------------------- | ---------------------------------------- |
| mysql     |                                  |                                          |
|           | databasename                     | sys_*                                    |
|           |                                  | chat_*                                   |
| redis     | 1号库                            | agent_frame_*                            |
|           |                                  |                                          |
| mq        |                                  | databasename.agent.frame.folder.create   |
|           |                                  |                                          |
| es_search | databasename_agent_frame_index   | agent_frame_*                            |

---

# 端口和错误码

## 错误码和端口定义

| 模块大类      | 子模块        | 端口        | 中间三位错误码 | 案例      |
| ------------- | ------------- | ----------- | -------------- | --------- |
| agent-*       | agent-frame   | 对外：33000 | 000            | 400300100 |
|               |               | 对内：6667  | 001            | 400301100 |
|               |               | grpc：6668  | 002            | 400302100 |

## 错误码说明

错误码举例：

```json
{
    "code": 401300100,
    "cause": "token expired：xxxx",
    "message": "授权过期",
    "solution": "请刷新页面更新token或重新登录",
    "detail": {}
}
```

> - `code: 错误码（前三位：标准http错误码，中间三位为服务器特定码，后三位服务中自定义码）`
> - `cause: 错误原因，产生错误的具体原，比如错误的方法位置、行数`
> - `solution：符合国际化要求的针对当前错误的操作提示`
> - `message:  符合国际化要求的错误描述`
> - `detail:  错误码拓展信息，补充说明错误信息。`

错误码位数说明：

> 400300100
>
> 400 是 http 的状态码
>
> 300 是服务的 标记码
>
> 100 是服务定义的错误码

---

# 数据库表结构

## 系统表

| 表名 | 说明 |
|------|------|
| sys_agent | 智能体配置 |
| sys_skill | 技能/工具配置 |
| sys_model | AI 模型配置 |
| sys_knowledge_base | 知识库配置 |
| sys_channel | 渠道配置 |

## 聊天表

| 表名 | 说明 |
|------|------|
| chat_session | 聊天会话 |
| chat_message | 聊天消息（含 Token 统计） |
| chat_approval | 审批记录 |
| chat_token_stats | Token 消耗日统计 |

---

# 快速开始

## 配置文件

```
manifest/config/
```

## 国际化

```
manifest/i18n/
```

## 错误码定义

```
types/apierror
```

---

# 项目运行

## 配置

```bash
manifest/config/config.yaml
```

```yaml
# HTTP Server.
server:
  lang: zh-CN # "zh-CN", "zh-TW", "en"
  public_port: 6666 # 对外端口
  private_port: 6667 # 对内端口
  server_name: "agent-frame"
  mode: "debug" # gin的模式配置 debug, test, release
  dev: true # true,false;校验token等,开发模式的时候打开
  enable_event: false
  enable_job: true
  enable_grpc: false

# GRPC Server.
gserver:
  host: "0.0.0.0"
  public_port: 6668
  max_msg_size: 1024
  client_goods_host: "0.0.0.0"
  client_goods_port: 18080

# Database.
mysql:
  username: "root"
  password: "admin123"
  db_host: "127.0.0.1"
  db_port: 5432
  db_name: "xtext"
  charset: "utf8mb4"
  max_open_conn: 50
  max_idle_conn: 10
  conn_max_lifetime: 500
  log_mode: 4
  slow_threshold: 10

# Redis.
redis:
  host: "127.0.0.1"
  port: 6379
  password: ""
  db: 1
```

## 运行

```bash
# 直接运行
go run main.go

# 指定环境运行
go run main.go -env test

# 编译
go build
```

---

# API 接口

## 智能体接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/agent/sys-agent | 创建智能体 |
| GET | /api/v1/agent/sys-agent/:ulid | 获取智能体详情 |
| PUT | /api/v1/agent/sys-agent/:ulid | 更新智能体 |
| DELETE | /api/v1/agent/sys-agent/:ulid | 删除智能体 |
| GET | /api/v1/agent/sys-agent | 查询智能体列表 |

## 技能接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/skill/sys-skill | 创建技能 |
| GET | /api/v1/skill/sys-skill/:ulid | 获取技能详情 |
| PUT | /api/v1/skill/sys-skill/:ulid | 更新技能 |
| DELETE | /api/v1/skill/sys-skill/:ulid | 删除技能 |
| GET | /api/v1/skill/sys-skill | 查询技能列表 |
| POST | /api/v1/skill/upload | 上传 ZIP 包 |

## 聊天接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/chat/chat-session | 创建会话 |
| GET | /api/v1/chat/chat-session/:sessionId | 获取会话消息 |
| POST | /api/v1/chat/chat-message | 发送消息 |
| GET | /api/v1/chat/chat-approval | 获取待审批列表 |
| PUT | /api/v1/chat/chat-approval/:ulid | 审批操作 |

## Runner 接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/runner/run | 运行智能体 |
| POST | /api/v1/runner/resume | 审批后继续执行 |

---

# 支持多配置运行

```bash
go run main.go                          # 默认开发配置，debug模式
go run main.go -env test                # 测试环境配置
go run main.go -env release             # 正式环境配置
```

---

# 支持多协议并存

程序支持 HTTP 和 gRPC 两种协议：

- **外部接口**: http://127.0.0.1:6666/api/v1/agent/sys-agent
- **内部接口**: http://127.0.0.1:6667/private/v1/agent/sys-agent
- **gRPC 接口**: 127.0.0.1:6668

---

# 构建镜像

```bash
make gbuild   # 构建 Go 二进制文件
sudo make dbuild   # 构建 Docker 镜像
sudo make dpush    # 推送 Docker 镜像
```

---

# 开发调试

```bash
EINO_PROXY_DEBUG=1 go run .
```
