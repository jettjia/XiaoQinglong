package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ========== A2A Client ==========

// A2AClient represents an A2A agent client
type A2AClient struct {
	name      string
	endpoint  string
	headers   map[string]string
	httpClient *http.Client
}

// NewA2AClient creates a new A2A client
func NewA2AClient(ctx context.Context, config A2AAgentConfig) (*A2AClient, error) {
	// Validate endpoint
	if config.Endpoint == "" {
		return nil, fmt.Errorf("empty endpoint")
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

	log.Printf("[A2A] Created client for agent: %s, endpoint: %s", config.Name, endpoint)

	return &A2AClient{
		name:      config.Name,
		endpoint:  endpoint,
		headers:   headers,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Run executes the A2A agent with a query using JSON-RPC over HTTP
func (a *A2AClient) Run(ctx context.Context, query string) (string, error) {
	log.Printf("[A2A] Calling agent %s with query: %s", a.name, query)

	// 构建 JSON-RPC 请求
	reqBody := map[string]any{
		"jsonrpc": "2.0",
		"method":  "agents/call",
		"params": map[string]any{
			"message": map[string]string{
				"role":    "user",
				"content": query,
			},
		},
		"id": time.Now().UnixNano(),
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.endpoint, bytes.NewReader(reqBytes))
	if err != nil {
		return "", fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range a.headers {
		req.Header.Set(k, v)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("agent returned status %d: %s", resp.StatusCode, string(body))
	}

	// 解析 JSON-RPC 响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response failed: %w", err)
	}

	type JSONRPCResponse struct {
		JSONRPC string `json:"jsonrpc"`
		ID      any    `json:"id"`
		Result  struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			Status string `json:"status"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	var rpcResp JSONRPCResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return "", fmt.Errorf("parse response failed: %w", err)
	}

	if rpcResp.Error != nil {
		return "", fmt.Errorf("agent error: %s", rpcResp.Error.Message)
	}

	return rpcResp.Result.Message.Content, nil
}

// CreateA2ARunner creates an ADK runner for the A2A agent
func (a *A2AClient) CreateA2ARunner(ctx context.Context, model model.ToolCallingChatModel) (*adk.Runner, error) {
	log.Printf("[A2A] Creating runner for agent: %s", a.name)

	if model == nil {
		return nil, fmt.Errorf("model is required for A2A agent")
	}

	// 创建 A2A agent（使用简单的方式：HTTP + JSON-RPC）
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
	clients map[string]*A2AClient
}

func NewA2ATool(clients map[string]*A2AClient) *A2ATool {
	return &A2ATool{
		clients: clients,
	}
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

	return client.Run(ctx, input.Query)
}

// ========== Helper ==========

func parseBaseAndPath(endpoint string) (base, path string, err error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", "", fmt.Errorf("empty endpoint")
	}

	// Remove trailing slash
	if strings.HasSuffix(endpoint, "/") {
		endpoint = endpoint[:len(endpoint)-1]
	}

	// Find the last slash before the path
	lastSlash := strings.LastIndex(endpoint, "/")
	if lastSlash < 0 {
		return endpoint, "/", nil
	}

	return endpoint[:lastSlash], endpoint[lastSlash:], nil
}

// headerRoundTripper adds custom headers to HTTP requests
type headerRoundTripper struct {
	base    http.RoundTripper
	headers map[string]string
}

func (rt headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req != nil && len(rt.headers) > 0 {
		for k, v := range rt.headers {
			if strings.TrimSpace(k) != "" && strings.TrimSpace(v) != "" {
				if req.Header.Get(k) == "" {
					req.Header.Set(k, v)
				}
			}
		}
	}
	return rt.base.RoundTrip(req)
}

// parseJSON parses JSON string into target struct
func parseJSON(data string, v any) error {
	return json.Unmarshal([]byte(data), v)
}
