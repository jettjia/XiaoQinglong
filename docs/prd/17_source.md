# Plugin 数据源集成 - 实现计划

## Context

需要在 agent-frame 中实现插件化的数据源集成功能（飞书、腾讯文档、钉钉文档等），参考 sys_user 模块的 DDD 分层架构规范。

核心需求：
1. 插件市场：用户可以安装/卸载数据源插件
2. Web授权：用户在网页上完成 OAuth 授权
3. 多用户隔离：每个用户独立授权，独立存储
4. 安全存储：token 用 RSA 公钥加密

---

## 1. 调用链路（参考 sys_user）

```
handler (api/http/handler/public/plugin/)
    ↓
application/service/plugin (SysPluginService)
    ↓
domain/srv/plugin (SysPluginSvc) ←→ domain/entity/plugin
    ↓
domain/irepository/plugin (IPluginRepo, IRSAKeyManager)
    ↓
infra/repository/repo/plugin (SysPlugin, RSAKeyManager)
    ↓
infra/repository/po/plugin (PluginInstancePO, OAuthStatePO)
```

---

## 2. 文件落点清单

### 2.1 domain 层

**entity（已存在，需确认）**
- `domain/entity/plugin/plugin.go` - PluginInstance, OAuthState, PluginDefinition, PluginUserInfo

**irepository（需修改）**
- `domain/irepository/plugin/i_plugin_repo.go` - IPluginRepo, IRSAKeyManager 接口

**srv（需新建）**
- `domain/srv/plugin/sys_plugin_svc.go` - SysPluginSvc 领域服务

### 2.2 infra 层

**po（已存在，需确认）**
- `infra/repository/po/plugin/plugin_po.go` - PluginInstancePO, OAuthStatePO

**converter（需新建）**
- `infra/repository/converter/plugin/sys_plugin_conv.go` - E2P/P2E 转换

**repo（已存在，需修改）**
- `infra/repository/repo/plugin/sys_plugin_impl.go` - 实现 IPluginRepo
- `infra/repository/repo/plugin/rsa_key_mgr.go` - 实现 IRSAKeyManager

### 2.3 application 层

**dto（需新建）**
- `application/dto/plugin/sys_plugin_dto.go` - 请求/响应 DTO

**assembler（需新建）**
- `application/assembler/plugin/sys_plugin_dto.go` - DTO ↔ Entity 转换

**service（需新建）**
- `application/service/plugin/sys_plugin_svc.go` - SysPluginService

### 2.4 api 层

**handler（需新建）**
- `api/http/handler/public/plugin/handler.go` - Handler 结构体
- `api/http/handler/public/plugin/sys_plugin_handler.go` - HTTP 处理函数

**router（需修改）**
- `api/http/router/public/sys_router.go` - 注册 plugin 路由

### 2.5 boot（需修改）
- `boot/init_plugin.go` - 初始化 plugin 系统（如需要）

---

## 3. API 设计

| 方法 | 路由 | 说明 |
|------|------|------|
| GET | /plugin/list | 获取插件列表 |
| GET | /plugin/instances | 获取用户插件实例 |
| POST | /plugin/auth/start | 开始授权（device flow） |
| POST | /plugin/auth/poll | 轮询授权状态 |
| GET | /plugin/auth/url | 获取授权 URL |
| GET | /plugin/public-key | 获取 RSA 公钥 |
| DELETE | /plugin/instance/:id | 删除插件实例 |
| POST | /plugin/instance/:id/refresh | 刷新令牌 |

---

## 4. DTO 设计

```go
// GetPluginsReq / GetPluginsRsp
type GetPluginsRsp struct {
    Plugins []PluginItem
}
type PluginItem struct {
    ID, Name, Icon, Description, AuthType, Version, Author string
    Status string // available, installed, authorized
    InstanceID string
}

// GetUserInstancesReq / GetUserInstancesRsp
type GetUserInstancesRsp struct {
    Instances []InstanceItem
}
type InstanceItem struct {
    ID, PluginID, Status string
    UserInfo *PluginUserInfo
    AuthorizedAt time.Time
    ExpiresAt *time.Time
}

// StartAuthReq / StartAuthRsp
type StartAuthRsp struct {
    AuthType string // "device"
    DeviceCode, UserCode, VerificationURL string
    ExpiresIn, Interval int
    State string
}

// PollAuthReq / PollAuthRsp
type PollAuthRsp struct {
    Status string // pending, authorized
    InstanceID string
    UserInfo *PluginUserInfo
}

// RefreshTokenReq / RefreshTokenRsp
type RefreshTokenRsp struct {
    Status string // active, expired
}
```

---

## 5. Entity 设计（已有，需确认）

```go
// PluginInstance - 插件实例
type PluginInstance struct {
    ID, TenantID, UserID, PluginID string
    Status string // active, revoked, expired
    EncryptedToken, EncryptedAES string
    TokenVersion int
    Config map[string]string
    UserInfo *PluginUserInfo
    AuthorizedAt time.Time
    ExpiresAt *time.Time
    CreatedAt, UpdatedAt time.Time
}

// OAuthState - OAuth 状态
type OAuthState struct {
    State, TenantID, UserID, PluginID, CallbackURL string
    ExpiresAt, CreatedAt time.Time
}

// PluginDefinition - 插件定义
type PluginDefinition struct {
    ID, Name, Icon, Description, AuthType, Version, Author string
}

// PluginUserInfo - 用户信息
type PluginUserInfo struct {
    OpenID, Name, Avatar, Email string
}
```

---

## 6. 实施步骤

### Step 1: 确认/创建 domain/entity
- 检查 `domain/entity/plugin/plugin.go` 是否完整
- 如需要补充 Entity 定义

### Step 2: 创建/修改 domain/irepository
- 修改 `i_plugin_repo.go` 定义 IPluginRepo, IRSAKeyManager 接口

### Step 3: 创建 infra 层
- `infra/repository/converter/plugin/sys_plugin_conv.go`
- 修改 `infra/repository/repo/plugin/sys_plugin_impl.go` 实现 IPluginRepo
- 创建 `infra/repository/repo/plugin/rsa_key_mgr.go` 实现 IRSAKeyManager
- 确认 `infra/repository/po/plugin/plugin_po.go`

### Step 4: 创建 domain/srv
- `domain/srv/plugin/sys_plugin_svc.go`

### Step 5: 创建 application 层
- `application/dto/plugin/sys_plugin_dto.go`
- `application/assembler/plugin/sys_plugin_dto.go`
- `application/service/plugin/sys_plugin_svc.go`

### Step 6: 创建 api 层
- `api/http/handler/public/plugin/handler.go`
- `api/http/handler/public/plugin/sys_plugin_handler.go`
- 修改 `api/http/router/public/sys_router.go` 注册路由

### Step 7: 添加 AutoMigrate
- 修改 `infra/repository/po/auto_table.go` 添加 PluginInstancePO, OAuthStatePO

---

## 7. 参考文件

- 规范文档：`backend/agent-frame/skills.md`
- DDD 分层示例：sys_user 模块
  - handler: `api/http/handler/public/user/`
  - service: `application/service/user/sys_user_svc.go`
  - srv: `domain/srv/user/sys_user_svc.go`
  - repo: `infra/repository/repo/user/sys_user_impl.go`
  - po: `infra/repository/po/user/sys_user_po.go`
  - converter: `infra/repository/converter/user/sys_user_conv.go`
  - dto: `application/dto/user/sys_user_dto.go`
  - assembler: `application/assembler/user/sys_user_dto.go`

---

## 8. 验证方案

1. `go build ./...` - 确保编译通过
2. 启动服务，调用 `/api/xiaoqinglong/agent-frame/v1/plugin/list` 验证路由
3. 测试完整 OAuth 流程（如有飞书 app 配置）
