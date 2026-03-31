# 文件上传与内容提取执行计划

## 1. 背景与目标

### 现状问题
| 问题               | 说明                                                    |
| ------------------ | ------------------------------------------------------- |
| 文件仅显示不处理   | ChatInterface.tsx 的 onDrop 只创建 blob URL，前端展示用 |
| 缺少内容提取       | 没有 PDF/Word/Excel 等文档的内容解析                    |
| Agent 无法访问文件 | Runner 没有注入文件路径信息给模型                       |

### 参考方案:
- **上传阶段**: 前端 → `/api/threads/{thread_id}/uploads` → 后端存储
- **存储结构**: 线程隔离目录 `backend/threads/{thread_id}/user-data/uploads/`
- **内容解析**: 使用 markitdown 库转换 PDF/Word/Excel 为文本
- **工具调用**: Agent 使用 `file_reader` skill 按需读取文件内容

### 本方案目标
1. 支持多格式文档（PDF/Word/Excel/TXT/图片）的上传与内容提取
2. 注入文件元信息到 Agent 上下文
3. 支持多轮对话中同一文件的持续访问
4. 与现有架构（agent-frame → runner）一致

---

## 2. 架构设计

### 2.1 整体流程

```
┌─────────────────────────────────────────────────────────────────────┐
│  前端 (ChatInterface.tsx)                                           │
│                                                                     │
│  1. onDrop → 读取文件为 ArrayBuffer                                 │
│  2. 调用 POST /api/xiaoqinglong/agent-frame/v1/runner/upload        │
│     - FormData 方式上传                                              │
│     - 参数: session_id, files[]                                      │
│  3. 后端返回: [{ name, virtual_path, size, extracted_text? }]       │
│  4. 保存 virtual_path 到 files state                                │
│  5. 调用 /runner/run 时带上 files 参数                              │
└─────────────────────────────────────────────────────────────────────┘
                                ↓
┌─────────────────────────────────────────────────────────────────────┐
│  Agent-Frame (runner_handler.go)                                    │
│                                                                     │
│  1. 新增 Upload 接口: 接收文件，存储到磁盘                           │
│  2. 调用 Runner 时，将 files 注入 messages                           │
│  3. 在用户消息前注入 <uploaded_files> 块                             │
│     <uploaded_files>                                                │
│     - report.pdf (125 KB)                                           │
│       Path: /mnt/uploads/{session_id}/report.pdf                   │
│     You can read this file using the file_reader skill.              │
│     </uploaded_files>                                               │
└─────────────────────────────────────────────────────────────────────┘
                                ↓
┌─────────────────────────────────────────────────────────────────────┐
│  Runner (dispatcher.go)                                             │
│                                                                     │
│  1. RunRequest 添加 Files 字段                                       │
│  2. 构建消息时，在用户消息前追加 uploaded_files 块                    │
│  3. 注册 file_reader skill（沙箱中已有）                             │
│     - 读取指定虚拟路径的文件内容                                     │
│     - 支持 .txt/.md 直接读取                                         │
│     - 支持 .pdf/.doc/.xlsx 调用内容提取服务                         │
└─────────────────────────────────────────────────────────────────────┘
```

### 2.2 文件存储结构

```
{app_data}/
└── uploads/
    └── {session_id}/
        ├── report.pdf          # 原始文件
        ├── report.txt          # 提取的文本内容（可选缓存）
        └── another.docx

# 虚拟路径（给 Agent 看到的）
/mnt/uploads/{session_id}/report.pdf
```

**生命周期**:
- **创建**: Upload API 时创建
- **持久化**: 跟随 session_id，同一会话多轮复用
- **清理**: Session 删除时级联删除整个 `uploads/{session_id}/` 目录

### 2.3 内容提取策略

**核心思路**: Agent 通过 Skill 调用 markitdown，而不是 Runner 直接调用

| 文件类型        | 处理方式                                               |
| --------------- | ------------------------------------------------------ |
| .txt, .md, .csv | 沙箱内直接读取                                         |
| .pdf            | Agent 调用 `file_reader` skill → 沙箱内执行 markitdown |
| .docx           | Agent 调用 `file_reader` skill → 沙箱内执行 markitdown |
| .xlsx           | Agent 调用 `file_reader` skill → 沙箱内执行 markitdown |
| .png, .jpg      | 保存原始文件，Agent 可用 skill + base64 读取           |

**架构对比**:

```
❌ 原方案（不推荐）
Runner → 调用 markitdown CLI → 需要在 Runner 容器装 markitdown

✅ 新方案（推荐）
Agent → 调用 file_reader skill → 沙箱内执行 markitdown → 已有的沙箱环境
```

**为什么更好**:
1. **无需 Runner 改动** - Runner 不需要调用 markitdown，保持简洁
2. **复用现有架构** - Skill 体系已经存在，markitdown 已在沙箱中
3. **K8s 友好** - 只需管理沙箱镜像，Runner 镜像无需额外依赖
4. **Skill 可复用** - 内容提取 skill 可以被任何 Agent 调用

---

## 3. 接口设计

### 3.1 文件上传接口

**请求**:
```
POST /api/xiaoqinglong/agent-frame/v1/runner/upload
Content-Type: multipart/form-data

session_id: "session_xxx"
files: [File1, File2, ...]
```

**响应**:
```json
{
    "files": [
        {
            "name": "report.pdf",
            "size": 128456,
            "type": "application/pdf",
            "virtual_path": "/mnt/uploads/session_xxx/report.pdf",
            "extracted_text_length": 0
        }
    ],
    "count": 1
}
```

### 3.2 Run 请求扩展

**ChatRunReq 扩展**:
```go
type ChatRunReq struct {
    AgentID   string `json:"agent_id"`
    UserID    string `json:"user_id"`
    SessionID string `json:"session_id"`
    Input     string `json:"input"`
    Files     []struct {
        Name        string `json:"name"`
        VirtualPath string `json:"virtual_path"`
        Size        int64  `json:"size"`
    } `json:"files,omitempty"`
    IsTest bool `json:"is_test"`
}
```

### 3.3 Runner RunRequest 扩展

```go
type RunRequest struct {
    // ... 现有字段
    Files []FileInfo `json:"files,omitempty"`
}

type FileInfo struct {
    Name        string `json:"name"`
    VirtualPath string `json:"virtual_path"`
    Size        int64  `json:"size"`
    Type        string `json:"type"` // mime type
}
```

---

## 4. 实现细节

### 4.1 Agent-Frame 改动

#### 4.1.1 Runner Handler 新增 Upload 接口

```go
// runner_file_handler.go

// Upload 文件上传
func (h *Handler) Upload(c *gin.Context) {
    // 1. 获取 session_id
    sessionID := c.PostForm("session_id")
    if sessionID == "" {
        c.JSON(400, gin.H{"error": "session_id is required"})
        return
    }

    // 2. 创建上传目录
    uploadDir := filepath.Join(os.Getenv("APP_DATA"), "uploads", sessionID)
    os.MkdirAll(uploadDir, 0755)

    // 3. 处理文件
    form, _ := c.MultipartForm()
    files := form.File["files"]

    var uploadedFiles []map[string]any
    for _, f := range files {
        // 保存文件
        dst := filepath.Join(uploadDir, f.Filename)
        f.Save(dst)

        uploadedFiles = append(uploadedFiles, map[string]any{
            "name":                 f.Filename,
            "size":                f.Size,
            "type":                f.Header.Get("Content-Type"),
            "virtual_path":        fmt.Sprintf("/mnt/uploads/%s/%s", sessionID, f.Filename),
            "extracted_text_length": 0,
        })
    }

    c.JSON(200, gin.H{
        "files": uploadedFiles,
        "count": len(uploadedFiles),
    })
}
```

#### 4.1.2 Run 接口注入 Files 信息

在 `runnerReq["messages"]` 构建时:

```go
// 如果有文件，在用户消息前追加 uploaded_files 块
if len(chatReq.Files) > 0 {
    filesBlock := buildFilesBlock(chatReq.Files)
    messages = append(messages, map[string]any{
        "role":    "system",
        "content": filesBlock,
    })
}

// 追加用户消息
messages = append(messages, map[string]any{
    "role":    "user",
    "content": chatReq.Input,
})

func buildFilesBlock(files []FileInfo) string {
    lines := []string{"<uploaded_files>", ""}
    for _, f := range files {
        lines = append(lines, fmt.Sprintf("- %s (%d bytes)", f.Name, f.Size))
        lines = append(lines, fmt.Sprintf("  Path: %s", f.VirtualPath))
        lines = append(lines, "")
    }
    lines = append(lines, "You can read these files using the file_reader skill.")
    lines = append(lines, "</uploaded_files>")
    return strings.Join(lines, "\n")
}
```

### 4.2 Runner 改动

**核心改动**: Runner 本身不需要调用 markitdown，内容提取由沙箱中的 Skill 负责。

#### 4.2.1 RunRequest 扩展

```go
type RunRequest struct {
    // ... 现有字段
    Files []FileInfo `json:"files,omitempty"`
}

type FileInfo struct {
    Name        string `json:"name"`
    VirtualPath string `json:"virtual_path"`
    Size        int64  `json:"size"`
    Type        string `json:"type"` // mime type
}
```

#### 4.2.2 Dispatcher 改动

dispatcher.go 只需要在构建消息时注入 `uploaded_files` 块：

```go
// dispatcher.go

// 如果有上传文件，在用户消息前追加 uploaded_files 块
if len(req.Files) > 0 {
    filesBlock := buildFilesBlock(req.Files)
    messages = append(messages, adk.Message{
        Role:    "system",
        Content: filesBlock,
    })
}

// 构建用户消息
messages = append(messages, schema.UserMessage(userInput))

func buildFilesBlock(files []FileInfo) string {
    lines := []string{"<uploaded_files>", ""}
    for _, f := range files {
        lines = append(lines, fmt.Sprintf("- %s (%d bytes)", f.Name, f.Size))
        lines = append(lines, fmt.Sprintf("  Path: %s", f.VirtualPath))
        lines = append(lines, "")
    }
    lines = append(lines, "You can read these files using the file_reader skill.")
    lines = append(lines, "</uploaded_files>")
    return strings.Join(lines, "\n")
}
```

#### 4.2.3 Skill 调用机制

Agent 需要读取文件时，调用 `file_reader` skill：

```
Agent: "请帮我总结这个PDF的内容"
         ↓
LLM 识别需要读取文件
         ↓
调用 file_reader skill
         ↓
Skill 在沙箱中执行: markitdown /mnt/uploads/{session}/report.pdf
         ↓
返回提取的文本给 Agent
```

---

## 5. 前端改动

### 5.1 ChatInterface.tsx

```tsx
// onDrop 改动
const onDrop = React.useCallback(async (acceptedFiles: File[]) => {
    if (!currentSession?.ulid && !activeConversationId) {
        toast.error("请先选择一个会话");
        return;
    }

    const sessionId = currentSession?.ulid || activeConversationId;

    // 构建 FormData 上传
    const formData = new FormData();
    formData.append("session_id", sessionId);
    acceptedFiles.forEach(file => {
        formData.append("files", file);
    });

    try {
        const response = await fetch(`${API_BASE}/runner/upload`, {
            method: "POST",
            body: formData,
        });
        const result = await response.json();

        // 保存返回的文件信息（virtual_path 用于后续请求）
        const uploadedFiles = result.files.map((f: any) => ({
            name: f.name,
            size: f.size,
            type: f.type,
            url: URL.createObjectURL(acceptedFiles.find(af => af.name === f.name)!),
            virtual_path: f.virtual_path,
        }));

        setFiles(prev => [...prev, ...uploadedFiles]);
        toast.success(`已上传 ${result.count} 个文件`);
    } catch (err) {
        console.error("Upload failed:", err);
        toast.error("文件上传失败");
    }
}, [currentSession, activeConversationId]);

// handleSend 改动 - 带上 files 信息
const handleSend = async () => {
    // ... 现有逻辑

    // 构建 files 参数（使用 virtual_path）
    const filesParam = files.map(f => ({
        name: f.name,
        virtual_path: f.virtual_path,
        size: f.size,
    }));

    const runResponse = await chatApi.runAgentStream({
        agent_id: activeAgent.ulid || activeAgent.id,
        user_id: CURRENT_USER_ID,
        session_id: sessionId || undefined,
        input: input,
        files: filesParam.length > 0 ? filesParam : undefined,
        is_test: false
    });
    // ...
};
```

### 5.2 types.ts

```ts
export interface FileInfo {
    name: string;
    size: number;
    type: string;
    url?: string;           // blob URL for display
    virtual_path?: string;  // 后端虚拟路径
}
```

---

## 6. 路由注册

### 6.1 Agent-Frame 路由

```go
// sys_router.go 或 runner_router.go

func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
    // ... existing routes

    // 文件上传
    r.POST("/runner/upload", h.Upload)
}
```

---

## 7. 依赖安装

### 7.1 内容提取 Skill

**复用沙箱中已有的 markitdown**，无需在 Runner 中安装任何依赖。

如果沙箱镜像中还没有 markitdown，在 Dockerfile 中安装：

```dockerfile
# 沙箱镜像 Dockerfile
RUN pip install markitdown
# 或
RUN apt-get update && apt-get install -y markitdown
```

### 7.2 Agent-Frame

无额外依赖。文件存储使用应用已有的 `APP_DATA` 目录。

---

## 8. 参考
前端文件是在：ChatInterface.tsx
后端的接口是在：agent-frame,遵循项目的设计，参考agent-frame/DDD.md
runner执行器是在：backend/runner
调用链：文件上传后，Agent 通过 `file_reader` skill 在沙箱中调用 markitdown 读取文件内容，支持多轮对话复用。