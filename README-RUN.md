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