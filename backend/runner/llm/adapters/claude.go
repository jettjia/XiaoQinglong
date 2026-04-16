package adapters

import (
	"context"

	claude "github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino/components/model"
	"github.com/jettjia/XiaoQinglong/runner/llm"
)

type claudeFactory struct{}

func (f *claudeFactory) Provider() string {
	return "claude"
}

func (f *claudeFactory) CreateChatModel(ctx context.Context, cfg *llm.ModelConfig) (model.ToolCallingChatModel, error) {
	claudeCfg := &claude.Config{
		APIKey: cfg.APIKey,
		Model:  cfg.Name,
	}

	if cfg.APIBase != "" {
		baseURL := cfg.APIBase
		claudeCfg.BaseURL = &baseURL
	}
	if cfg.Temperature > 0 {
		t := float32(cfg.Temperature)
		claudeCfg.Temperature = &t
	}
	if cfg.MaxTokens > 0 {
		claudeCfg.MaxTokens = cfg.MaxTokens
	}
	if cfg.TopP > 0 {
		t := float32(cfg.TopP)
		claudeCfg.TopP = &t
	}

	return claude.NewChatModel(ctx, claudeCfg)
}

func init() {
	llm.RegisterFactory(&claudeFactory{})
	// 注册 anthropic provider 作为 claude 的别名（部分 OpenAI 兼容 API 如 MiniMax 使用 Anthropic 格式）
	llm.RegisterFactory(&anthropicFactory{})
}

// anthropicFactory 是 claude 的别名，用于支持 Anthropic 兼容格式的 API（如 MiniMax）
type anthropicFactory struct{}

func (f *anthropicFactory) Provider() string {
	return "anthropic"
}

func (f *anthropicFactory) CreateChatModel(ctx context.Context, cfg *llm.ModelConfig) (model.ToolCallingChatModel, error) {
	claudeCfg := &claude.Config{
		APIKey: cfg.APIKey,
		Model:  cfg.Name,
	}

	if cfg.APIBase != "" {
		baseURL := cfg.APIBase
		claudeCfg.BaseURL = &baseURL
	}
	if cfg.Temperature > 0 {
		t := float32(cfg.Temperature)
		claudeCfg.Temperature = &t
	}
	if cfg.MaxTokens > 0 {
		claudeCfg.MaxTokens = cfg.MaxTokens
	}
	if cfg.TopP > 0 {
		t := float32(cfg.TopP)
		claudeCfg.TopP = &t
	}

	return claude.NewChatModel(ctx, claudeCfg)
}
