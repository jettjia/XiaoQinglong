#!/bin/bash
set -e

IMAGE="${1:-my-sandbox/code-interpreter:v1.0.2-pip}"
COMMAND="${2:-python3 --version}"

echo "=========================================="
echo "在沙箱中执行命令"
echo "镜像: ${IMAGE}"
echo "命令: ${COMMAND}"
echo "=========================================="

docker run --rm --entrypoint /bin/sh -v "$(pwd):/workspace" "${IMAGE}" -lc "${COMMAND}"