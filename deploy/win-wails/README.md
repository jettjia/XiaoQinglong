# XiaoQinglong Windows 构建指南

## 前提条件

1. 安装 Go 1.25+
2. 安装 Wails 2.x:
   ```bash
   go install github.com/wailsapp/wails/v2/cmd/wails@latest
   ```

## 快速构建

### 方式一：使用脚本（Linux/macOS）

```bash
cd deploy/win-wails
./build.sh
```

### 方式二：手动构建

```bash
cd deploy/win-wails
wails build -platform windows/amd64
```

## 输出文件

构建完成后，文件在 `build/bin/` 目录：

```
build/bin/
├── xiaoqinglong.exe    # 主程序（Wails 窗口）
└── runner.exe          # runner 服务
```

## 运行

1. 将 `build/bin/` 下的两个 exe 文件拷贝到 Windows 同一目录
2. 双击 `xiaoqinglong.exe`
3. 窗口自动打开，显示前端界面
4. runner 服务自动启动

## 前端更新

如果修改了前端代码，需要重新复制静态资源：

```bash
cp -r frontend/agent-ui/dist/* frontend/
```

然后重新构建。

## 常见问题

### 1. WebView2 未安装
下载安装：https://developer.microsoft.com/en-us/microsoft-edge/webview2/

### 2. 数据库初始化失败
检查 `manifest/config/config.yaml` 配置是否正确

### 3. 构建失败
确保在 Linux/macOS 上使用 `-platform windows/amd64` 交叉编译

### 4. 查看详细日志
```bash
wails build -platform windows/amd64 -debug
```
然后运行 debug 版本，会显示控制台日志
