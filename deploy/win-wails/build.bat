@echo off
echo ========================================
echo XiaoQinglong Windows Build (Wails 2.x)
echo ========================================
echo.

cd /d "%~dp0"

echo [1/3] Installing frontend dependencies...
cd frontend
if exist package.json (
    call npm install
)
cd ..

echo.
echo [2/3] Building Wails application...
wails build -platform windows/amd64 -quiet

if errorlevel 1 (
    echo.
    echo Build failed!
    pause
    exit /b 1
)

echo.
echo [3/3] Build complete!
echo.
echo Output: %~dp0%xiaoqinglong.exe
echo.
pause
