#!/bin/bash
set -e

IMAGE_NAME="my-sandbox/code-interpreter"
IMAGE_TAG="${1:-v1.0.2-pip}"
FULL_IMAGE="${IMAGE_NAME}:${IMAGE_TAG}"

echo "=========================================="
echo "测试沙箱镜像: ${FULL_IMAGE}"
echo "=========================================="

# 测试1: Python和pip版本
echo ""
echo ">>> 测试1: Python环境"
docker run --rm --entrypoint /bin/sh "${FULL_IMAGE}" -lc "python3 --version && pip3 --version"

# 测试2: 检查预装的包
echo ""
echo ">>> 测试2: 检查预装包"
docker run --rm --entrypoint /bin/sh "${FULL_IMAGE}" -lc "python3 -c 'import PIL; import defusedxml; import lxml; import markitdown; print(\"所有预装包导入成功!\")'"

# 测试3: 动态安装新包
echo ""
echo ">>> 测试3: 动态安装新包"
docker run --rm --entrypoint /bin/sh "${FULL_IMAGE}" -lc "pip3 install --quiet httpx && python3 -c 'import httpx; print(\"httpx 安装成功!\")'"

# 测试4: 执行一个简单的skill命令
echo ""
echo ">>> 测试4: 执行测试命令"
docker run --rm --entrypoint /bin/sh "${FULL_IMAGE}" -lc "echo 'Hello from sandbox' && python3 -c 'print(\"Python works!\")'"

echo ""
echo "=========================================="
echo "所有测试通过！"
echo "=========================================="