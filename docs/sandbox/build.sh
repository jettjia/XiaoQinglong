#!/bin/bash
set -e

IMAGE_NAME="my-sandbox/code-interpreter"
IMAGE_TAG="${1:-v1.0.2-pip}"
FULL_IMAGE="${IMAGE_NAME}:${IMAGE_TAG}"

echo "=========================================="
echo "构建自定义沙箱镜像: ${FULL_IMAGE}"
echo "=========================================="

# 登录阿里云Registry（如果需要）
# docker login --username=<your-username> registry.cn-zhangjiakou.cr.aliyuncs.com

# 拉取基础镜像
echo ">>> 拉取基础镜像..."
docker pull sandbox-registry.cn-zhangjiakou.cr.aliyuncs.com/opensandbox/code-interpreter:v1.0.2

# 构建镜像
echo ">>> 构建镜像..."
docker build -t "${FULL_IMAGE}" -f Dockerfile .

# 验证镜像
echo ">>> 验证镜像..."
docker run --rm --entrypoint /bin/sh "${FULL_IMAGE}" -lc "python3 --version && pip3 --version"

echo ""
echo "=========================================="
echo "构建完成！"
echo "镜像名称: ${FULL_IMAGE}"
echo "=========================================="
echo ""
echo "使用方式："
echo "1. 在 docker-compose.yml 中配置:"
echo "   image: ${FULL_IMAGE}"
echo ""
echo "2. 或在代码中设置环境变量:"
echo "   ORCH_SKILL_SANDBOX_DOCKER_IMAGE=${FULL_IMAGE}"