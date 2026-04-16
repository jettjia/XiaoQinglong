package adapters

import (
	"context"

	claude "github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/jettjia/XiaoQinglong/runner/llm"
	"github.com/cloudwego/eino/components/model"
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
}