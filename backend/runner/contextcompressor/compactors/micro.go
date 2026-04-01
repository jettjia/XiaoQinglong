package compactors

import (
	"context"
)

// MicroCompacter 微压缩器 - 工具结果压缩
type MicroCompacter struct {
	tokenizer Tokenizer
	config    *Config
}

// NewMicroCompacter 创建微压缩器
func NewMicroCompacter(tokenizer Tokenizer, config *Config) *MicroCompacter {
	if config == nil {
		config = &Config{MaxOutputTokens: 5000, CompactBufferTokens: 13000}
	}
	return &MicroCompacter{
		tokenizer: tokenizer,
		config:    config,
	}
}

// Compact 压缩工具结果
func (c *MicroCompacter) Compact(ctx context.Context, messages []Message, opts ...func(*Config)) (*CompactionResult, error) {
	// 应用选项
	for _, opt := range opts {
		opt(c.config)
	}

	if len(messages) == 0 {
		return nil, nil
	}

	// 1. 剥离图片/文档
	cleaned := StripImagesFromMessages(messages)

	// 2. 估算 token
	preTokens := c.tokenizer.EstimateMessages(messages)
	postTokens := c.tokenizer.EstimateMessages(cleaned)

	return &CompactionResult{
		BoundaryMarker:    nil,
		SummaryMessages:   nil,
		MessagesToKeep:    cleaned,
		PreCompactTokens:  preTokens,
		PostCompactTokens: postTokens,
	}, nil
}