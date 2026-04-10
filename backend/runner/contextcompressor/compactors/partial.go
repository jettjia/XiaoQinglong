package compactors

import (
	"context"
	"errors"
	"fmt"

	"github.com/jettjia/XiaoQinglong/runner/contextcompressor/prompt"
)

// PartialCompacter 只总结最近的消息
type PartialCompacter struct {
	chatModel ChatModel
	tokenizer Tokenizer
	config    *Config
}

// NewPartialCompacter 创建部分压缩器
func NewPartialCompacter(chatModel ChatModel, tokenizer Tokenizer, config *Config) *PartialCompacter {
	if config == nil {
		config = &Config{MaxOutputTokens: 20000, CompactBufferTokens: 13000}
	}
	return &PartialCompacter{
		chatModel:  chatModel,
		tokenizer:  tokenizer,
		config:     config,
	}
}

// Compact 总结最近消息，保留早期上下文
func (c *PartialCompacter) Compact(ctx context.Context, messages []Message, opts ...func(*Config)) (*CompactionResult, error) {
	// 应用选项
	for _, opt := range opts {
		opt(c.config)
	}

	if len(messages) == 0 {
		return nil, errors.New("not enough messages to compact")
	}

	// 1. 确定保留点
	preserveCount := len(messages) / 2
	if preserveCount < 2 {
		preserveCount = 2
	}

	toSummarize := messages[:len(messages)-preserveCount]
	toKeep := messages[len(messages)-preserveCount:]

	if len(toSummarize) == 0 {
		return nil, errors.New("nothing to summarize")
	}

	// 2. 剥离图片
	cleaned := StripImagesFromMessages(toSummarize)

	// 3. 估算压缩前 token
	preTokens := c.tokenizer.EstimateMessages(messages)

	// 4. 构建压缩提示
	direction := prompt.PartialCompactDirectionFrom
	compactPrompt := prompt.GetPartialCompactPrompt(c.config.CustomInstructions, direction)

	// 5. 构建消息列表
	summaryRequest := Message{
		Type:    "user",
		Content: []ContentBlock{{Type: "text", Text: compactPrompt}},
	}

	systemPrompt := Message{
		Type:    "system",
		Content: []ContentBlock{{Type: "text", Text: "You are a helpful AI assistant tasked with summarizing conversations."}},
	}

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
	if len(toSummarize) > 0 {
		lastMsgID = toSummarize[len(toSummarize)-1].ID
	}
	boundary := CreateCompactBoundaryMessage("manual", preTokens, lastMsgID)

	// 9. 构建摘要消息
	summaryMsg := NewTextMessage("user", summary)

	// 10. 估算压缩后 token
	postTokens := c.tokenizer.EstimateMessages(toKeep)

	return &CompactionResult{
		BoundaryMarker:    boundary,
		SummaryMessages:   []Message{*summaryMsg},
		MessagesToKeep:    toKeep,
		PreCompactTokens:  preTokens,
		PostCompactTokens: postTokens,
	}, nil
}