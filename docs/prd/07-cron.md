# 周期任务（CRON）实现方案

## 1. 背景

用户需要在 Agent 配置中添加周期任务（CRON），并能在前端查看执行日志。

## 2. 设计目标

- 支持用户配置 Agent 的周期执行（秒/分/时/日级别）
- 动态管理 cron jobs（创建/更新/删除时动态调度）
- 独立存储执行日志，不撑爆 chat_message 表
- 保留策略：每个 agent 最多保留 100 条执行记录，自动清理最旧的
- 前端可查看执行历史

## 3. 数据模型

### 3.1 job_execution_log 表（周期任务执行日志）

```sql
CREATE TABLE job_execution_log (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    ulid VARCHAR(64) UNIQUE,
    agent_id VARCHAR(64) NOT NULL,
    agent_name VARCHAR(255),
    session_id VARCHAR(64),           -- 关联的 session
    status VARCHAR(20),               -- running, success, failed
    trigger_time BIGINT,              -- 触发时间戳
    started_at BIGINT,                -- 开始时间戳
    finished_at BIGINT,               -- 结束时间戳
    input_summary VARCHAR(500),       -- 输入摘要
    output_summary VARCHAR(1000),     -- 输出摘要（截断）
    output_full LONGTEXT,            -- 完整输出
    error_msg TEXT,                  -- 错误信息
    tokens_used INT,                 -- token消耗
    latency_ms BIGINT,               -- 延迟毫秒
    created_at BIGINT,
    updated_at BIGINT,
    deleted_at BIGINT DEFAULT 0,
    INDEX idx_agent_id (agent_id),
    INDEX idx_trigger_time (trigger_time)
);
```

### 3.2 SysAgent 表已有字段

- `is_periodic` - 是否周期任务
- `cron_rule` - Cron 表达式
- `last_run_at` - 上次运行时间（新增）
- `next_run_at` - 下次运行时间（新增）

## 4. 目录结构（遵循 agent-frame 框架）

```
backend/agent-frame/
├── domain/
│   ├── entity/
│   │   └── job/
│   │       └── job_execution_entity.go      # 领域实体
│   ├── irepository/
│   │   └── job/
│   │       └── i_job_execution_repo.go      # 仓库接口
│   └── srv/
│       └── job/
│           └── job_execution_svc.go          # 领域服务
├── infra/
│   └── repository/
│       ├── po/
│       │   └── job/
│       │       └── job_execution_po.go       # PO（数据库模型）
│       ├── repo/
│       │   └── job/
│       │       └── job_execution_impl.go     # 仓库实现
│       └── converter/
│           └── job/
│               └── job_execution_conv.go     # 实体<->PO转换器
├── application/
│   ├── dto/
│   │   └── job/
│   │       └── job_execution_dto.go          # DTO定义
│   ├── assembler/
│   │   └── job/
│   │       └── job_execution_dto.go         # DTO<->Entity转换器
│   └── service/
│       └── job/
│           └── job_execution_svc.go          # 应用服务
└── api/
    ├── job/
    │   └── manager.go                        # JobManager（cron调度器）
    └── http/
        └── handler/
            └── public/
                └── job/
                    ├── handler.go            # HTTP Handler
                    └── job_handler.go        # 处理器实现
```

## 5. 核心组件

### 5.1 JobManager（调度器）

- 单例模式，通过 `robfig/cron` 实现 cron 调度
- 启动时从 DB 加载所有 `is_periodic=true` 的 Agent
- 监听 Agent CRUD 事件，动态增删 cron jobs
- 执行时调用 runner service，传入完整 agent 配置
- 执行结果写入 `job_execution_log` 表

### 5.2 JobExecutionSvc（执行记录服务）

- 创建执行记录（running 状态）
- 更新执行结果（success/failed）
- 自动清理：超过 100 条时删除最旧的
- 查询执行历史

### 5.3 动态管理

```
Agent Create (IsPeriodic=true)
  → 验证 CronRule 格式
  → JobManager.AddCronJob(agent)
  → 保存到 DB

Agent Update (IsPeriodic 或 CronRule 变化)
  → JobManager.RemoveCronJob(agentID)
  → 如果仍需要: JobManager.AddCronJob(agent)

Agent Delete / IsPeriodic=false
  → JobManager.RemoveCronJob(agentID)
```

## 6. 前端实现

### 6.1 API 端点

- `GET /job/execution/:agent_id` - 获取 Agent 的执行历史
- `GET /job/execution/detail/:ulid` - 获取执行详情

### 6.2 UI 入口

在 AgentOrchestrator 详情页添加「执行历史」Tab

### 6.3 展示信息

- 时间、状态（成功/失败/运行中）
- 输入摘要、输出摘要
- Token 消耗、执行时长
- 点击查看完整输出

## 7. 依赖

- `github.com/robfig/cron/v3` - Cron 调度
- 数据库依赖使用现有的 `idata.NewDataOptionCli()`

## 8. 实现步骤

1. 创建 PO `job_execution_po.go`
2. 创建 Entity `job_execution_entity.go`
3. 创建 IRepository 接口
4. 创建 Converter
5. 创建 Repository 实现
6. 创建 Domain Service
7. 创建 DTO 和 Assembler
8. 创建 Application Service
9. 创建 JobManager（调度器）
10. 创建 HTTP Handler
11. 集成到 Agent CRUD
12. 前端 API 和 UI

## 9. 注意事项

- 周期任务不走 chat_message 表，避免数据库膨胀
- 执行日志默认保留 100 条，可配置
- 秒级 cron 需要注意时钟抖动和并发问题
- runner 调用超时设置为 10 分钟
