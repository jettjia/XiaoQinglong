# 上下文压缩方案 (Go + Eino)

基于 Claude Code 压缩逻辑的通用消息压缩方案。

---

## 一、目录结构

```
contextcompressor/
├── go.mod
├── message.go          # 消息类型定义
├── tokenizer.go        # Token 估算器
├── grouper.go          # 消息分组器
├── compressor.go      # 核心压缩接口
├── compactors/
│   ├── full.go         # Full Compacter - 总结整个对话
│   ├── partial.go     # Partial Compacter - 只总结最近消息
│   └── micro.go       # Micro Compacter - 工具结果压缩
├── prompt/
│   └── templates.go    # 压缩提示词模板
├── cleanup/
│   └── cleanup.go      # 压缩后清理函数
├── eino/
│   └── eino.go         # eino 框架集成
└── examples/
    └── basic/main.go   # 使用示例
```

---

## 二、核心类型定义 (message.go)

```go
// MessageType 消息类型
type MessageType string

const (
    MessageTypeUser      MessageType = "user"
    MessageTypeAssistant MessageType = "assistant"
    MessageTypeSystem    MessageType = "system"
    MessageTypeTool      MessageType = "tool"
)

// ContentBlock 内容块
type ContentBlock struct {
    Type      string `json:"type"` // text, image, tool_use, tool_result, document
    Text      string `json:"text,omitempty"`
    Image     *ImageContent    `json:"image,omitempty"`
    ToolUse   *ToolUseBlock    `json:"tool_use,omitempty"`
    ToolResult *ToolResultBlock `json:"tool_result,omitempty"`
    Document  *DocumentContent `json:"document,omitempty"`
}

// Message 消息结构
type Message struct {
    ID       string            `json:"id"`
    Type     MessageType       `json:"type"`
    Role     string            `json:"role,omitempty"`
    Content  []ContentBlock    `json:"content"`
    Metadata map[string]any    `json:"metadata,omitempty"`
}

// SystemMessage 系统消息（压缩边界标记）
type SystemMessage struct {
    Message
    CompactMetadata *CompactMetadata `json:"compact_metadata,omitempty"`
}

// CompactMetadata 压缩元数据
type CompactMetadata struct {
    CompactType  string `json:"compact_type,omitempty"`  // "auto" | "manual"
    PreCompactTokenCount int `json:"pre_compact_token_count,omitempty"`
    LastMessageID       string `json:"last_message_id,omitempty"`
}

// CompactionResult 压缩结果
type CompactionResult struct {
    BoundaryMarker   *SystemMessage  // 压缩边界标记
    SummaryMessages  []Message      // 摘要消息
    MessagesToKeep   []Message      // 保留的消息
    PreCompactTokens int            // 压缩前 token 数
    PostCompactTokens int           // 压缩后 token 数
}
```

---

## 三、核心接口 (compressor.go)

```go
// Tokenizer Token 估算器
type Tokenizer interface {
    Estimate(text string) int                          // 轻量级估算
    EstimateMessages(messages []Message) int           // 估算消息列表
    EstimateWithModel(ctx context.Context, messages []Message, model string) (int, error) // LLM API 精确计算
}

// MessageGrouper 消息分组器
type MessageGrouper interface {
    Group(messages []Message) []MessageGroup
}

// MessageGroup 按 API 轮次分组
type MessageGroup struct {
    AssistantID string    // assistant 消息 ID
    Messages    []Message // 该轮次的所有消息
}

// Compressor 压缩器接口
type Compressor interface {
    Compact(ctx context.Context, messages []Message, opts ...Option) (*CompactionResult, error)
}
```

---

## 四、Token 预算常量 (options.go)

```go
const (
    DefaultCompactBufferTokens    = 13_000  // 自动压缩缓冲
    DefaultWarningBufferTokens    = 20_000  // 警告阈值
    DefaultMaxOutputTokens        = 20_000  // 摘要最大输出
    DefaultPostCompactTokenBudget = 50_000  // 压缩后保留 token
    DefaultMaxTokensPerFile       = 5_000   // 文件结果最大 token
    DefaultMaxTokensPerSkill      = 5_000   // Skill 结果最大 token
)

// Config 压缩配置
type Config struct {
    Model                 string
    MaxOutputTokens       int
    CompactBufferTokens   int
    CustomInstructions    string
    SuppressFollowUp      bool
}
```

---

## 五、消息分组器实现 (grouper.go)

**核心逻辑**：按 API 轮次分组，当新的 assistant 消息开始时创建新分组。

```go
// GroupMessagesByApiRound 按 API 轮次分组消息
// 分组边界：新的 assistant 消息开始（不同 message.id）
func GroupMessagesByApiRound(messages []Message) []MessageGroup {
    var groups []MessageGroup
    var current []Message
    var lastAssistantID string

    for _, msg := range messages {
        if msg.Type == MessageTypeAssistant &&
           msg.ID != lastAssistantID &&
           len(current) > 0 {
            groups = append(groups, MessageGroup{
                AssistantID: lastAssistantID,
                Messages:    current,
            })
            current = []Message{msg}
        } else {
            current = append(current, msg)
        }
        if msg.Type == MessageTypeAssistant {
            lastAssistantID = msg.ID
        }
    }

    if len(current) > 0 {
        groups = append(groups, MessageGroup{
            AssistantID: lastAssistantID,
            Messages:    current,
        })
    }
    return groups
}
```

---

## 六、清理函数 (cleanup/cleanup.go)

### 6.1 剥离图片和文档

```go
// StripImagesFromMessages 剥离图片，替换为 [image] 标记
// 参考: src/services/compact/compact.ts:145-200
func StripImagesFromMessages(messages []Message) []Message {
    result := make([]Message, len(messages))
    for i, msg := range messages {
        if msg.Type != MessageTypeUser {
            result[i] = msg
            continue
        }

        newContent := make([]ContentBlock, 0)
        for _, block := range msg.Content {
            switch block.Type {
            case "image":
                newContent = append(newContent, ContentBlock{Type: "text", Text: "[image]"})
            case "document":
                newContent = append(newContent, ContentBlock{Type: "text", Text: "[document]"})
            case "tool_result":
                // 处理 tool_result 中的图片/文档
                newBlock := stripMediaFromToolResult(block.ToolResult)
                newContent = append(newContent, newBlock)
            default:
                newContent = append(newContent, block)
            }
        }
        result[i] = msg
        result[i].Content = newContent
    }
    return result
}
```

### 6.2 Micro Compact - 工具结果压缩

```go
// CompactToolResults 压缩工具结果中的大内容
// 参考: src/services/compact/microCompact.ts
func CompactToolResults(messages []Message, maxTokens int) []Message {
    // 1. 图片/文档 → [image]/[document] 标记
    // 2. 长文本 → 截断 + [truncated] 标记
    // 3. 保留：工具名、文件路径、错误信息
}
```

---

## 七、压缩提示词模板 (prompt/templates.go)

**参考**: src/services/compact/prompt.ts

```go
// 核心提示词模板
const BASE_COMPACT_PROMPT = `Your task is to create a detailed summary of the conversation so far...

Your summary should include the following sections:
1. Primary Request and Intent
2. Key Technical Concepts
3. Files and Code Sections
4. Errors and fixes
5. Problem Solving
6. All user messages
7. Pending Tasks
8. Current Work
9. Optional Next Step

CRITICAL: Respond with TEXT ONLY. Do NOT call any tools.
`

// 禁止工具调用的前缀
const NO_TOOLS_PREAMBLE = `CRITICAL: Respond with TEXT ONLY. Do NOT call any tools.
- Do NOT use Read, Bash, Grep, Glob, Edit, Write, or ANY other tool.
- Tool calls will be REJECTED and will waste your only turn.
- Your entire response must be plain text: an <analysis> block followed by a <summary> block.
`

// FormatCompactSummary 格式化 LLM 输出的摘要
// 参考: src/services/compact/prompt.ts:311-335
func FormatCompactSummary(summary string) string {
    // 1. 剥离 <analysis> 标签（草稿，不保留）
    summary = stripAnalysisTags(summary)
    // 2. 提取 <summary> 内容
    summary = extractSummaryContent(summary)
    // 3. 清理多余空白
    return strings.TrimSpace(summary)
}

// GetCompactUserSummaryMessage 构建压缩后的摘要消息
// 参考: src/services/compact/prompt.ts:337-374
func GetCompactUserSummaryMessage(summary string, suppressFollowUp bool) string {
    // 构建类似格式:
    // "This session is being continued from a previous conversation...
    //  [summary content]
    //  Continue the conversation from where it left off..."
}
```

---

## 八、Full Compacter 实现 (compactors/full.go)

```go
// FullCompacter 总结整个对话
type FullCompacter struct {
    chatModel  chatmodel.ChatModel  // eino chat model
    tokenizer  Tokenizer
    config     *Config
}

// Compact 实现压缩
// 参考: src/services/compact/compact.ts:387-710
func (c *FullCompacter) Compact(ctx context.Context, messages []Message, opts ...Option) (*CompactionResult, error) {
    // 1. 前置检查：消息数量
    if len(messages) == 0 {
        return nil, errors.New("not enough messages to compact")
    }

    // 2. 预处理：剥离图片/文档
    cleaned := StripImagesFromMessages(messages)

    // 3. 估算压缩前 token
    preTokens := c.tokenizer.EstimateMessages(cleaned)

    // 4. 构建压缩提示
    prompt := GetCompactPrompt(c.config.CustomInstructions)

    // 5. 调用 LLM 生成摘要
    summaryResponse, err := c.chatModel.Generate(ctx, []Message{
        {Type: MessageTypeUser, Content: []ContentBlock{{Type: "text", Text: prompt}}},
        // ... 添加要总结的消息
    })
    if err != nil {
        return nil, err
    }

    // 6. 格式化摘要
    summary := FormatCompactSummary(GetResponseText(summaryResponse))

    // 7. 创建边界消息
    boundary := CreateCompactBoundaryMessage("manual", preTokens, messages[len(messages)-1].ID)

    // 8. 构建结果
    summaryMsg := NewTextMessage(MessageTypeUser, summary)
    return &CompactionResult{
        BoundaryMarker:   boundary,
        SummaryMessages:  []Message{*summaryMsg},
        MessagesToKeep:   messages[len(messages)-2:], // 保留最后 1-2 条
        PreCompactTokens: preTokens,
        PostCompactTokens: c.tokenizer.EstimateMessages(messages[len(messages)-2:]),
    }, nil
}

// CreateCompactBoundaryMessage 创建压缩边界标记
func CreateCompactBoundaryMessage(compactType string, preTokens int, lastMsgID string) *SystemMessage {
    return &SystemMessage{
        Message: *NewTextMessage(MessageTypeSystem, "[earlier conversation compacted]"),
        CompactMetadata: &CompactMetadata{
            CompactType:           compactType,
            PreCompactTokenCount: preTokens,
            LastMessageID:         lastMsgID,
        },
    }
}
```

---

## 九、Partial Compacter 实现 (compactors/partial.go)

```go
// PartialCompacter 只总结最近的消息
// 参考: src/services/compact/compact.ts 中 partial compact 逻辑
type PartialCompacter struct {
    chatModel chatmodel.ChatModel
    tokenizer Tokenizer
    config    *Config
}

// Compact 总结最近消息，保留早期上下文
// direction: "from" (从早期保留点到现在) | "up_to" (从开始到现在)
func (c *PartialCompacter) Compact(ctx context.Context, messages []Message, opts ...Option) (*CompactionResult, error) {
    // 1. 确定保留点（早期消息）和总结点
    preserveCount := getPreserveCount(len(messages))
    toSummarize := messages[:len(messages)-preserveCount]
    toKeep := messages[len(messages)-preserveCount:]

    // 2. 剥离图片
    cleaned := StripImagesFromMessages(toSummarize)

    // 3. 生成摘要
    prompt := GetPartialCompactPrompt(c.config.CustomInstructions, direction)
    summaryResponse, err := c.chatModel.Generate(ctx, cleaned, prompt)

    // 4. 构建结果
    // ...
}
```

---

## 十、eino 集成 (eino/eino.go)

```go
package eino

import (
    "context"
    "github.com/yuininks/gopher-ai/eino"
    "github.com/yuininks/gopher-ai/eino/chatmodel"
)

// Compactor eino 压缩器
type Compactor struct {
    chatModel chatmodel.ChatModel
    tokenizer Tokenizer
    config    *Config
    compactors []Compactor // 支持多种压缩策略
}

// Option 压缩选项
type Option func(*Config)

// WithCustomInstructions 设置自定义指令
func WithCustomInstructions(s string) Option {
    return func(c *Config) { c.CustomInstructions = s }
}

// WithMaxOutputTokens 设置最大输出 token
func WithMaxOutputTokens(n int) Option {
    return func(c *Config) { c.MaxOutputTokens = n }
}

// NewCompactor 创建 eino 压缩器
func NewCompactor(chatModel chatmodel.ChatModel, tokenizer Tokenizer, opts ...Option) *Compactor {
    cfg := &Config{
        Model:       chatModel.ModelName(),
        MaxOutputTokens: 20000,
    }
    for _, o := range opts {
        o(cfg)
    }
    return &Compactor{
        chatModel:  chatModel,
        tokenizer:  tokenizer,
        config:     cfg,
        compactors: []Compactor{
            &FullCompacter{chatModel, tokenizer, cfg},
            &PartialCompacter{chatModel, tokenizer, cfg},
            &MicroCompacter{tokenizer, cfg},
        },
    }
}

// ShouldCompact 判断是否需要压缩
func (c *Compactor) ShouldCompact(messages []Message, threshold int) bool {
    tokens := c.tokenizer.EstimateMessages(messages)
    return tokens >= threshold
}

// Compact 自动选择合适的压缩策略
func (c *Compactor) Compact(ctx context.Context, messages []Message, opts ...Option) (*CompactionResult, error) {
    // 根据消息特征选择压缩策略
    // ...
}
```

---

## 十一、自动压缩触发 (autoCompact.go)

```go
// 参考: src/services/compact/autoCompact.ts

const (
    AUTOCOMPACT_BUFFER_TOKENS = 13_000
    WARNING_THRESHOLD_BUFFER_TOKENS = 20_000
)

// ShouldAutoCompact 判断是否应触发自动压缩
func ShouldAutoCompact(messages []Message, model string, tokenizer Tokenizer) bool {
    tokens := tokenizer.EstimateMessages(messages)
    threshold := getAutoCompactThreshold(model)
    return tokens >= threshold
}

// getAutoCompactThreshold 获取自动压缩阈值
func getAutoCompactThreshold(model string) int {
    // contextWindow := getContextWindowForModel(model)
    // return contextWindow - AUTOCOMPACT_BUFFER_TOKENS
    return 150_000 - AUTOCOMPACT_BUFFER_TOKENS
}

// AutoCompactIfNeeded 如果需要则自动压缩
func (c *Compactor) AutoCompactIfNeeded(ctx context.Context, messages []Message) (*CompactionResult, bool, error) {
    if !ShouldAutoCompact(messages, c.config.Model, c.tokenizer) {
        return nil, false, nil
    }
    result, err := c.Compact(ctx, messages)
    return result, true, err
}
```

---

## 十二、使用示例

```go
package main

import (
    "context"
    "github.com/yuininks/gopher-ai/eino"
    "github.com/yuininks/gopher-ai/contextcompressor/eino"
)

func main() {
    // 1. 创建 eino chat model
    chatModel := eino.NewOpenAIChatModel(&eino.OpenAIConfig{
        APIKey: "your-api-key",
        Model:  "claude-sonnet-4-20250514",
    })

    // 2. 创建 tokenizer
    tokenizer := contextcompressor.NewDefaultTokenizer(4.0)

    // 3. 创建压缩器
    compactor := contextcompressor_eino.NewCompactor(
        chatModel,
        tokenizer,
        contextcompressor.WithMaxOutputTokens(20000),
    )

    // 4. 检查是否需要压缩
    messages := loadMessages()
    if compactor.ShouldCompact(messages, 130000) {
        // 5. 执行压缩
        result, err := compactor.Compact(context.Background(), messages)
        if err != nil {
            log.Fatal(err)
        }
        // 6. 使用压缩结果
        newMessages := append([]Message{*result.BoundaryMarker}, result.SummaryMessages...)
        newMessages = append(newMessages, result.MessagesToKeep...)
    }
}
```

---

## 十三、关键参考文件

| Claude Code 文件 | 对应实现 | 说明 |
|----------------|---------|------|
| `src/services/compact/compact.ts` | compactors/full.go | 主压缩引擎 |
| `src/services/compact/prompt.ts` | prompt/templates.go | 提示词模板 |
| `src/services/compact/grouping.ts` | grouper.go | 消息分组 |
| `src/services/compact/microCompact.ts` | cleanup/cleanup.go | 工具结果压缩 |
| `src/services/compact/autoCompact.ts` | autoCompact.go | 自动压缩触发 |
| `src/services/compact/compact.ts:145` | cleanup/cleanup.go | 图片剥离 |

---

## 十四、实现优先级

1. **P0**: message.go, tokenizer.go, grouper.go, cleanup.go
2. **P1**: prompt/templates.go, compactors/full.go
3. **P2**: compactors/partial.go, compactors/micro.go
4. **P3**: eino/eino.go, autoCompact.go
5. **P4**: examples, README
