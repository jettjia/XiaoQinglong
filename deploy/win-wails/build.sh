#!/bin/bash
# XiaoQinglong Windows Build (Wails 2.x) - Single File Distribution
# All assets embedded into xiaoqinglong.exe

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

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

echo "[1/5] Building runner.exe..."
cd "$SCRIPT_DIR/../../backend/runner"
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 go build -o "$SCRIPT_DIR/build/bin/runner.exe" .
echo "      runner.exe built"

echo ""
echo "[2/5] Copying assets to embed directory..."

# Ensure bin directory exists at deploy/win-wails/ (for Go embed)
rm -rf "$SCRIPT_DIR/bin"
mkdir -p "$SCRIPT_DIR/bin"

# Copy runner.exe to bin/
cp "$SCRIPT_DIR/build/bin/runner.exe" "$SCRIPT_DIR/bin/"

# Copy skills to bin/
cp -r "$SCRIPT_DIR/../../skills" "$SCRIPT_DIR/bin/"

# Copy skills-config.yaml to bin/
cp "$SCRIPT_DIR/../../backend/runner/skills-config.yaml" "$SCRIPT_DIR/bin/"

# Copy config.yaml to bin/config/
mkdir -p "$SCRIPT_DIR/bin/config"
cp "$SCRIPT_DIR/../../backend/agent-frame/manifest/config/config.yaml" "$SCRIPT_DIR/bin/config/"

echo "      Assets copied to build/bin/"
ls -la "$SCRIPT_DIR/build/bin/"
echo ""

echo "[3/5] Building xiaoqinglong.exe with embedded assets..."
cd "$SCRIPT_DIR"
wails build -platform windows/amd64
echo "      xiaoqinglong.exe built with embedded assets"

echo ""
echo "[4/5] Verifying single file distribution..."
if [ -f "$SCRIPT_DIR/build/bin/xiaoqinglong.exe" ]; then
    SIZE=$(du -h "$SCRIPT_DIR/build/bin/xiaoqinglong.exe" | cut -f1)
    echo "      xiaoqinglong.exe: $SIZE"
    echo "      All assets embedded - single file distribution!"
else
    echo "      ERROR: xiaoqinglong.exe not found!"
    exit 1
fi

echo ""
echo "[5/5] Build complete!"
echo ""
echo "Distribution: Single file - xiaoqinglong.exe"
echo "No additional files needed!"
echo ""
ls -la "$SCRIPT_DIR/build/bin/xiaoqinglong.exe"
echo ""
echo "To deploy:"
echo "  1. Copy xiaoqinglong.exe to Windows machine"
echo "  2. Double-click xiaoqinglong.exe to run"
echo "  3. On first run, assets will be extracted to ~/.xiaoqinglong/"
