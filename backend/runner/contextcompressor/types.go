package contextcompressor

import (
	"context"
)

// Token 预算常量
const (
	DefaultCompactBufferTokens    = 13_000
	DefaultWarningBufferTokens    = 20_000
	DefaultMaxOutputTokens        = 20_000
	DefaultPostCompactTokenBudget = 50_000
	DefaultMaxTokensPerFile       = 5_000
	DefaultMaxTokensPerSkill      = 5_000
)

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
	Type       string           `json:"type"`
	Text       string           `json:"text,omitempty"`
	Image      *ImageContent    `json:"image,omitempty"`
	ToolUse    *ToolUseBlock    `json:"tool_use,omitempty"`
	ToolResult *ToolResultBlock `json:"tool_result,omitempty"`
	Document   *DocumentContent `json:"document,omitempty"`
}

// ImageContent 图片内容
type ImageContent struct {
	Type     string `json:"type"`
	Source   string `json:"source,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
}

// ToolUseBlock 工具调用块
type ToolUseBlock struct {
	ID    string         `json:"id,omitempty"`
	Type  string         `json:"type,omitempty"`
	Name  string         `json:"name,omitempty"`
	Input map[string]any `json:"input,omitempty"`
}

// ToolResultBlock 工具结果块
type ToolResultBlock struct {
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   any    `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

// DocumentContent 文档内容
type DocumentContent struct {
	Type     string `json:"type,omitempty"`
	Source   string `json:"source,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
}

// Message 消息结构
type Message struct {
	ID       string          `json:"id"`
	Type     MessageType     `json:"type"`
	Role     string          `json:"role,omitempty"`
	Content  []ContentBlock  `json:"content"`
	Metadata map[string]any  `json:"metadata,omitempty"`
}

// SystemMessage 系统消息
type SystemMessage struct {
	Message
	CompactMetadata *CompactMetadata `json:"compact_metadata,omitempty"`
}

// CompactMetadata 压缩元数据
type CompactMetadata struct {
	CompactType            string   `json:"compact_type,omitempty"`
	PreCompactTokenCount  int      `json:"pre_compact_token_count,omitempty"`
	LastMessageID         string   `json:"last_message_id,omitempty"`
	UserContext           string   `json:"user_context,omitempty"`
	MessagesSummarized    int      `json:"messages_summarized,omitempty"`
}

// CompactionResult 压缩结果
type CompactionResult struct {
	BoundaryMarker    *SystemMessage `json:"boundary_marker,omitempty"`
	SummaryMessages   []Message      `json:"summary_messages,omitempty"`
	MessagesToKeep    []Message      `json:"messages_to_keep,omitempty"`
	PreCompactTokens  int            `json:"pre_compact_tokens,omitempty"`
	PostCompactTokens int            `json:"post_compact_tokens,omitempty"`
}

// MessageGroup 按 API 轮次分组
type MessageGroup struct {
	AssistantID string
	Messages    []Message
}

// Config 压缩配置
type Config struct {
	Model               string
	MaxOutputTokens     int
	CompactBufferTokens int
	CustomInstructions  string
	SuppressFollowUp    bool
}

// Option 压缩选项函数
type Option func(*Config)

// WithCustomInstructions 设置自定义指令
func WithCustomInstructions(s string) Option {
	return func(c *Config) { c.CustomInstructions = s }
}

// WithMaxOutputTokens 设置最大输出 token
func WithMaxOutputTokens(n int) Option {
	return func(c *Config) { c.MaxOutputTokens = n }
}

// WithCompactBufferTokens 设置压缩缓冲 token
func WithCompactBufferTokens(n int) Option {
	return func(c *Config) { c.CompactBufferTokens = n }
}

// WithSuppressFollowUp 设置是否抑制后续问题
func WithSuppressFollowUp(b bool) Option {
	return func(c *Config) { c.SuppressFollowUp = b }
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		MaxOutputTokens:     DefaultMaxOutputTokens,
		CompactBufferTokens: DefaultCompactBufferTokens,
	}
}

// Tokenizer Token 估算器接口
type Tokenizer interface {
	Estimate(text string) int
	EstimateMessages(messages []Message) int
}

// MessageGrouper 消息分组器接口
type MessageGrouper interface {
	Group(messages []Message) []MessageGroup
}

// Compressor 压缩器接口
type Compressor interface {
	Compact(ctx context.Context, messages []Message, opts ...Option) (*CompactionResult, error)
}

// GetCompactThreshold 获取压缩阈值
func GetCompactThreshold(model string, contextWindow int) int {
	if contextWindow == 0 {
		contextWindow = 150_000
	}
	return contextWindow - DefaultCompactBufferTokens
}

// NewTextMessage 创建文本消息
func NewTextMessage(msgType MessageType, text string) *Message {
	return &Message{
		Type:    msgType,
		Content: []ContentBlock{{Type: "text", Text: text}},
	}
}

// NewSystemMessage 创建系统消息
func NewSystemMessage(content string) *Message {
	return &Message{
		Type:    MessageTypeSystem,
		Content: []ContentBlock{{Type: "text", Text: content}},
	}
}