package adapters

import (
	"context"

	openai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/jettjia/XiaoQinglong/runner/llm"
	"github.com/cloudwego/eino/components/model"
)

type openaiFactory struct{}

func (f *openaiFactory) Provider() string {
	return "openai"
}

func (f *openaiFactory) CreateChatModel(ctx context.Context, cfg *llm.ModelConfig) (model.ToolCallingChatModel, error) {
	openaiCfg := &openai.ChatModelConfig{
		APIKey:  cfg.APIKey,
		Model:   cfg.Name,
		BaseURL: cfg.APIBase,
	}

	if cfg.Temperature > 0 {
		t := float32(cfg.Temperature)
		openaiCfg.Temperature = &t
	}
	if cfg.MaxTokens > 0 {
		openaiCfg.MaxTokens = &cfg.MaxTokens
	}
	if cfg.TopP > 0 {
		t := float32(cfg.TopP)
		openaiCfg.TopP = &t
	}
	if len(cfg.ExtraFields) > 0 {
		openaiCfg.ExtraFields = cfg.ExtraFields
	}

	return openai.NewChatModel(ctx, openaiCfg)
}

func init() {
	llm.RegisterFactory(&openaiFactory{})
}