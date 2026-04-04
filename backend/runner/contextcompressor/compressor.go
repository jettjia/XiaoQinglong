package contextcompressor

import (
	"context"
	"errors"
	"fmt"

	"github.com/jettjia/XiaoQinglong/runner/contextcompressor/compactors"
)

// Compactor 压缩器（支持多种压缩策略）
type Compactor struct {
	chatModel        compactors.ChatModel
	tokenizer        *TokenizerAdapter
	config           *Config
	fullCompacter    *compactors.FullCompacter
	partialCompacter *compactors.PartialCompacter
	microCompacter   *compactors.MicroCompacter
}

// TokenizerAdapter 适配器，将 contextcompressor.Tokenizer 适配为 compactors.Tokenizer
type TokenizerAdapter struct {
	inner Tokenizer
}

func (a *TokenizerAdapter) Estimate(text string) int {
	return a.inner.Estimate(text)
}

func (a *TokenizerAdapter) EstimateMessages(messages []compactors.Message) int {
	// 转换消息格式
	msgs := make([]Message, len(messages))
	for i, m := range messages {
		content := make([]ContentBlock, len(m.Content))
		for j, block := range m.Content {
			content[j] = ContentBlock{Type: block.Type, Text: block.Text}
		}
		msgs[i] = Message{ID: m.ID, Type: MessageType(m.Type), Role: m.Role, Content: content}
	}
	return a.inner.EstimateMessages(msgs)
}

// NewCompactor 创建压缩器
func NewCompactor(chatModel compactors.ChatModel, tokenizer Tokenizer, opts ...Option) *Compactor {
	cfg := &Config{
		Model:               "claude-sonnet-4-20250514",
		MaxOutputTokens:     DefaultMaxOutputTokens,
		CompactBufferTokens: DefaultCompactBufferTokens,
	}
	for _, o := range opts {
		o(cfg)
	}

	cc := &compactors.Config{
		Model:               cfg.Model,
		MaxOutputTokens:     cfg.MaxOutputTokens,
		CompactBufferTokens: cfg.CompactBufferTokens,
		CustomThreshold:     cfg.CustomThreshold,
		CustomInstructions:  cfg.CustomInstructions,
		SuppressFollowUp:    cfg.SuppressFollowUp,
	}

	adapter := &TokenizerAdapter{inner: tokenizer}

	return &Compactor{
		chatModel:        chatModel,
		tokenizer:        adapter,
		config:           cfg,
		fullCompacter:    compactors.NewFullCompacter(chatModel, adapter, cc),
		partialCompacter: compactors.NewPartialCompacter(chatModel, adapter, cc),
		microCompacter:   compactors.NewMicroCompacter(adapter, cc),
	}
}

// ShouldCompact 判断是否需要压缩
func (c *Compactor) ShouldCompact(messages []Message, threshold int) bool {
	tokens := c.tokenizer.EstimateMessages(toCompactorsMessages(messages))
	return tokens >= threshold
}

// toCompactorsMessages 转换为 compactors.Message
func toCompactorsMessages(messages []Message) []compactors.Message {
	result := make([]compactors.Message, len(messages))
	for i, m := range messages {
		content := make([]compactors.ContentBlock, len(m.Content))
		for j, block := range m.Content {
			content[j] = compactors.ContentBlock{Type: block.Type, Text: block.Text}
			if block.ToolUse != nil {
				content[j].ToolUse = &compactors.ToolUseBlock{ID: block.ToolUse.ID, Name: block.ToolUse.Name, Input: block.ToolUse.Input}
			}
			if block.ToolResult != nil {
				content[j].ToolResult = &compactors.ToolResultBlock{ToolUseID: block.ToolResult.ToolUseID, Content: block.ToolResult.Content}
			}
		}
		result[i] = compactors.Message{ID: m.ID, Type: string(m.Type), Role: m.Role, Content: content}
	}
	return result
}

// fromCompactorsResult 转换 compactors.CompactionResult
func fromCompactorsResult(result *compactors.CompactionResult) *CompactionResult {
	if result == nil {
		return nil
	}

	var boundaryMarker *SystemMessage
	if result.BoundaryMarker != nil {
		boundaryMarker = &SystemMessage{
			Message: Message{ID: result.BoundaryMarker.ID, Type: MessageType(result.BoundaryMarker.Type), Role: result.BoundaryMarker.Role},
			CompactMetadata: &CompactMetadata{
				CompactType:          result.BoundaryMarker.CompactMetadata.CompactType,
				PreCompactTokenCount: result.BoundaryMarker.CompactMetadata.PreCompactTokenCount,
				LastMessageID:        result.BoundaryMarker.CompactMetadata.LastMessageID,
			},
		}
	}

	summaryMessages := make([]Message, len(result.SummaryMessages))
	for i, m := range result.SummaryMessages {
		summaryMessages[i] = Message{ID: m.ID, Type: MessageType(m.Type), Role: m.Role}
	}

	messagesToKeep := make([]Message, len(result.MessagesToKeep))
	for i, m := range result.MessagesToKeep {
		content := make([]ContentBlock, len(m.Content))
		for j, block := range m.Content {
			content[j] = ContentBlock{Type: block.Type, Text: block.Text}
		}
		messagesToKeep[i] = Message{ID: m.ID, Type: MessageType(m.Type), Role: m.Role, Content: content}
	}

	return &CompactionResult{
		BoundaryMarker:    boundaryMarker,
		SummaryMessages:   summaryMessages,
		MessagesToKeep:    messagesToKeep,
		PreCompactTokens:  result.PreCompactTokens,
		PostCompactTokens: result.PostCompactTokens,
	}
}

// Compact 自动选择合适的压缩策略
func (c *Compactor) Compact(ctx context.Context, messages []Message, opts ...Option) (*CompactionResult, error) {
	for _, opt := range opts {
		opt(c.config)
	}

	msgCount := len(messages)
	tokenCount := c.tokenizer.EstimateMessages(toCompactorsMessages(messages))
	threshold := c.getThreshold()

	var result *compactors.CompactionResult
	var err error

	switch {
	case msgCount <= 10 && tokenCount < 50000:
		// 使用 ShouldCompactMicro 判断是否真正需要微压缩
		if compactors.ShouldCompactMicro(toCompactorsMessages(messages), c.tokenizer, threshold/4) {
			result, err = c.microCompacter.Compact(ctx, toCompactorsMessages(messages))
		} else {
			// 不需要压缩，直接返回
			return &CompactionResult{
				MessagesToKeep:    messages,
				PreCompactTokens:  tokenCount,
				PostCompactTokens: tokenCount,
			}, nil
		}
	case msgCount <= 50 && tokenCount < 100000:
		result, err = c.partialCompacter.Compact(ctx, toCompactorsMessages(messages))
	default:
		result, err = c.fullCompacter.Compact(ctx, toCompactorsMessages(messages))
	}

	if err != nil {
		return nil, err
	}

	return fromCompactorsResult(result), nil
}

// getThreshold 获取压缩阈值
func (c *Compactor) getThreshold() int {
	if c.config.CustomThreshold > 0 {
		return c.config.CustomThreshold
	}
	// 默认阈值
	return 100000
}

// FullCompact 完整压缩
func (c *Compactor) FullCompact(ctx context.Context, messages []Message, opts ...Option) (*CompactionResult, error) {
	result, err := c.fullCompacter.Compact(ctx, toCompactorsMessages(messages))
	if err != nil {
		return nil, err
	}
	return fromCompactorsResult(result), nil
}

// PartialCompact 部分压缩
func (c *Compactor) PartialCompact(ctx context.Context, messages []Message, opts ...Option) (*CompactionResult, error) {
	result, err := c.partialCompacter.Compact(ctx, toCompactorsMessages(messages))
	if err != nil {
		return nil, err
	}
	return fromCompactorsResult(result), nil
}

// MicroCompact 微压缩
func (c *Compactor) MicroCompact(ctx context.Context, messages []Message, opts ...Option) (*CompactionResult, error) {
	result, err := c.microCompacter.Compact(ctx, toCompactorsMessages(messages))
	if err != nil {
		return nil, err
	}
	return fromCompactorsResult(result), nil
}

// GetConfig 获取配置
func (c *Compactor) GetConfig() *Config {
	return c.config
}

// GetTokenCount 估算消息 token 数
func (c *Compactor) GetTokenCount(messages []Message) int {
	return c.tokenizer.EstimateMessages(toCompactorsMessages(messages))
}

// BuildPostCompactMessages 构建压缩后的消息列表
func BuildPostCompactMessages(result *CompactionResult) []Message {
	if result == nil {
		return nil
	}

	var messages []Message

	if result.BoundaryMarker != nil {
		messages = append(messages, result.BoundaryMarker.Message)
	}

	if len(result.SummaryMessages) > 0 {
		messages = append(messages, result.SummaryMessages...)
	}

	if len(result.MessagesToKeep) > 0 {
		messages = append(messages, result.MessagesToKeep...)
	}

	return messages
}

// ValidateCompactionResult 验证压缩结果
func ValidateCompactionResult(result *CompactionResult) error {
	if result == nil {
		return errors.New("compaction result is nil")
	}
	if result.PostCompactTokens > result.PreCompactTokens {
		return fmt.Errorf("post compact tokens (%d) should be less than pre compact tokens (%d)",
			result.PostCompactTokens, result.PreCompactTokens)
	}
	return nil
}