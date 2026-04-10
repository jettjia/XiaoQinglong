#!/bin/bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "============================================"
echo "XiaoQinglong Docker 停止脚本"
echo "============================================"

echo "正在停止 Docker 服务..."
docker compose down

echo "清理未使用的 Docker 资源..."
docker system prune -f

echo ""
echo "============================================"
echo "服务已停止"
echo "============================================"
