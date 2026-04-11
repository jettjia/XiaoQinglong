---
name: agent-browser
description: "网页搜索、打开网站、浏览器操作、获取网页数据、点击网页元素、填表提交、抓取网页内容"
---

# Agent Browser Skill

使用 agent-browser CLI 进行浏览器自动化操作。

## 核心命令

### 1. 打开网页
```bash
agent-browser open <url>
# 示例: agent-browser open https://www.baidu.com
```

### 2. 获取页面快照（关键！）
```bash
agent-browser snapshot -i --json
# 返回 JSON 格式的 accessibility tree，包含 ref 引用
# 示例输出: {"success": true, "data": {"snapshot": "...", "refs": {"e1": {...}, "e2": {...}}}}
```

### 3. 交互操作
```bash
agent-browser click @e2        # 点击元素
agent-browser fill @e3 "文本"   # 填写表单
agent-browser type @e3 "文本"   # 输入文本（逐字输入）
agent-browser press Enter       # 按键
agent-browser hover @e4        # 悬停
agent-browser scroll down 500   # 滚动
```

### 4. 获取数据
```bash
agent-browser get text @e1 --json          # 获取文本
agent-browser get attr @e2 "href" --json   # 获取属性
agent-browser get html @e3 --json           # 获取 HTML
agent-browser get value @e4 --json          # 获取值
agent-browser get title --json              # 获取标题
agent-browser get url --json                # 获取 URL
agent-browser get count ".item" --json      # 计数
```

### 5. 截图
```bash
agent-browser screenshot page.png           # 普通截图
agent-browser screenshot --full page.png    # 全屏截图
```

### 6. 等待
```bash
agent-browser wait --load networkidle   # 等待网络空闲
agent-browser wait 2000                # 等待毫秒
agent-browser wait --text "结果"        # 等待文本出现
```

### 7. 搜索（便捷命令）
```bash
agent-browser search "关键词"   # 相当于打开搜索引擎并搜索
```

## 工作流程

```
1. agent-browser open <url>
   ↓
2. agent-browser snapshot -i --json  → 获取页面 refs
   ↓
3. agent-browser click/fill @eN       → 交互
   ↓
4. agent-browser snapshot -i --json  → 再次获取
   ↓
5. agent-browser get text @eN --json  → 提取数据
```

## 使用 CDP 连接（远程浏览器）

如果需要连接远程浏览器（如 Windows 上的 Chrome）：

```bash
# 方式1: 通过 CDP 端口连接
agent-browser --cdp 192.168.x.x:9222 open https://example.com

# 方式2: 自动发现
agent-browser --auto-connect open https://example.com
```

## 注意事项

1. **每次页面变化后都需要重新 snapshot** - 获取最新的 refs
2. **使用 --json 输出** - 便于解析结构化数据
3. **使用 -i 标志** - 只获取交互元素，减少噪音
4. **等待网络空闲** - 动态页面需要等待加载完成

## 前置条件

- agent-browser CLI 已安装: `npm install -g agent-browser`
- 浏览器已启动，或使用 `--cdp` / `--auto-connect` 连接已有浏览器

## 示例：搜索并提取天气数据

```bash
# 1. 打开天气网站
agent-browser open https://weather.com

# 2. 获取快照，找到搜索框
agent-browser snapshot -i --json

# 3. 填入城市并搜索
agent-browser fill @e5 "Beijing"
agent-browser press Enter

# 4. 等待加载
agent-browser wait --load networkidle

# 5. 再次获取快照
agent-browser snapshot -i --json

# 6. 获取温度
agent-browser get text @e12 --json
```
