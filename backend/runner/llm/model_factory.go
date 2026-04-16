package llm

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloudwego/eino/components/model"
)

// ModelFactory creates chat models based on provider configuration.
// Each provider (openai, claude, gemini, ollama, etc.) implements this interface.
type ModelFactory interface {
	// Provider returns the provider name (e.g., "openai", "claude", "gemini")
	Provider() string

	// CreateChatModel creates a new chat model instance from the given config.
	CreateChatModel(ctx context.Context, cfg *ModelConfig) (model.ToolCallingChatModel, error)
}

// ModelConfig is the internal configuration format passed to factories.
type ModelConfig struct {
	Name        string
	APIKey      string
	APIBase     string
	Temperature float64
	MaxTokens   int
	TopP        float64
	ExtraFields map[string]any
}

// global factory registry
var (
	factoriesMu sync.RWMutex
	factories   = make(map[string]ModelFactory)
)

// RegisterFactory registers a model factory for a provider.
func RegisterFactory(factory ModelFactory) {
	factoriesMu.Lock()
	defer factoriesMu.Unlock()
	factories[factory.Provider()] = factory
}

// GetFactory returns the factory for the given provider.
func GetFactory(provider string) (ModelFactory, error) {
	factoriesMu.RLock()
	defer factoriesMu.RUnlock()
	f, ok := factories[provider]
	if !ok {
		return nil, fmt.Errorf("no model factory registered for provider: %s", provider)
	}
	return f, nil
}

// ListProviders returns a list of all registered provider names.
func ListProviders() []string {
	factoriesMu.RLock()
	defer factoriesMu.RUnlock()
	providers := make([]string, 0, len(factories))
	for p := range factories {
		providers = append(providers, p)
	}
	return providers
}