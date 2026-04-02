# CLI 工具集成方案

## 一、背景与目标

AI Agent 需要操作飞书服务（日历、消息、文档等）。飞书官方提供 `lark-cli` CLI 工具，通过 Skill 机制封装了 19 个 AI Agent Skills，使 AI 能够准确调用。

**目标**：将 lark-cli 通过 Skill 机制接入 Runner，使 AI Agent 能够自主操作飞书服务。

## 二、整体架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Runner Framework                             │
│                                                                      │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐  │
│  │ BuiltinTools │  │   MCP Tools  │  │     CLI Skills           │  │
│  │   (17个)     │  │   (动态)     │  │                          │  │
│  └──────────────┘  └──────────────┘  │  lark-calendar (飞书)    │  │
│                                        │  lark-im (飞书)         │  │
│                                        │  lark-doc (飞书)        │  │
│                                        │  ...                    │  │
│                                        └──────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      Skill Runner 执行层                              │
│                                                                      │
│  ┌────────────────┐                                                  │
│  │ lark-cli       │  (npm install -g @larksuite/cli)                │
│  └────────────────┘                                                  │
│                                                                      │
│  Token 存储: 每个用户独立的 token 目录                                 │
└─────────────────────────────────────────────────────────────────────┘
```

## 三、多用户支持

### 3.1 三种部署模式

```
┌─────────────────────────────────────────────────────────────────────┐
│                     模式 A: 单用户独立 Runner                         │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│   用户 A 的机器                                                      │
│   ├── ~/.config/lark-cli-A/  (用户 A 的 token)                       │
│   └── Runner A (port 18080)                                          │
│                                                                      │
│   用户 B 的机器                                                      │
│   ├── ~/.config/lark-cli-B/  (用户 B 的 token)                       │
│   └── Runner B (port 18080)                                          │
│                                                                      │
│   适用：个人开发者、自建自用                                           │
│   优点：简单、隔离好                                                  │
│   缺点：资源占用多                                                   │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                     模式 B: 多用户共享 Runner (按目录隔离)             │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│   共享 Runner (port 18080)                                           │
│   └── 每个请求通过 header/path 指定用户                              │
│                                                                      │
│   Token 存储:                                                        │
│   ├── /var/run/lark-cli/tenant-A/config/  (用户 A)                  │
│   ├── /var/run/lark-cli/tenant-B/config/  (用户 B)                  │
│   └── /var/run/lark-cli/tenant-C/config/  (用户 C)                  │
│                                                                      │
│   适用：多租户 SaaS 服务                                              │
│   优点：资源复用、統一管理                                             │
│   缺点：需要处理目录映射                                              │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                     模式 C: Token 服务模式                            │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│   ┌────────────────┐     ┌─────────────────┐                       │
│   │  Runner         │────▶│  Token Service   │                       │
│   │  (无状态)       │     │  (管理 OAuth)    │                       │
│   └────────────────┘     └─────────────────┘                       │
│                                    │                                │
│                                    ▼                                │
│                           ┌─────────────────┐                       │
│                           │  DB / Redis     │                       │
│                           │  (存储 tokens)  │                       │
│                           └─────────────────┘                       │
│                                                                      │
│   适用：企业级多用户、需要 OAuth 代持                                   │
│   优点：最灵活、可代理多个企业的 OAuth                                 │
│   缺点：架构复杂                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### 3.2 模式 B 详细设计（推荐实现）

**请求时指定用户身份**：

```json
// 请求头
{
  "X-Tenant-ID": "tenant-001",
  "Authorization": "Bearer xxx"
}

// 或在请求中指定
{
  "tenant_id": "tenant-001",
  "prompt": "帮我查一下今天的日程"
}
```

**Token 目录映射**：

```json
{
  "lark_cli": {
    "tenants_dir": "/var/run/lark-cli",
    "tenant_config": {
      "tenant-001": {
        "config_dir": "/var/run/lark-cli/tenant-001/config",
        "skills_dir": "/var/run/lark-cli/tenant-001/skills"
      }
    }
  }
}
```

**执行时注入 Token**：

```go
func (sr *SkillRunner) ExecCLI(ctx context.Context, tenantID, command string) error {
    // 1. 根据 tenantID 获取配置目录
    configDir := fmt.Sprintf("/var/run/lark-cli/%s/config", tenantID)

    // 2. 设置环境变量，让 lark-cli 读取对应目录
    os.Setenv("LARK_CONFIG_DIR", configDir)

    // 3. 执行 lark-cli 命令
    return sr.bashTool.Exec(command)
}
```

### 3.3 OAuth 授权流程（多用户）

```
Step 1: 用户在网页端发起授权
┌─────────┐    POST /auth/lark/start     ┌─────────────┐
│  用户   │ ───────────────────────────▶ │   Runner    │
└─────────┘                             └─────────────┘
                                              │
                                              ▼
                                        ┌─────────────┐
                                        │ Lark OAuth  │
                                        │  获取 auth  │
                                        │   URL       │
                                        └─────────────┘
                                              │
                                              ▼
┌─────────┐     返回 auth URL               │
│  用户   │ ◀───────────────────────────────
└─────────┘
      │
      ▼
Step 2: 用户在浏览器完成授权
      │
      ▼
┌─────────┐    OAuth Callback            ┌─────────────┐
│  浏览器  │ ───────────────────────────▶ │   Runner    │
└─────────┘                             └─────────────┘
                                              │
                                              ▼
                                        ┌─────────────┐
                                        │ 保存 token  │
                                        │ 到 tenant   │
                                        │ 目录        │
                                        └─────────────┘
```

## 四、用户使用流程

### 4.1 安装 CLI 工具（管理员操作）

```bash
# 安装飞书 CLI
npm install -g @larksuite/cli

# 安装飞书 CLI Skills
npx skills add larksuite/cli -y -g
```

### 4.2 配置授权

**Step 1: 创建飞书应用**
```
1. 打开 https://open.feishu.cn/app
2. 创建企业自建应用
3. 获取 App ID 和 App Secret
```

**Step 2: 初始化配置并授权**
```bash
# 方式 A: 用户在终端手动授权（适合单用户）
lark-cli config init --new
lark-cli auth login --recommend

# 方式 B: OAuth 流程（适合多用户 SaaS）
POST /auth/lark/start
# 返回授权 URL，用户在浏览器完成授权
```

### 4.3 配置 Skills

```json
{
  "context": {
    "skills_dir": "/root/.skills"
  },
  "skills": [
    {
      "id": "lark-calendar",
      "name": "lark-calendar",
      "description": "飞书日历：查看和创建日程",
      "file_path": "/root/.skills/larksuite/lark-calendar/SKILL.md",
      "risk_level": "low"
    }
  ]
}
```

### 4.4 调用方式

**场景 1: 自然语言驱动**

```json
// 请求（单用户模式）
{
  "prompt": "帮我看看今天下午3点后有没有空"
}

// 请求（多用户模式）
{
  "tenant_id": "tenant-001",
  "prompt": "帮我看看今天下午3点后有没有空"
}
```

## 五、技术实现

### 5.1 需要修改的组件

| 组件 | 文件 | 修改内容 |
|------|------|---------|
| SkillRunner | `plugins/skill.go` | 支持 CLI 执行 + 多租户 |
| 类型定义 | `types/types.go` | 新增 CLI 相关配置 |
| Token 管理 | `plugins/lark_token.go` | 多租户 Token 管理 |

### 5.2 多租户 Token 管理

```go
// plugins/lark_token.go

type LarkTokenManager struct {
    baseDir string  // 如 "/var/run/lark-cli"
    mu      sync.RWMutex
}

func (m *LarkTokenManager) GetTokenDir(tenantID string) string {
    return filepath.Join(m.baseDir, tenantID, "config")
}

func (m *LarkTokenManager) SetupTenant(tenantID, appID, appSecret string) error {
    // 1. 创建租户目录
    dir := m.GetTokenDir(tenantID)

    // 2. 生成配置文件
    config := &LarkConfig{
        AppID:     appID,
        AppSecret: appSecret,
        Brand:     "lark",
    }

    // 3. 保存配置
    configPath := filepath.Join(dir, "config.json")
    return json.WriteFile(configPath, config)
}
```

### 5.3 SkillRunner CLI 执行

```go
func (sr *SkillRunner) ExecCLI(ctx context.Context, req *CLIExecRequest) (string, error) {
    // 1. 获取租户配置目录
    tokenDir := sr.tokenManager.GetTokenDir(req.TenantID)

    // 2. 设置环境变量
    env := []string{
        fmt.Sprintf("LARK_CONFIG_DIR=%s", tokenDir),
        fmt.Sprintf("HOME=%s", filepath.Dir(tokenDir)),
    }

    // 3. 构建命令
    cmd := fmt.Sprintf("%s %s", req.EntryScript, req.Args)

    // 4. 执行
    return sr.bashTool.ExecWithEnv(ctx, cmd, env)
}
```

## 六、部署架构

### 6.1 单用户模式

```
┌─────────────────────────────────────────────────────────────┐
│                        本地机器                              │
│                                                              │
│  ~/.config/lark-cli/     ← OAuth token                      │
│  ~/.skills/larksuite/    ← Skills 文件                       │
│                                                              │
│  ┌──────────────────┐                                        │
│  │ Runner (:18080)   │                                        │
│  └──────────────────┘                                        │
└─────────────────────────────────────────────────────────────┘
```

### 6.2 多用户模式

```
┌─────────────────────────────────────────────────────────────┐
│                        Docker 容器                           │
│                                                              │
│  /var/run/lark-cli/                                         │
│  ├── tenant-001/config/  ← 租户 A 的 token                  │
│  ├── tenant-002/config/  ← 租户 B 的 token                  │
│  └── ...                                                     │
│                                                              │
│  /root/.skills/           ← Skills 文件 (只读)               │
│                                                              │
│  ┌──────────────────┐                                        │
│  │ Runner (:18080)   │                                        │
│  └──────────────────┘                                        │
└─────────────────────────────────────────────────────────────┘
```

**docker-compose**：
```yaml
version: '3.8'
services:
  runner:
    image: runner:latest
    volumes:
      - ~/lark-cli-tenants:/var/run/lark-cli:rw
      - ~/.skills:/root/.skills:ro
      - ~/.config/lark-cli:/root/.config/lark-cli:ro  # 单用户fallback
```

## 七、安全考虑

### 7.1 Token 安全

| 措施 | 说明 |
|------|------|
| 目录隔离 | 每个租户独立目录 |
| 只读挂载 | 容器内只读，防止篡改 |
| Token 不落盘 | 内存中传递（Token 服务模式） |

### 7.2 风险级别

| Skill | 风险级别 | 说明 |
|-------|---------|------|
| lark-calendar | low | 只读日程查询 |
| lark-contact | low | 只读联系人查询 |
| lark-im | medium | 可发送消息 |
| lark-doc | medium | 可读写文档 |
| lark-drive | high | 可上传下载文件 |

### 7.3 审批策略

```json
{
  "approval_policy": {
    "enabled": true,
    "risk_threshold": "medium",
    "auto_approve": ["lark-calendar"]
  }
}
```

## 八、实现计划

| Step | 内容 | 优先级 |
|------|------|--------|
| 1 | SkillRunner 支持 CLI 命令执行 | P0 |
| 2 | 多租户 Token 目录管理 | P0 |
| 3 | OAuth 授权流程封装 | P1 |
| 4 | 前端配置界面 | P2 |

## 九、其他 CLI 规划（暂不实现）

| CLI 工具 | 安装命令 | 状态 |
|----------|---------|------|
| 钉钉 CLI | `npm install -g @dingtalk/cli` | 规划中 |
| 企业微信 CLI | `npm install -g @wecom/cli` | 规划中 |
| GitHub CLI | `apt install gh` | 规划中 |

## 十、FAQ

**Q: 多用户模式下，用户 A 能看到用户 B 的数据吗？**
A: 不能。Token 目录完全隔离，Runner 执行时根据 tenant_id 选择对应目录。

**Q: 如何管理租户？**
A: 需要管理员 API：创建租户、配置 App Credentials、删除租户。

**Q: OAuth 授权后的 token 会过期吗？**
A: lark-cli 的 OAuth token 通常有刷新机制，过期后会自动刷新。

**Q: Skills 文件需要每个租户一份吗？**
A: 不需要。Skills 文件是只读的，可以所有租户共享。
