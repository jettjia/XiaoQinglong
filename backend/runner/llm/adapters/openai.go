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
	// 注册 Custom provider，作为 OpenAI 兼容的自定义模型别名
	llm.RegisterFactory(&customFactory{})
}

// customFactory 用于用户自定义注册的 OpenAI 兼容模型
type customFactory struct{}

func (f *customFactory) Provider() string {
	return "custom"
}

func (f *customFactory) CreateChatModel(ctx context.Context, cfg *llm.ModelConfig) (model.ToolCallingChatModel, error) {
	// Custom 与 OpenAI 行为完全相同，使用 OpenAI 兼容格式
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