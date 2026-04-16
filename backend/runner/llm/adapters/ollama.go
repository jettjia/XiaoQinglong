package adapters

import (
	"context"

	ollama "github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/eino-contrib/ollama/api"
	"github.com/jettjia/XiaoQinglong/runner/llm"
	"github.com/cloudwego/eino/components/model"
)

type ollamaFactory struct{}

func (f *ollamaFactory) Provider() string {
	return "ollama"
}

func (f *ollamaFactory) CreateChatModel(ctx context.Context, cfg *llm.ModelConfig) (model.ToolCallingChatModel, error) {
	ollamaCfg := &ollama.ChatModelConfig{
		BaseURL: cfg.APIBase,
		Model:   cfg.Name,
		Options: &api.Options{},
	}

	if cfg.Temperature > 0 {
		t := float32(cfg.Temperature)
		ollamaCfg.Options.Temperature = t
	}
	if cfg.TopP > 0 {
		t := float32(cfg.TopP)
		ollamaCfg.Options.TopP = t
	}

	return ollama.NewChatModel(ctx, ollamaCfg)
}

func init() {
	llm.RegisterFactory(&ollamaFactory{})
}