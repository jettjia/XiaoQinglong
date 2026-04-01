package compactors

import (
	"context"
	"errors"
	"fmt"

	"github.com/jettjia/XiaoQinglong/runner/contextcompressor/prompt"
)

// Message 消息结构
type Message struct {
	ID      string
	Type    string // "user", "assistant", "system", "tool"
	Role    string
	Content []ContentBlock
}

// ContentBlock 内容块
type ContentBlock struct {
	Type       string
	Text       string
	ToolUse    *ToolUseBlock
	ToolResult *ToolResultBlock
}

// ToolUseBlock 工具调用块
type ToolUseBlock struct {
	ID    string
	Name  string
	Input map[string]any
}

// ToolResultBlock 工具结果块
type ToolResultBlock struct {
	ToolUseID string
	Content   any
}

// CompactMetadata 压缩元数据
type CompactMetadata struct {
	CompactType          string
	PreCompactTokenCount int
	LastMessageID        string
}

// SystemMessage 系统消息
type SystemMessage struct {
	Message
	CompactMetadata *CompactMetadata
}

// CompactionResult 压缩结果
type CompactionResult struct {
	BoundaryMarker    *SystemMessage
	SummaryMessages   []Message
	MessagesToKeep    []Message
	PreCompactTokens  int
	PostCompactTokens int
}

// NewTextMessage 创建文本消息
func NewTextMessage(msgType, text string) *Message {
	return &Message{
		Type:    msgType,
		Content: []ContentBlock{{Type: "text", Text: text}},
	}
}

// NewSystemMessage 创建系统消息
func NewSystemMessage(content string) *Message {
	return &Message{
		Type:    "system",
		Content: []ContentBlock{{Type: "text", Text: content}},
	}
}

// ChatModel Chat 模型接口
type ChatModel interface {
	Generate(ctx context.Context, messages []Message) (string, error)
}

// Tokenizer Token 估算器接口
type Tokenizer interface {
	Estimate(text string) int
	EstimateMessages(messages []Message) int
}

// Config 压缩配置
type Config struct {
	Model               string
	MaxOutputTokens     int
	CompactBufferTokens int
	CustomInstructions  string
	SuppressFollowUp    bool
}

// FullCompacter 总结整个对话
type FullCompacter struct {
	chatModel  ChatModel
	tokenizer  Tokenizer
	config     *Config
}

// NewFullCompacter 创建完整压缩器
func NewFullCompacter(chatModel ChatModel, tokenizer Tokenizer, config *Config) *FullCompacter {
	if config == nil {
		config = &Config{MaxOutputTokens: 20000, CompactBufferTokens: 13000}
	}
	return &FullCompacter{
		chatModel:  chatModel,
		tokenizer:  tokenizer,
		config:     config,
	}
}

// Compact 实现压缩 - 总结整个对话
func (c *FullCompacter) Compact(ctx context.Context, messages []Message, opts ...func(*Config)) (*CompactionResult, error) {
	// 应用选项
	for _, opt := range opts {
		opt(c.config)
	}

	// 1. 前置检查：消息数量
	if len(messages) == 0 {
		return nil, errors.New("not enough messages to compact")
	}

	// 2. 预处理：剥离图片/文档
	cleaned := StripImagesFromMessages(messages)

	// 3. 估算压缩前 token
	preTokens := c.tokenizer.EstimateMessages(cleaned)

	// 4. 构建压缩提示
	compactPrompt := prompt.GetCompactPrompt(c.config.CustomInstructions)

	// 5. 构建消息列表
	summaryRequest := Message{
		Type:    "user",
		Content: []ContentBlock{{Type: "text", Text: compactPrompt}},
	}

	systemPrompt := Message{
		Type:    "system",
		Content: []ContentBlock{{Type: "text", Text: "You are a helpful AI assistant tasked with summarizing conversations."}},
	}

	// 合并消息
	allMessages := append([]Message{systemPrompt}, cleaned...)
	allMessages = append(allMessages, summaryRequest)

	// 6. 调用 LLM 生成摘要
	summaryResponse, err := c.chatModel.Generate(ctx, allMessages)
	if err != nil {
		return nil, fmt.Errorf("failed to generate summary: %w", err)
	}

	// 7. 格式化摘要
	summary := prompt.FormatCompactSummary(summaryResponse)

	// 8. 创建边界消息
	lastMsgID := ""
	if len(messages) > 0 {
		lastMsgID = messages[len(messages)-1].ID
	}
	boundary := CreateCompactBoundaryMessage("manual", preTokens, lastMsgID)

	// 9. 构建摘要消息
	summaryMsg := NewTextMessage("user", summary)

	// 10. 估算压缩后 token
	keptMessages := messages
	if len(messages) > 2 {
		keptMessages = messages[len(messages)-2:]
	}
	postTokens := c.tokenizer.EstimateMessages(keptMessages)

	return &CompactionResult{
		BoundaryMarker:    boundary,
		SummaryMessages:   []Message{*summaryMsg},
		MessagesToKeep:    keptMessages,
		PreCompactTokens:  preTokens,
		PostCompactTokens: postTokens,
	}, nil
}

// CreateCompactBoundaryMessage 创建压缩边界标记
func CreateCompactBoundaryMessage(compactType string, preTokens int, lastMsgID string) *SystemMessage {
	return &SystemMessage{
		Message: *NewSystemMessage("[earlier conversation compacted]"),
		CompactMetadata: &CompactMetadata{
			CompactType:          compactType,
			PreCompactTokenCount: preTokens,
			LastMessageID:        lastMsgID,
		},
	}
}