package contextcompressor

import (
	"context"

	"github.com/jettjia/XiaoQinglong/runner/contextcompressor/compactors"
)

// ChatModelProxy ChatModel 代理，用于 contextcompressor
type ChatModelProxy struct {
	GenerateFunc func(ctx context.Context, messages []compactors.Message) (string, error)
}

// Generate 实现 compactors.ChatModel 接口
func (c *ChatModelProxy) Generate(ctx context.Context, messages []compactors.Message) (string, error) {
	if c.GenerateFunc != nil {
		return c.GenerateFunc(ctx, messages)
	}
	return "", nil
}

// DefaultTokenizerImpl 默认的 Token 估算器实现
type DefaultTokenizerImpl struct {
	charsPerToken float64
}

// NewDefaultTokenizerImpl 创建默认估算器
func NewDefaultTokenizerImpl(charsPerToken float64) *DefaultTokenizerImpl {
	if charsPerToken <= 0 {
		charsPerToken = 4.0
	}
	return &DefaultTokenizerImpl{charsPerToken: charsPerToken}
}

// Estimate 估算单条文本的 token 数
func (t *DefaultTokenizerImpl) Estimate(text string) int {
	if text == "" {
		return 0
	}
	return int(float64(len(text)) / t.charsPerToken)
}

// EstimateMessages 估算消息列表的总 token 数
func (t *DefaultTokenizerImpl) EstimateMessages(messages []Message) int {
	total := 0
	for i := range messages {
		total += t.estimateMessage(&messages[i])
	}
	return total
}

func (t *DefaultTokenizerImpl) estimateMessage(msg *Message) int {
	if msg == nil {
		return 0
	}

	overhead := 4
	contentLen := 0

	for _, block := range msg.Content {
		switch block.Type {
		case "text":
			contentLen += len(block.Text)
		case "image", "document":
			contentLen += 85 * 4
		}
	}

	return overhead + int(float64(contentLen)/t.charsPerToken)
}

// IntegrationService 集成服务，用于将 context compressor 集成到 runner
type IntegrationService struct {
	compactor *Compactor
	enabled   bool
}

// NewIntegrationService 创建集成服务
func NewIntegrationService(compactor *Compactor) *IntegrationService {
	return &IntegrationService{
		compactor: compactor,
		enabled:   compactor != nil,
	}
}

// IsEnabled 检查是否启用
func (s *IntegrationService) IsEnabled() bool {
	return s.enabled
}

// GetCompactor 获取压缩器
func (s *IntegrationService) GetCompactor() *Compactor {
	return s.compactor
}

// ShouldCompact 检查是否应该压缩
func (s *IntegrationService) ShouldCompact(messages []Message) bool {
	if !s.enabled || s.compactor == nil {
		return false
	}
	threshold := GetAutoCompactThreshold(s.compactor.config.Model)
	return s.compactor.ShouldCompact(messages, threshold)
}

// Compact 执行压缩
func (s *IntegrationService) Compact(ctx context.Context, messages []Message) (*CompactionResult, error) {
	if !s.enabled || s.compactor == nil {
		return nil, nil
	}
	return s.compactor.Compact(ctx, messages)
}

// GetTokenCount 获取消息 token 数
func (s *IntegrationService) GetTokenCount(messages []Message) int {
	if !s.enabled || s.compactor == nil {
		return 0
	}
	return s.compactor.GetTokenCount(messages)
}

// GetWarningState 获取警告状态
func (s *IntegrationService) GetWarningState(tokenCount int) TokenWarningState {
	if !s.enabled || s.compactor == nil {
		return TokenWarningState{}
	}
	return CalculateTokenWarningState(tokenCount, s.compactor.config.Model, true)
}

// AutoCompactThreshold 获取自动压缩阈值
func (s *IntegrationService) AutoCompactThreshold() int {
	if !s.enabled || s.compactor == nil {
		return 150000
	}
	return GetAutoCompactThreshold(s.compactor.config.Model)
}
