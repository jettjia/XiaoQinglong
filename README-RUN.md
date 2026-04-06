# 如何运行项目

## 项目代码结构说明
```
├── backend
│   ├── agent-frame agent相关的业务
│   └── runner 执行器，可以运行多个实例
├── mock
│   ├── a2a 模拟第三方的a2a服务
│   ├── http 模拟第三方的http,tool服务
│   ├── kb-service 模拟外接知识库，数据源的服务
│   ├── mcp 模拟第三方的mcp服务
├── frontend
│   ├── agent-ui agent 前端d代码
├── docs
│   ├── prd 产品的模块设计说明，便于AI识别和快速编码
│   ├── sandbox 沙箱构建
├── deploy
│   ├── docker docker部署
```

## 快速开始
```bash
# 构建所有 Docker 镜像
make docker-build

# 单独构建某个服务的 Docker 镜像
make docker-build-frame   # 构建 agent-frame
make docker-build-runner  # 构建 runner
make docker-build-ui      # 构建 agent-ui

# 快速启动 agent-frame, runner, agent-ui
make quick-start

# 快速启动 mock 服务
make mock-start

# 快速停止 mock 服务
make mock-stop

```

# 测试 channel
## feishu

### 模式说明
飞书支持两种接收消息的模式：
- `webhook`：通过 HTTP 回调接收消息（被动）
- `websocket`：通过长连接接收消息（主动，推荐）

### Webhook 模式配置
```bash
export FEISHU_DOMAIN="open.feishu.cn"
export FEISHU_MODE="webhook"
export FEISHU_APP_ID="cli_xxx"
export FEISHU_APP_SECRET="xxx"
export FEISHU_ENCRYPT_KEY="your_encrypt_key"
export FEISHU_VERIFICATION_TOKEN="your_verification_token"  # 可选
```

### WebSocket 模式配置
```bash
export FEISHU_MODE="websocket"
export FEISHU_APP_ID="cli_xxx"
export FEISHU_APP_SECRET="xxx"
export FEISHU_ENCRYPT_KEY="your_encrypt_key"
export FEISHU_VERIFICATION_TOKEN="your_verification_token"  # 可选
export FEISHU_DOMAIN="feishu"  # 可选，默认 "lark"
```

### 环境变量说明
| 变量                      | 说明                                | 必填 |
| ------------------------- | ----------------------------------- | ---- |
| FEISHU_MODE               | 连接模式：`webhook` 或 `websocket`  | 是   |
| FEISHU_APP_ID             | 飞书应用 App ID                     | 是   |
| FEISHU_APP_SECRET         | 飞书应用 App Secret                 | 是   |
| FEISHU_ENCRYPT_KEY        | 消息加密密钥（用于签名验证）        | 是   |
| FEISHU_VERIFICATION_TOKEN | 飞书事件订阅验证 Token              | 否   |
| FEISHU_DOMAIN             | 飞书域名：`lark`（飞书）或 `feishu` | 否   |

## dingtalk
```bash
export DINGTALK_CLIENT_ID=your_client_id
export DINGTALK_CLIENT_SECRET=your_client_secret
export DINGTALK_MODE=websocket
```

## wework
```bash
export WEWORK_BOT_ID=your_bot_id
export WEWORK_SECRET=your_secret
export WEWORK_WS_URL=wss://openws.work.weixin.qq.com  # 可选，有默认值
export WEWORK_MODE=websocket
```

## wechat
```bash
http://127.0.0.1:9292/api/xiaoqinglong/agent-frame/v1/weixin/login/qrcode

http://127.0.0.1:9292/api/xiaoqinglong/agent-frame/v1/weixin/login/qrcode/image

http://127.0.0.1:9292/api/xiaoqinglong/agent-frame/v1/weixin/login

export WEIXIN_ACCOUNT_ID=default          # 可选，默认 "default"
export WEIXIN_MODE=longpolling           # 开启微信长轮询
```