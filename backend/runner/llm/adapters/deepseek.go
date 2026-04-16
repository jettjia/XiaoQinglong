package adapters

import (
	"context"

	deepseek "github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/jettjia/XiaoQinglong/runner/llm"
	"github.com/cloudwego/eino/components/model"
)

type deepseekFactory struct{}

func (f *deepseekFactory) Provider() string {
	return "deepseek"
}

func (f *deepseekFactory) CreateChatModel(ctx context.Context, cfg *llm.ModelConfig) (model.ToolCallingChatModel, error) {
	deepseekCfg := &deepseek.ChatModelConfig{
		APIKey:  cfg.APIKey,
		Model:   cfg.Name,
		BaseURL: cfg.APIBase,
	}

	if cfg.Temperature > 0 {
		deepseekCfg.Temperature = float32(cfg.Temperature)
	}
	if cfg.MaxTokens > 0 {
		deepseekCfg.MaxTokens = cfg.MaxTokens
	}
	if cfg.TopP > 0 {
		deepseekCfg.TopP = float32(cfg.TopP)
	}

	return deepseek.NewChatModel(ctx, deepseekCfg)
}

func init() {
	llm.RegisterFactory(&deepseekFactory{})
}