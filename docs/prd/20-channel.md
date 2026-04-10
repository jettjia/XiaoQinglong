# Channel 架构实现方案

## 一、背景与目标

### 1.1 背景

当前 agent-frame 通过 `runner_handler.go` 统一处理聊天请求，但存在以下问题：

- **渠道耦合**：所有请求被假设为来自 Web 端，`channel_id` 被硬编码为 `"web"`
- **响应格式单一**：无法根据不同渠道（如飞书、微信）进行格式适配
- **扩展性差**：新增渠道需要修改核心代码

### 1.2 目标

构建一个统一的 Channel 层，实现：
- 接收来自不同渠道的消息（web, 飞书, 微信等）
- 调用 Runner 处理消息
- 将响应发送回对应的渠道
- 各渠道消息格式的解析和转换

### 1.3 参考实现

| 系统 | 架构模式 | 关键设计 |
|------|---------|---------|
| **Claude Code** | Transport 抽象层 | 统一的 Transport 接口，适配器模式 |
| **OpenClaw** | ChannelPlugin + Adapter | 大型插件对象 + 多 Adapter 组合 |

---

## 二、整体架构

### 2.1 架构图

```
┌─────────────────────────────────────────────────────────────────────┐
│                           Channel Layer                              │
│                                                                      │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐ │
│  │  Web Handler │  │ Feishu      │  │ Wechat      │  │ Future      │ │
│  │             │  │ Handler     │  │ Handler     │  │ Handlers    │ │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └─────────────┘ │
│         │                │                │                          │
│         │                │                │                          │
│         └────────────────┼────────────────┘                          │
│                          ▼                                           │
│              ┌───────────────────────┐                                │
│              │   ChannelDispatcher   │  ← 统一调度器                  │
│              └───────────┬───────────┘                                │
│                          │                                            │
│                          ▼                                            │
│              ┌───────────────────────┐                                │
│              │  ChannelContext       │  ← 统一上下文                   │
│              │  - channel_code       │                                │
│              │  - session_id         │                                │
│              │  - user_id           │                                │
│              │  - agent_id           │                                │
│              │  - request (raw)      │                                │
│              └───────────┬───────────┘                                │
│                          │                                            │
│                          ▼                                            │
│              ┌───────────────────────┐                                │
│              │     RunnerCaller      │  ← 调用 Runner                 │
│              └───────────┬───────────┘                                │
│                          │                                            │
│                          ▼                                            │
│              ┌───────────────────────┐                                │
│              │   OutboundAdapter     │  ← 渠道特定输出                 │
│              │  - SendText()         │                                │
│              │  - SendRichText()     │                                │
│              │  - SendStream()       │                                │
│              └───────────────────────┘                                │
└─────────────────────────────────────────────────────────────────────┘
```

### 2.2 消息流

```
[渠道]          [Channel Layer]       [Runner]         [Outbound]
  │                   │                  │                │
  │  接收消息         │                  │                │
  │ ─────────────────▶│                  │                │
  │                   │  ParseRequest    │                │
  │                   │ ───────────────▶ │                │
  │                   │                  │                │
  │                   │     Run Request  │                │
  │                   │ ─────────────────▶│                │
  │                   │                  │                │
  │                   │                  │  SSE/JSON      │
  │                   │                  │ ──────────────▶│
  │                   │                  │                │ 格式化发送
  │                   │                  │                │ ──────────▶ [用户]
```

---

## 三、核心接口定义

### 3.1 ChannelContext - 渠道上下文

```go
// channel/channel_context.go

// ChannelContext 统一渠道上下文
type ChannelContext struct {
    ChannelCode string            // 渠道标识: web, feishu, wechat
    ChannelID   string            // 渠道配置ID (sys_channel.ulid)
    SessionID   string            // 会话ID
    UserID      string            // 用户ID
    AgentID     string            // Agent ID
    Config      map[string]any    // 渠道特定配置 (从 sys_channel.config 获取)
    Request     any               // 原始请求对象
    Header      http.Header       // HTTP Header
}

// ChatRunReq 统一聊天请求
type ChatRunReq struct {
    AgentID   string     `json:"agent_id"`
    UserID    string     `json:"user_id"`
    SessionID string     `json:"session_id"`
    Input     string     `json:"input"`
    Files     []FileInfo `json:"files"`
    IsTest    bool       `json:"is_test"`
}
```

### 3.2 InboundHandler - 入站消息处理接口

```go
// channel/inbound.go

// InboundHandler 入站消息处理接口
type InboundHandler interface {
    // 获取支持的渠道代码
    GetChannelCode() string

    // 从请求解析 ChannelContext
    ParseRequest(c *gin.Context) (*ChannelContext, error)

    // 验证请求合法性（签名、token等）
    Validate(c *gin.Context) error

    // 获取渠道配置
    GetChannelConfig(code string) (*entity.SysChannel, error)
}
```

### 3.3 OutboundHandler - 出站消息处理接口

```go
// channel/outbound.go

// OutboundHandler 出站消息处理接口
type OutboundHandler interface {
    // 获取支持的渠道代码
    GetChannelCode() string

    // 发送文本消息
    SendText(ctx *ChannelContext, text string) error

    // 发送富文本消息 (markdown, cards)
    SendRichText(ctx *ChannelContext, content any) error

    // 发送流式消息 (SSE)
    SendStream(ctx *ChannelContext, reader io.Reader) error

    // 发送错误消息
    SendError(ctx *ChannelContext, err error)

    // 确认消息收到 (用于 webhook 类渠道)
    Ack(ctx *ChannelContext) error
}
```

### 3.4 ChannelDispatcher - 统一调度器

```go
// channel/dispatcher.go

// ChannelDispatcher 渠道调度器
type ChannelDispatcher struct {
    inboundHandlers  map[string]InboundHandler
    outboundHandlers map[string]OutboundHandler
    runnerURL        string
    agentSvc         *agentSvc.SysAgentService
    chatSvc          *chatSvc.ChatMessageService
    memorySvc        *memorySvc.AgentMemorySvc
}

// Dispatch 统一处理入口
func (d *ChannelDispatcher) Dispatch(c *gin.Context) {
    // 1. 从 URL 路径获取 channel_code (如 /api/v1/channels/{channel}/run)
    // 2. 获取对应 handler
    // 3. Validate
    // 4. ParseRequest -> ChannelContext
    // 5. 调用 Runner
    // 6. 通过 OutboundHandler 发送响应
}
```

---

## 四、具体渠道实现

### 4.1 Web 渠道

**路径**: `api/http/handler/public/channel/web/handler.go`

**端点**:
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/channels/web/run` | 聊天请求 |
| POST | `/api/v1/channels/web/stream` | 流式聊天请求 |

**特点**:
- 请求格式：标准 JSON (`ChatRunReq`)
- 响应格式：JSON 或 SSE 流式
- 验证：无需特殊验证（已在 Agent 层面验证）

```go
type WebHandler struct {
    dispatcher *ChannelDispatcher
}

func (h *WebHandler) GetChannelCode() string {
    return "web"
}

func (h *WebHandler) ParseRequest(c *gin.Context) (*ChannelContext, error) {
    var req ChatRunReq
    if err := c.ShouldBindJSON(&req); err != nil {
        return nil, err
    }

    return &ChannelContext{
        ChannelCode: "web",
        SessionID:   req.SessionID,
        UserID:      req.UserID,
        AgentID:     req.AgentID,
        Request:     req,
        Header:      c.Request.Header,
    }, nil
}
```

### 4.2 飞书渠道

**路径**: `api/http/handler/public/channel/feishu/handler.go`

**端点**:
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/channels/feishu/callback` | 飞书事件回调 |

**飞书消息格式**:
```go
// 飞书事件回调格式
type FeishuCallback struct {
    Header struct {
        EventID   string `json:"event_id"`
        EventType string `json:"event_type"` // im.message.receive_v1
        CreateTime string `json:"create_time"`
        Token     string `json:"token"`
        AppID     string `json:"app_id"`
        TenantKey string `json:"tenant_key"`
    } `json:"header"`
    Event struct {
        Sender struct {
            SenderID struct {
                OpenID  string `json:"open_id"`
                UserID  string `json:"user_id"`
                UnionID string `json:"union_id"`
            } `json:"sender_id"`
            SenderType string `json:"sender_type"` // user, bot
            TenantKey  string `json:"tenant_key"`
        } `json:"sender"`
        Message struct {
            MessageID string `json:"message_id"`
            CreateTime string `json:"create_time"`
            ChatID    string `json:"chat_id"`
            Text      string `json:"text"`
            Content   string `json:"content"` // JSON string
        } `json:"message"`
    } `json:"event"`
}
```

**验证**:
- 飞书 signature 验证：`X-Lark-Signature` header
- 使用 `ENCRYPT_KEY` 进行校验

```go
type FeishuHandler struct {
    dispatcher *ChannelDispatcher
    encryptKey string
}

func (h *FeishuHandler) Validate(c *gin.Context) error {
    signature := c.GetHeader("X-Lark-Signature")
    if signature == "" {
        return errors.New("missing signature")
    }
    // 验证逻辑：HMAC-SHA256
    body, _ := io.ReadAll(c.Request.Body)
    expected := h.sign(body)
    if signature != expected {
        return errors.New("invalid signature")
    }
    return nil
}

func (h *FeishuHandler) ParseRequest(c *gin.Context) (*ChannelContext, error) {
    var callback FeishuCallback
    if err := json.Unmarshal(body, &callback); err != nil {
        return nil, err
    }

    // 解析用户消息内容
    content := callback.Event.Message.Content
    var msgContent struct {
        Text string `json:"text"`
    }
    json.Unmarshal([]byte(content), &msgContent)

    return &ChannelContext{
        ChannelCode: "feishu",
        SessionID:   callback.Event.Message.ChatID,
        UserID:      callback.Event.Sender.SenderID.OpenID,
        AgentID:     "", // 从 session 或配置获取
        Request:     callback,
        Header:      c.Request.Header,
    }, nil
}
```

### 4.3 微信渠道

**路径**: `api/http/handler/public/channel/wechat/handler.go`

**端点**:
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/channels/wechat/callback` | 微信验证 (GET) |
| POST | `/api/v1/channels/wechat/callback` | 微信事件回调 (POST) |

**微信消息格式**:
```go
// 微信回调格式
type WechatCallback struct {
    ToUserName   string `xml:"ToUserName"`
    FromUserName string `xml:"FromUserName"`
    CreateTime   string `xml:"CreateTime"`
    MsgType      string `xml:"MsgType"` // text, image, voice, event
    Content      string `xml:"Content"`
    MsgID        string `xml:"MsgId"`
    Event        string `xml:"Event"` // subscribe, unsubscribe, CLICK, VIEW
}
```

**验证**:
- URL 验证：微信后台配置时需要
- 使用 `GET /callback` 返回 `echostr` 参数

```go
type WechatHandler struct {
    dispatcher  *ChannelDispatcher
    token       string
    appID       string
    appSecret   string
}

func (h *WechatHandler) Validate(c *gin.Context) error {
    // 微信 signature 验证
    signature := c.GetHeader("X-WX-Signature")
    timestamp := c.Query("timestamp")
    nonce := c.Query("nonce")

    // 验证逻辑：token, timestamp, nonce 字典序排序后 SHA1
    expected := h.sign(timestamp, nonce, h.token)
    if signature != expected {
        return errors.New("invalid signature")
    }
    return nil
}

func (h *WechatHandler) ParseRequest(c *gin.Context) (*ChannelContext, error) {
    var callback WechatCallback
    if err := c.ShouldBindXML(&callback); err != nil {
        return nil, err
    }

    return &ChannelContext{
        ChannelCode: "wechat",
        SessionID:   callback.FromUserName,
        UserID:      callback.FromUserName,
        AgentID:     "",
        Request:     callback,
        Header:      c.Request.Header,
    }, nil
}
```

---

## 五、Outbound 输出适配

### 5.1 Web Outbound

```go
type WebOutboundHandler struct{}

func (h *WebOutboundHandler) SendText(ctx *ChannelContext, text string) error {
    // 直接返回 JSON
    ctx.Response.JSON(200, gin.H{"content": text})
    return nil
}

func (h *WebOutboundHandler) SendStream(ctx *ChannelContext, reader io.Reader) error {
    // SSE 流式输出
    ctx.Response.Header().Set("Content-Type", "text/event-stream")
    // ... copy reader to response
    return nil
}
```

### 5.2 飞书 Outbound

```go
type FeishuOutboundHandler struct {
    client *lark.Client
}

func (h *FeishuOutboundHandler) SendText(ctx *ChannelContext, text string) error {
    callback := ctx.Request.(*FeishuCallback)

    // 构造飞书消息
    msg := lark.CreateMessageReq{
        ReceiveID: callback.Event.Sender.SenderID.OpenID,
        MsgType:   "text",
        Content: map[string]any{
            "text": text,
        },
    }

    _, err := h.client.Im.Message.Create(context.Background(), &msg)
    return err
}

func (h *FeishuOutboundHandler) SendRichText(ctx *ChannelContext, content any) error {
    // 发送飞书富文本消息 (卡片)
    // ...
    return nil
}
```

### 5.3 微信 Outbound

```go
type WechatOutboundHandler struct{}

func (h *WechatOutboundHandler) SendText(ctx *ChannelContext, text string) error {
    callback := ctx.Request.(*WechatCallback)

    // 构造微信消息 XML
    xmlResp := fmt.Sprintf(`<xml>
        <ToUserName><![CDATA[%s]]></ToUserName>
        <FromUserName><![CDATA[%s]]></FromUserName>
        <CreateTime>%d</CreateTime>
        <MsgType><![CDATA[text]]></MsgType>
        <Content><![CDATA[%s]]></Content>
    </xml>`, callback.FromUserName, callback.ToUserName, time.Now().Unix(), text)

    ctx.Response.Header().Set("Content-Type", "application/xml")
    ctx.Response.Write([]byte(xmlResp))
    return nil
}
```

---

## 六、文件结构

```
backend/agent-frame/
├── api/http/handler/public/channel/           # NEW
│   ├── channel.go                             # 核心接口定义
│   │    ├── ChannelContext
│   │    ├── InboundHandler
│   │    ├── OutboundHandler
│   │    └── ChatRunReq
│   ├── dispatcher.go                           # ChannelDispatcher
│   ├── web/
│   │   ├── handler.go                          # Web InboundHandler
│   │   └── outbound.go                         # Web OutboundHandler
│   ├── feishu/
│   │   ├── handler.go                          # Feishu InboundHandler
│   │   ├── outbound.go                         # Feishu OutboundHandler
│   │   └── types.go                            # 飞书回调类型
│   └── wechat/
│       ├── handler.go                          # Wechat InboundHandler
│       ├── outbound.go                         # Wechat OutboundHandler
│       └── types.go                            # 微信回调类型
├── domain/entity/channel/sys_channel_entity.go  # 扩展：添加 config 字段
├── domain/srv/channel/sys_channel_svc.go       # 扩展：GetConfig
└── api/http/router/public/sys_router.go         # 修改：注册 channel 路由
```

---

## 七、路由设计

### 7.1 路由注册

```go
// sys_router.go

func SetupRoutes(r *gin.Engine) {
    channelDispatcher := channel.NewDispatcher()

    // 注册 handlers
    channelDispatcher.RegisterInboundHandler("web", web.NewHandler())
    channelDispatcher.RegisterInboundHandler("feishu", feishu.NewHandler())
    channelDispatcher.RegisterInboundHandler("wechat", wechat.NewHandler())

    channelDispatcher.RegisterOutboundHandler("web", web.NewOutboundHandler())
    channelDispatcher.RegisterOutboundHandler("feishu", feishu.NewOutboundHandler())
    channelDispatcher.RegisterOutboundHandler("wechat", wechat.NewOutboundHandler())

    // Web 渠道
    r.POST("/api/v1/channels/web/run", channelDispatcher.HandleRun())
    r.POST("/api/v1/channels/web/stream", channelDispatcher.HandleStream())

    // 飞书渠道
    r.POST("/api/v1/channels/feishu/callback", channelDispatcher.HandleCallback())

    // 微信渠道
    r.GET("/api/v1/channels/wechat/callback", wechat.HandleVerify())
    r.POST("/api/v1/channels/wechat/callback", channelDispatcher.HandleCallback())
}
```

### 7.2 端点汇总

| 渠道 | 方法 | 路径 | 说明 |
|------|------|------|------|
| Web | POST | `/api/v1/channels/web/run` | 聊天请求 |
| Web | POST | `/api/v1/channels/web/stream` | 流式聊天请求 |
| Feishu | POST | `/api/v1/channels/feishu/callback` | 飞书事件回调 |
| Wechat | GET | `/api/v1/channels/wechat/callback` | 微信 URL 验证 |
| Wechat | POST | `/api/v1/channels/wechat/callback` | 微信事件回调 |

---

## 八、渠道配置扩展

### 8.1 SysChannel 实体扩展

```go
// domain/entity/channel/sys_channel_entity.go

type SysChannel struct {
    // ... 现有字段 ...
    Config map[string]any `json:"config"` // 渠道特定配置
}

// 飞书配置示例
{
    "app_id": "cli_xxx",
    "app_secret": "xxx",
    "encrypt_key": "xxx",
    "bot_name": "小助手"
}

// 微信配置示例
{
    "app_id": "wx_xxx",
    "app_secret": "xxx",
    "token": "xxx",
    "encoding_aes_key": "xxx"
}
```

### 8.2 获取渠道配置

```go
func (h *InboundHandler) GetChannelConfig(code string) (*entity.SysChannel, error) {
    return h.channelSvc.FindByCode(context.Background(), code)
}
```

---

## 九、实现计划

### Phase 1: 核心抽象
| Step | 文件 | 内容 |
|------|------|------|
| 1 | `channel/channel.go` | 定义核心接口 (ChannelContext, InboundHandler, OutboundHandler) |
| 2 | `channel/dispatcher.go` | 实现 ChannelDispatcher |

### Phase 2: Web 渠道
| Step | 文件 | 内容 |
|------|------|------|
| 3 | `channel/web/handler.go` | Web InboundHandler |
| 4 | `channel/web/outbound.go` | Web OutboundHandler |

### Phase 3: 飞书渠道
| Step | 文件 | 内容 |
|------|------|------|
| 5 | `channel/feishu/types.go` | 飞书回调类型 |
| 6 | `channel/feishu/handler.go` | 飞书 InboundHandler + 签名验证 |
| 7 | `channel/feishu/outbound.go` | 飞书 OutboundHandler |

### Phase 4: 微信渠道
| Step | 文件 | 内容 |
|------|------|------|
| 8 | `channel/wechat/types.go` | 微信回调类型 |
| 9 | `channel/wechat/handler.go` | 微信 InboundHandler + 签名验证 |
| 10 | `channel/wechat/outbound.go` | 微信 OutboundHandler |

### Phase 5: 集成
| Step | 文件 | 内容 |
|------|------|------|
| 11 | `sys_channel_entity.go` | 扩展 Config 字段 |
| 12 | `sys_router.go` | 注册路由 |
| 13 | 单元测试 | 各渠道 Handler 测试 |

---

## 十、验证方式

### 10.1 编译测试
```bash
go build ./...
```

### 10.2 Web 渠道测试
```bash
curl -X POST http://localhost:8080/api/v1/channels/web/run \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id": "xxx",
    "user_id": "user1",
    "session_id": "sess1",
    "input": "你好"
  }'
```

### 10.3 飞书渠道测试
```bash
# 模拟飞书回调
curl -X POST http://localhost:8080/api/v1/channels/feishu/callback \
  -H "X-Lark-Signature: xxx" \
  -d '{
    "header": {
      "event_id": "xxx",
      "event_type": "im.message.receive_v1"
    },
    "event": {
      "sender": {
        "sender_id": {"open_id": "ou_xxx"}
      },
      "message": {
        "message_id": "msg_xxx",
        "chat_id": "chat_xxx",
        "content": "{\"text\":\"你好\"}"
      }
    }
  }'
```

### 10.4 微信渠道测试
```bash
# URL 验证 (GET)
curl "http://localhost:8080/api/v1/channels/wechat/callback?echostr=xxx&signature=xxx"

# 消息回调 (POST)
curl -X POST http://localhost:8080/api/v1/channels/wechat/callback \
  -d '<xml>
    <ToUserName>to</ToUserName>
    <FromUserName>from</FromUserName>
    <MsgType>text</MsgType>
    <Content>你好</Content>
  </xml>'
```

---

## 十一、后续扩展

### 11.1 未来渠道

| 渠道 | 优先级 | 说明 |
|------|--------|------|
| 钉钉 | P2 | 参考飞书实现 |
| Telegram | P2 | Bot API |
| Slack | P3 | WebSocket + Events API |
| Discord | P3 | Bot Gateway |

### 11.2 高级功能

| 功能 | 说明 |
|------|------|
| 渠道限流 | 不同渠道独立限流策略 |
| 渠道监控 | 各渠道请求量、延迟、错误率统计 |
| 故障转移 | 渠道服务异常时的降级策略 |
| 消息队列 | 高并发场景下的异步处理 |

---

## 十二、FAQ

**Q: 为什么不直接在 runner_handler.go 中判断 channel？**
A: 这样会导致 handler 膨胀，所有渠道逻辑混在一起。分离后更易维护和扩展。

**Q: 如何处理渠道特定的 token/secret？**
A: 存储在 SysChannel.Config 中，运行时获取。

**Q: 流式响应如何适配各渠道？**
A: Web 使用 SSE，飞书/微信暂不支持流式，采用轮询或 WebSocket 方案。

**Q: 渠道验证失败如何处理？**
A: 返回对应渠道要求的错误码（如微信返回纯文本），避免泄密。
