package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ========== MCP Tool Loader ==========

// MCPToolLoader loads MCP tools from SSE endpoint
type MCPToolLoader struct {
	sseURL  string
	headers map[string]string
}

// NewMCPToolLoader creates a new MCP tool loader
func NewMCPToolLoader(sseURL string, headers map[string]string) *MCPToolLoader {
	return &MCPToolLoader{
		sseURL:  sseURL,
		headers: headers,
	}
}

// LoadTools loads MCP tools from the configured SSE endpoint
func (l *MCPToolLoader) LoadTools(ctx context.Context) ([]tool.BaseTool, error) {
	log.Printf("[MCP] Loading tools from: %s", l.sseURL)

	if l.sseURL == "" {
		return nil, nil
	}

	// Request tools list from MCP server
	req, err := http.NewRequestWithContext(ctx, "GET", l.sseURL+"/tools", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range l.headers {
		req.Header.Add(k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MCP server returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	type MCPToolListResponse struct {
		Tools []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			InputSchema any    `json:"inputSchema"`
		} `json:"tools"`
	}

	var toolList MCPToolListResponse
	if err := json.Unmarshal(body, &toolList); err != nil {
		// If parse fails, return empty list
		log.Printf("[MCP] Failed to parse tool list: %v", err)
		return nil, nil
	}

	var tools []tool.BaseTool
	for _, t := range toolList.Tools {
		tools = append(tools, &mcpTool{
			name:        t.Name,
			description: t.Description,
			inputSchema: t.InputSchema,
			sseURL:      l.sseURL,
			headers:     l.headers,
		})
	}

	log.Printf("[MCP] Loaded %d tools", len(tools))
	return tools, nil
}

// mcpTool implements tool.BaseTool for MCP
type mcpTool struct {
	name        string
	description string
	inputSchema any
	sseURL      string
	headers     map[string]string
}

func (t *mcpTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	// Convert inputSchema to params
	var params *schema.ParamsOneOf
	if t.inputSchema != nil {
		if schemaObj, ok := t.inputSchema.(map[string]any); ok {
			props, _ := schemaObj["properties"].(map[string]any)
			required, _ := schemaObj["required"].([]any)

			paramMap := make(map[string]*schema.ParameterInfo)
			for name, prop := range props {
				propMap, _ := prop.(map[string]any)
				paramMap[name] = &schema.ParameterInfo{
					Type:     schema.String, // simplified
					Desc:     getString(propMap, "description"),
					Required: contains(required, name),
				}
			}
			params = schema.NewParamsOneOfByParams(paramMap)
		}
	}

	if params == nil {
		params = schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"args": {Type: schema.Object, Desc: "Tool arguments", Required: false},
		})
	}

	return &schema.ToolInfo{
		Name:        t.name,
		Desc:        t.description,
		ParamsOneOf: params,
	}, nil
}

func (t *mcpTool) InvokableRun(ctx context.Context, argumentsInJSON string, opt ...tool.Option) (string, error) {
	log.Printf("[MCP] Calling tool: %s, args: %s", t.name, argumentsInJSON)

	// Build request to MCP server
	reqBody := map[string]any{
		"name":      t.name,
		"arguments": json.RawMessage(argumentsInJSON),
	}
	reqBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", t.sseURL+"/tools/call", strings.NewReader(string(reqBytes)))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range t.headers {
		req.Header.Add(k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("MCP server returned status %d: %s", resp.StatusCode, string(body))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(respBody), nil
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func contains(slice []any, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ========== MCP Client (wrapper for SSE connection) ==========

// MCPClient manages connection to MCP SSE server
type MCPClient struct {
	sseURL  string
	headers map[string]string
}

// NewMCPClient creates a new MCP client
func NewMCPClient(sseURL string, headers map[string]string) *MCPClient {
	return &MCPClient{
		sseURL:  sseURL,
		headers: headers,
	}
}

// CallTool calls an MCP tool and returns the result
func (c *MCPClient) CallTool(ctx context.Context, toolName string, arguments map[string]any) (string, error) {
	argsJSON, err := json.Marshal(arguments)
	if err != nil {
		return "", fmt.Errorf("failed to marshal arguments: %w", err)
	}

	reqBody := map[string]any{
		"name":      toolName,
		"arguments": argsJSON,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", c.sseURL+"/tools/call", strings.NewReader(string(bodyBytes)))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range c.headers {
		req.Header.Add(k, v)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("MCP server returned status %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(respBody), nil
}

// ========== MCP stdio 模式实现 ==========

// MCPStdioClient 使用 stdio transport 的 MCP 客户端
type MCPStdioClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.Reader
	mu     sync.Mutex
}

func NewMCPStdioClient(cmd string, args []string, env map[string]string) (*MCPStdioClient, error) {
	execCmd := exec.Command(cmd, args...)
	for k, v := range env {
		execCmd.Env = append(execCmd.Env, k+"="+v)
	}

	stdin, err := execCmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := execCmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := execCmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start MCP process: %w", err)
	}

	return &MCPStdioClient{
		cmd:    execCmd,
		stdin:  stdin,
		stdout: stdout,
	}, nil
}

func (c *MCPStdioClient) CallTool(ctx context.Context, name string, arguments map[string]any) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := map[string]any{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]any{
			"name":      name,
			"arguments": arguments,
		},
		"id": time.Now().UnixNano(),
	}

	reqBytes, _ := json.Marshal(req)
	_, err := c.stdin.Write(append(reqBytes, '\n'))
	if err != nil {
		return "", fmt.Errorf("failed to write request: %w", err)
	}

	// 读取响应
	buf := make([]byte, 4096)
	n, err := c.stdout.Read(buf)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(buf[:n]), nil
}

func (c *MCPStdioClient) Close() error {
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
	}
	return nil
}
