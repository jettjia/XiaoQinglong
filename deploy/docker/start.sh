#!/bin/bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "============================================"
echo "XiaoQinglong Docker 启动脚本"
echo "============================================"

# 加载环境变量
if [ -f .env.docker ]; then
    export $(cat .env.docker | grep -v '^#' | xargs)
fi

# 创建必要的目录
mkdir -p config/agent-frame/manifest/i18n

# 复制 i18n 配置（如果不存在）
if [ ! -d "config/agent-frame/manifest/i18n" ]; then
    cp -r ../../backend/agent-frame/manifest/i18n config/agent-frame/manifest/
fi

echo "正在构建 Docker 镜像..."

# 构建 Agent Frame 镜像
echo "构建 agent-frame..."
cd ../../backend/agent-frame
make dbuild
cd -

# 构建 Runner 镜像
echo "构建 runner..."
docker build -t xiaoqinglong/runner:latest ../../backend/runner

# 构建 Agent UI 镜像
echo "构建 agent-ui..."
docker build -t xiaoqinglong/agent-ui:latest ../../frontend/agent-ui

echo "启动 Docker 服务..."

# 启动 PostgreSQL 和所有服务
docker compose up -d

echo ""
echo "============================================"
echo "服务启动完成！"
echo "============================================"
echo "PostgreSQL:  localhost:5432"
echo "Agent Frame: localhost:6666 (HTTP)"
echo "            localhost:6667 (gRPC)"
echo "            localhost:6668 (Metrics)"
echo "Runner:      localhost:18080"
echo "Agent UI:    localhost:3000"
echo ""
echo "查看日志: docker compose logs -f"
echo "停止服务: ./stop.sh"
echo "============================================"
