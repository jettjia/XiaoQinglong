package subagent

import (
	"context"
	"sync"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

// SubAgentAdapter wraps a SubAgent's inner adk.Agent to implement the adk.Agent interface
// This allows it to be used as a sub-agent in deep.New()
type SubAgentAdapter struct {
	name        string
	description string
	agent      adk.Agent
	config     *SubAgentConfig
	mu         sync.RWMutex
	result     *SubAgentResult
}

// NewSubAgentAdapter creates a new SubAgentAdapter
func NewSubAgentAdapter(config *SubAgentConfig, agent adk.Agent) *SubAgentAdapter {
	return &SubAgentAdapter{
		name:        config.Name,
		description: config.Description,
		agent:       agent,
		config:      config,
	}
}

// Name returns the agent name
func (s *SubAgentAdapter) Name(ctx context.Context) string {
	return s.name
}

// Description returns the agent description
func (s *SubAgentAdapter) Description(ctx context.Context) string {
	return s.description
}

// Run executes the sub-agent with the given input
func (s *SubAgentAdapter) Run(ctx context.Context, input *adk.AgentInput, options ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()

	go func() {
		defer gen.Close()

		// Convert AgentInput.Messages to schema.Messages
		messages := make([]*schema.Message, 0, len(input.Messages))
		for _, msg := range input.Messages {
			switch msg.Role {
			case schema.User:
				messages = append(messages, schema.UserMessage(msg.Content))
			case schema.Assistant:
				messages = append(messages, schema.AssistantMessage(msg.Content, nil))
			case schema.System:
				messages = append(messages, schema.SystemMessage(msg.Content))
			}
		}

		// Run the inner agent
		innerEvents := s.agent.Run(ctx, &adk.AgentInput{
			Messages:        messages,
			EnableStreaming: input.EnableStreaming,
		}, options...)

		// Forward events
		for {
			event, ok := innerEvents.Next()
			if !ok {
				break
			}
			gen.Send(event)
		}
	}()

	return iter
}