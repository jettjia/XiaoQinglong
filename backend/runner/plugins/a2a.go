package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino-ext/a2a/client"
	"github.com/cloudwego/eino-ext/a2a/models"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/jettjia/XiaoQinglong/runner/pkg/logger"
	"github.com/jettjia/XiaoQinglong/runner/types"
)

// ========== A2A Client ==========

// A2AClient represents an A2A agent client using eino-ext a2a library
type A2AClient struct {
	name     string
	endpoint string
	headers  map[string]string
	cli      *client.A2AClient
}

// NewA2AClient creates a new A2A client using eino-ext a2a library
func NewA2AClient(ctx context.Context, config types.A2AAgentConfig) (*A2AClient, error) {
	if config.Endpoint == "" {
		return nil, errors.New("empty endpoint")
	}

	// Clean headers
	headers := make(map[string]string)
	for k, v := range config.Headers {
		if strings.TrimSpace(k) != "" && strings.TrimSpace(v) != "" {
			headers[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}

	// Normalize endpoint
	endpoint := strings.TrimSpace(config.Endpoint)
	if !strings.HasPrefix(endpoint, "http") {
		endpoint = "http://" + endpoint
	}
	if !strings.HasSuffix(endpoint, "/a2a") && !strings.HasSuffix(endpoint, "/jsonrpc") {
		if strings.HasSuffix(endpoint, "/") {
			endpoint += "a2a"
		} else {
			endpoint += "/a2a"
		}
	}

	// Create transport using eino-ext a2a jsonrpc transport
	transport, err := jsonrpc.NewTransport(ctx, &jsonrpc.ClientConfig{
		BaseURL:     endpoint,
		HandlerPath: "",
	})
	if err != nil {
		return nil, fmt.Errorf("create a2a transport failed: %w", err)
	}

	// Create A2A client
	cli, err := client.NewA2AClient(ctx, &client.Config{Transport: transport})
	if err != nil {
		return nil, fmt.Errorf("create a2a client failed: %w", err)
	}

	logger.GetRunnerLogger().Infof("[A2A] Created eino-ext client for agent: %s, endpoint: %s", config.Name, endpoint)

	return &A2AClient{
		name:     config.Name,
		endpoint: endpoint,
		headers:  headers,
		cli:      cli,
	}, nil
}

// Run executes the A2A agent with a query using eino-ext a2a client
func (a *A2AClient) Run(ctx context.Context, query string, traceCtx map[string]string) (string, error) {
	logger.GetRunnerLogger().Infof("[A2A] Calling agent %s with query: %s", a.name, query)

	// Build metadata from trace context
	metadata := make(map[string]any)
	if traceCtx != nil {
		if traceID, ok := traceCtx["trace_id"]; ok {
			metadata["trace_id"] = traceID
		}
		if parentSpanID, ok := traceCtx["parent_span_id"]; ok {
			metadata["parent_span_id"] = parentSpanID
		}
	}

	// Send message using eino-ext a2a client
	result, err := a.cli.SendMessage(ctx, &models.MessageSendParams{
		Message: models.Message{
			Role: models.RoleUser,
			Parts: []models.Part{
				{Kind: models.PartKindText, Text: &query},
			},
		},
		Metadata: metadata,
	})
	if err != nil {
		return "", fmt.Errorf("a2a send message failed: %w", err)
	}

	// Extract content from response
	if result != nil && result.Task != nil && result.Task.Status.Message != nil {
		return extractContent(result.Task.Status.Message)
	}

	if result != nil && result.Message != nil {
		return extractContent(result.Message)
	}

	return "", nil
}

// extractContent extracts text content from A2A message parts
func extractContent(msg *models.Message) (string, error) {
	if msg == nil || msg.Parts == nil {
		return "", nil
	}

	var sb strings.Builder
	for _, part := range msg.Parts {
		if part.Kind == models.PartKindText && part.Text != nil {
			sb.WriteString(*part.Text)
		}
	}
	return sb.String(), nil
}

// CreateA2ARunner creates an ADK runner for the A2A agent
func (a *A2AClient) CreateA2ARunner(ctx context.Context, model model.ToolCallingChatModel) (*adk.Runner, error) {
	logger.GetRunnerLogger().Infof("[A2A] Creating runner for agent: %s", a.name)

	if model == nil {
		return nil, fmt.Errorf("model is required for A2A agent")
	}

	// Create A2A agent using HTTP + JSON-RPC
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        a.name,
		Description: fmt.Sprintf("A2A Agent: %s", a.name),
		Instruction: fmt.Sprintf("你是一个 A2A agent，可以调用远程 agent %s 来完成任务。", a.name),
		Model:       model,
	})
	if err != nil {
		return nil, fmt.Errorf("create agent failed: %w", err)
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{EnableStreaming: false, Agent: agent})
	return runner, nil
}

// ========== A2A Tool ==========

type A2ATool struct {
	clients   map[string]*A2AClient
	callCount *int              // shared counter pointer
	maxCalls  int               // max allowed calls
	traceCtx  map[string]string // trace context (trace_id, parent_span_id)
}

func NewA2ATool(clients map[string]*A2AClient) *A2ATool {
	return &A2ATool{
		clients: clients,
	}
}

// NewA2AToolWithCounter creates an A2ATool with call counting and limits
func NewA2AToolWithCounter(clients map[string]*A2AClient, counter *int, maxCalls int) *A2ATool {
	return &A2ATool{
		clients:   clients,
		callCount: counter,
		maxCalls:  maxCalls,
	}
}

// SetTraceContext sets the trace context for A2A calls
func (t *A2ATool) SetTraceContext(ctx map[string]string) {
	t.traceCtx = ctx
}

func (t *A2ATool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	var agentList []string
	for name := range t.clients {
		agentList = append(agentList, name)
	}

	agentDesc := strings.Join(agentList, ", ")
	if agentDesc == "" {
		agentDesc = "No agents available"
	}

	params := schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"agent": {
			Type:     schema.String,
			Desc:     "The name of the A2A agent to call",
			Enum:     agentList,
			Required: true,
		},
		"query": {
			Type:     schema.String,
			Desc:     "The query to send to the agent",
			Required: true,
		},
	})

	return &schema.ToolInfo{
		Name:        "call_a2a_agent",
		Desc:        fmt.Sprintf("Call external A2A agents. Available agents: %s", agentDesc),
		ParamsOneOf: params,
	}, nil
}

func (t *A2ATool) InvokableRun(ctx context.Context, argumentsInJSON string, opt ...tool.Option) (string, error) {
	// Check and increment call count
	if t.callCount != nil {
		*t.callCount++
		if t.maxCalls > 0 && *t.callCount > t.maxCalls {
			return "", fmt.Errorf("max a2a calls exceeded: %d", t.maxCalls)
		}
	}

	type a2aInput struct {
		Agent string `json:"agent"`
		Query string `json:"query"`
	}

	var input a2aInput
	if err := parseJSON(argumentsInJSON, &input); err != nil {
		return "", fmt.Errorf("parse input failed: %w", err)
	}

	client, ok := t.clients[input.Agent]
	if !ok {
		return "", fmt.Errorf("agent %s not found", input.Agent)
	}

	return client.Run(ctx, input.Query, t.traceCtx)
}

// ========== Helper ==========

func parseJSON(data string, v any) error {
	return json.Unmarshal([]byte(data), v)
}
