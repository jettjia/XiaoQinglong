#!/bin/bash
# XiaoQinglong Windows Build (Wails 2.x) - Cross Platform
# Usage: ./build.sh

set -e

cd "$(dirname "$0")"

echo "========================================"
echo "XiaoQinglong Windows Build (Wails 2.x)"
echo "========================================"
echo ""

# Check platform
PLATFORM=$(uname -s)
if [ "$PLATFORM" = "Darwin" ]; then
    PLATFORM_NAME="macOS"
elif [ "$PLATFORM" = "Linux" ]; then
    PLATFORM_NAME="Linux"
else
    PLATFORM_NAME="Windows (Git Bash)"
fi

echo "Detected platform: $PLATFORM_NAME"
echo ""

# Create output directory
mkdir -p build/bin

echo "[1/4] Building runner.exe..."
cd ../../backend/runner
GOOS=windows GOARCH=amd64 go build -o ../../deploy/win-wails/build/bin/runner.exe .
echo "      runner.exe built"

echo ""
echo "[2/4] Building xiaoqinglong.exe..."
cd ../../deploy/win-wails
wails build -platform windows/amd64
echo "      xiaoqinglong.exe built"

echo ""
echo "[3/4] Copying skills directory..."
cp -r ../../skills build/bin/
echo "      skills copied"

echo ""
echo "[4/4] Build complete!"
echo ""
echo "Output files:"
ls -la build/bin/
echo ""
echo "To deploy:"
echo "  1. Copy build/bin/ folder to Windows machine"
echo "  2. Run xiaoqinglong.exe"
