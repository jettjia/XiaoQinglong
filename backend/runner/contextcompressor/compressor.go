package contextcompressor

import (
	"context"
	"errors"
	"fmt"

	"github.com/jettjia/XiaoQinglong/runner/contextcompressor/compactors"
	"github.com/jettjia/XiaoQinglong/runner/pkg/logger"
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
// 参考 Hermes-agent 的压缩算法：
// 1. PruneToolResults - 裁剪旧工具结果的详细内容
// 2. ProtectHeadMessages - 保护头部消息（系统 + 前几个对话轮次）
// 3. ProtectTailMessages - 保护尾部消息（最近的上下文）
// 4. 对中间部分进行摘要压缩
func (c *Compactor) Compact(ctx context.Context, messages []Message, opts ...Option) (*CompactionResult, error) {
	for _, opt := range opts {
		opt(c.config)
	}

	msgCount := len(messages)
	tokenCount := c.tokenizer.EstimateMessages(toCompactorsMessages(messages))
	threshold := c.getThreshold()

	// Hermes-agent 风格预处理：先裁剪旧工具结果
	// 这可以显著减少 token 数量，避免过早触发完整压缩
	ccMessages := c.preprocessMessages(messages)

	// 重新计算 token 数
	preprocessedTokens := c.tokenizer.EstimateMessages(ccMessages)
	logger.GetRunnerLogger().Infof("[Compactor] After preprocessing: %d -> %d tokens", tokenCount, preprocessedTokens)

	var result *compactors.CompactionResult
	var err error

	// 根据消息数量和 token 数选择压缩策略
	switch {
	case msgCount <= 10 && preprocessedTokens < 50000:
		// 使用 ShouldCompactMicro 判断是否真正需要微压缩
		if compactors.ShouldCompactMicro(ccMessages, c.tokenizer, threshold/4) {
			result, err = c.microCompacter.Compact(ctx, ccMessages)
		} else {
			// 不需要压缩，直接返回
			return &CompactionResult{
				MessagesToKeep:    fromCompactorsMessages(ccMessages),
				PreCompactTokens:  preprocessedTokens,
				PostCompactTokens: preprocessedTokens,
			}, nil
		}
	case msgCount <= 50 && preprocessedTokens < 100000:
		result, err = c.partialCompacter.Compact(ctx, ccMessages)
	default:
		result, err = c.fullCompacter.Compact(ctx, ccMessages)
	}

	if err != nil {
		return nil, err
	}

	return fromCompactorsResult(result), nil
}

// preprocessMessages 预处理消息，应用 Hermes-agent 风格的裁剪
// 1. 裁剪旧工具结果
// 2. 保护头部和尾部消息
func (c *Compactor) preprocessMessages(messages []Message) []compactors.Message {
	if len(messages) == 0 {
		return toCompactorsMessages(messages)
	}

	// Step 1: 裁剪旧工具结果（保留摘要，只保留最后 N 个字符）
	maxResultLen := 500
	preprocessed := compactors.PruneToolResults(toCompactorsMessages(messages), maxResultLen)

	// Step 2: 保护头部消息（系统消息 + 前 3 条）
	headMsgs, rest := compactors.ProtectHeadMessages(preprocessed, 3)

	// Step 3: 保护尾部消息（最近 20000 tokens）
	_, tailMsgs := compactors.ProtectTailMessages(rest, c.tokenizer, 20000)

	// 合并：头部 + 尾部（中间部分已被裁剪）
	var result []compactors.Message
	if headMsgs != nil {
		result = append(result, headMsgs...)
	}
	if tailMsgs != nil {
		result = append(result, tailMsgs...)
	}

	// 如果结果为空或几乎没有变化，返回原始预处理结果
	if len(result) == 0 || len(result) >= len(preprocessed)/2 {
		return preprocessed
	}

	logger.GetRunnerLogger().Infof("[Compactor] Preprocessing reduced messages: %d -> %d", len(messages), len(result))
	return result
}

// fromCompactorsMessages converts []compactors.Message to []Message
func fromCompactorsMessages(ccMsgs []compactors.Message) []Message {
	result := make([]Message, len(ccMsgs))
	for i, m := range ccMsgs {
		content := make([]ContentBlock, len(m.Content))
		for j, block := range m.Content {
			content[j] = ContentBlock{Type: block.Type, Text: block.Text}
		}
		result[i] = Message{ID: m.ID, Type: MessageType(m.Type), Role: m.Role, Content: content}
	}
	return result
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