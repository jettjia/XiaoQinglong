package plugins

import (
	"bufio"
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
	log.Printf("[MCP] Loading tools from SSE: %s", l.sseURL)

	if l.sseURL == "" {
		return nil, nil
	}

	// Try to get tools via SSE connection
	tools, err := l.loadToolsFromSSE(ctx)
	if err != nil {
		log.Printf("[MCP] SSE tool loading failed: %v, trying HTTP...", err)
		// Fallback to HTTP if SSE fails
		tools, err = l.loadToolsFromHTTP(ctx)
		if err != nil {
			return nil, fmt.Errorf("both SSE and HTTP MCP loading failed: %w", err)
		}
	}

	log.Printf("[MCP] Loaded %d tools", len(tools))
	return tools, nil
}

// loadToolsFromSSE connects to MCP SSE server and receives tool definitions
func (l *MCPToolLoader) loadToolsFromSSE(ctx context.Context) ([]tool.BaseTool, error) {
	// MCP SSE: connect to the SSE endpoint and receive tool announcements
	// mcp-go library uses /sse as the default SSE endpoint
	sseURL := strings.TrimSuffix(l.sseURL, "/") + "/sse"
	req, err := http.NewRequestWithContext(ctx, "GET", sseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSE request: %w", err)
	}

	for k, v := range l.headers {
		req.Header.Add(k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MCP SSE: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MCP SSE returned status %d", resp.StatusCode)
	}

	// Parse SSE stream to find tool definitions
	reader := bufio.NewReader(resp.Body)
	var tools []tool.BaseTool
	var toolDefs []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		InputSchema any    `json:"inputSchema"`
	}

	// Read SSE events
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("error reading SSE stream: %w", err)
		}

		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimSpace(data)

			// Try to parse as JSON array of tools or individual tool announcement
			if strings.HasPrefix(data, "[") {
				if err := json.Unmarshal([]byte(data), &toolDefs); err != nil {
					// Try parsing as individual tool
					var tool struct {
						Name        string `json:"name"`
						Description string `json:"description"`
						InputSchema any    `json:"inputSchema"`
					}
					if err2 := json.Unmarshal([]byte(data), &tool); err2 == nil && tool.Name != "" {
						toolDefs = append(toolDefs, tool)
					}
				}
			} else if strings.HasPrefix(data, "{") {
				// Single tool announcement
				var tool struct {
					Name        string `json:"name"`
					Description string `json:"description"`
					InputSchema any    `json:"inputSchema"`
				}
				if err := json.Unmarshal([]byte(data), &tool); err == nil && tool.Name != "" {
					toolDefs = append(toolDefs, tool)
				}
			}
		}

		// Check for end of stream
		if line == "" || line == "event: done" {
			break
		}
	}

	// If no tools from SSE, return empty (caller can fallback)
	if len(toolDefs) == 0 {
		return nil, fmt.Errorf("no tools received from SSE")
	}

	for _, t := range toolDefs {
		tools = append(tools, &mcpTool{
			name:        t.Name,
			description: t.Description,
			inputSchema: t.InputSchema,
			sseURL:      l.sseURL,
			headers:     l.headers,
		})
	}

	return tools, nil
}

// loadToolsFromHTTP tries to load tools via HTTP GET (for MCP servers with REST API)
func (l *MCPToolLoader) loadToolsFromHTTP(ctx context.Context) ([]tool.BaseTool, error) {
	// Try common MCP HTTP endpoints
	endpoints := []string{
		l.sseURL + "/tools",
		l.sseURL,
	}

	var lastErr error
	for _, endpoint := range endpoints {
		req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
		if err != nil {
			lastErr = fmt.Errorf("failed to create request for %s: %w", endpoint, err)
			continue
		}

		for k, v := range l.headers {
			req.Header.Add(k, v)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed for %s: %w", endpoint, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			resp.Body.Read(make([]byte, 1)) // drain body to allow reuse
			lastErr = fmt.Errorf("endpoint %s returned status %d", endpoint, resp.StatusCode)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("failed to read response from %s: %w", endpoint, err)
			continue
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
			lastErr = fmt.Errorf("failed to parse tool list from %s: %w", endpoint, err)
			continue
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

		return tools, nil
	}

	return nil, lastErr
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

	req, err := http.NewRequestWithContext(ctx, "POST", t.sseURL+"/message", strings.NewReader(string(reqBytes)))
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
