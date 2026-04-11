---
name: agent-browser
description: "网页搜索、打开网站、浏览器操作、获取网页数据、点击网页元素、填表提交、抓取网页内容、模拟用户操作浏览器"
trigger: "搜索、浏览、打开网站、点击、填表、爬取网页、获取网页数据、浏览器操作"
inputs: ["url", "search_query", "element_ref"]
outputs: ["text", "html", "screenshot", "json"]
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
agent-browser type @e3 "文本"  # 输入文本（逐字输入）
agent-browser press Enter       # 按键
agent-browser hover @e4        # 悬停
agent-browser scroll down 500   # 滚动（像素）
agent-browser scroll up 300     # 向上滚动
```

### 4. 获取数据
```bash
agent-browser get text @e1 --json          # 获取文本
agent-browser get attr @e2 "href" --json   # 获取属性
agent-browser get html @e3 --json           # 获取 HTML
agent-browser get value @e4 --json          # 获取值
agent-browser get title --json              # 获取标题
agent-browser get url --json                # 获取 URL
agent-browser get count ".item" --json     # 计数
```

### 5. 截图
```bash
agent-browser screenshot                    # 普通截图
agent-browser screenshot page.png           # 指定文件名
agent-browser screenshot --full page.png   # 全屏截图
```

### 6. 等待
```bash
agent-browser wait --load networkidle   # 等待网络空闲
agent-browser wait 2000                # 等待毫秒
agent-browser wait --text "结果"        # 等待文本出现
agent-browser wait @e5                 # 等待元素出现
```

### 7. 导航控制
```bash
agent-browser back      # 后退
agent-browser forward    # 前进
agent-browser reload     # 刷新
agent-browser close      # 关闭
```

## 标准工作流程（重要！）

```
1. agent-browser open <url>
   ↓
2. agent-browser snapshot -i --json  → 获取页面 refs
   ↓
3. agent-browser click/fill @eN       → 交互（根据 snapshot 结果选择正确 ref）
   ↓
4. agent-browser snapshot -i --json  → 页面变化后重新获取 refs
   ↓
5. agent-browser get text/attr @eN --json  → 提取数据
```

## 常见场景处理

### 场景1：搜索并提取结果

**问题**：搜索后找不到温度/数据在哪

**解决步骤**：
```bash
# 1. 打开搜索
agent-browser open "https://cn.bing.com/search?q=北京天气"

# 2. 获取快照
agent-browser snapshot -i --json

# 3. 如果数据不在可见区域，先滚动
agent-browser scroll down 500

# 4. 再次获取快照（页面变化后必须重新 snapshot！）
agent-browser snapshot -i --json

# 5. 找到目标元素的 ref（如 @e15），提取数据
agent-browser get text @e15 --json
```

### 场景2：天气/股票等 Widget 数据

**问题**：天气信息在页面顶部 widget 里，但 snapshot 看不到

**解决步骤**：
```bash
# 1. 打开页面
agent-browser open "https://www.bing.com/search?q=北京天气"

# 2. 截图看实际页面
agent-browser screenshot

# 3. 如果看到天气 widget 但 snapshot 没有，用 scroll 找到它
agent-browser scroll down 200
agent-browser snapshot -i --json

# 4. 或者直接用 get text 获取可能的位置
agent-browser get text --json  # 获取整个页面文本
```

### 场景3：表单填写和提交

**解决步骤**：
```bash
# 1. 打开页面
agent-browser open <url>

# 2. 获取快照找到输入框
agent-browser snapshot -i --json
# 假设搜索框是 @e3

# 3. 填写表单
agent-browser fill @e3 "搜索内容"

# 4. 点击搜索按钮（假设是 @e4）
agent-browser click @e4

# 5. 等待加载完成
agent-browser wait --load networkidle
```

### 场景4：点击链接进入详情页

**解决步骤**：
```bash
# 1. 获取快照
agent-browser snapshot -i --json

# 2. 找到目标链接的 ref（如 @e7）
agent-browser click @e7

# 3. 等待页面加载
agent-browser wait --load networkidle

# 4. 新页面需要重新获取快照
agent-browser snapshot -i --json
```

## 失败处理（重要！）

### 1. snapshot 返回空或很少元素
```bash
# 可能页面还没加载完
agent-browser wait --load networkidle
agent-browser snapshot -i --json
```

### 2. 元素 ref 找不到
```bash
# 页面可能更新了，需要重新获取 snapshot
agent-browser snapshot -i --json
```

### 3. 点击/填写没反应
```bash
# 先确认元素存在
agent-browser is visible @e3 --json
agent-browser is enabled @e3 --json

# 或者等待元素出现
agent-browser wait @e3
```

### 4. 页面是 JavaScript 渲染
```bash
# 先等待网络空闲
agent-browser wait --load networkidle
# 再获取快照
agent-browser snapshot -i --json
```

## 数据提取技巧

### 提取链接地址
```bash
agent-browser get attr @e5 "href" --json
```

### 提取表格数据
```bash
# 先获取表格有多少行
agent-browser get count "table tr" --json
# 然后逐行获取
agent-browser get text @e10 --json
```

### 提取图片 alt 文本
```bash
agent-browser get attr @e8 "alt" --json
```

## 调试技巧

### 1. 随时截图看效果
```bash
agent-browser screenshot debug.png
```

### 2. 获取完整页面文本
```bash
agent-browser get text --json  # 不带 @ref 获取全部文本
```

### 3. 查看当前 URL 和标题
```bash
agent-browser get url --json
agent-browser get title --json
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

1. **每次页面变化后都需要重新 snapshot** - refs 会改变！
2. **使用 --json 输出** - 便于解析结构化数据
3. **使用 -i 标志** - 只获取交互元素，减少噪音
4. **等待网络空闲** - 动态页面需要等待加载完成
5. **scroll 后要重新 snapshot** - 页面内容变了

## 前置条件

- agent-browser CLI 已安装: `npm install -g agent-browser`
- 浏览器已启动，或使用 `--cdp` / `--auto-connect` 连接已有浏览器
