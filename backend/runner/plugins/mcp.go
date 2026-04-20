package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/bytedance/sonic"
	"github.com/eino-contrib/jsonschema"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/jettjia/XiaoQinglong/runner/pkg/logger"
)

// Config MCP tool configuration
type Config struct {
	// Cli is the MCP client (should be initialized before passing)
	Cli client.MCPClient
	// ToolNameList specifies which tools to fetch from MCP server
	// If empty, all available tools will be fetched
	ToolNameList []string
	// CustomHeaders specifies the http headers passed to mcp server
	CustomHeaders map[string]string
	// Meta specifies the metadata passed to mcp server when requesting
	Meta *mcp.Meta
}

// MCPLoader loads MCP tools using the official mcp-go library
type MCPLoader struct {
	cli        client.MCPClient
	toolNames  []string
	headers    map[string]string
	meta       *mcp.Meta
}

// NewMCPLoader creates a new MCP loader
func NewMCPLoader(cli client.MCPClient, toolNames []string, headers map[string]string, meta *mcp.Meta) *MCPLoader {
	return &MCPLoader{
		cli:        cli,
		toolNames:  toolNames,
		headers:    headers,
		meta:       meta,
	}
}

// GetTools loads MCP tools from the configured client
func (l *MCPLoader) GetTools(ctx context.Context) ([]tool.BaseTool, error) {
	header := http.Header{}
	if l.headers != nil {
		for k, v := range l.headers {
			header.Set(k, v)
		}

	}

	listResults, err := l.cli.ListTools(ctx, mcp.ListToolsRequest{
		Header: header,
	})
	if err != nil {
		return nil, fmt.Errorf("list mcp tools fail: %w", err)
	}

	nameSet := make(map[string]struct{})
	for _, name := range l.toolNames {
		nameSet[name] = struct{}{}
	}

	ret := make([]tool.BaseTool, 0, len(listResults.Tools))
	for _, t := range listResults.Tools {
		if len(l.toolNames) > 0 {
			if _, ok := nameSet[t.Name]; !ok {
				continue
			}
		}

		marshaledInputSchema, err := sonic.Marshal(t.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("conv mcp tool input schema fail(marshal): %w, tool name: %s", err, t.Name)
		}
		inputSchema := &jsonschema.Schema{}
		err = sonic.Unmarshal(marshaledInputSchema, inputSchema)
		if err != nil {
			return nil, fmt.Errorf("conv mcp tool input schema fail(unmarshal): %w, tool name: %s", err, t.Name)
		}

		ret = append(ret, &mcpToolHelper{
			cli:           l.cli,
			customHeaders: l.headers,
			info: &schema.ToolInfo{
				Name:        t.Name,
				Desc:        t.Description,
				ParamsOneOf: schema.NewParamsOneOfByJSONSchema(inputSchema),
			},
			meta: l.meta,
		})
	}

	return ret, nil
}

type mcpToolHelper struct {
	cli           client.MCPClient
	info          *schema.ToolInfo
	customHeaders map[string]string
	meta          *mcp.Meta
}

func (m *mcpToolHelper) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return m.info, nil
}

func (m *mcpToolHelper) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	logger.Infof("[MCP] Calling tool: %s, args: %s", m.info.Name, argumentsInJSON)

	specOptions := getMCPSpecificOptions(&mcpOptions{
		customHeaders: m.customHeaders,
		meta:          m.meta,
	}, opts...)

	headers := http.Header{}
	if specOptions.customHeaders != nil {
		for k, v := range specOptions.customHeaders {
			headers.Set(k, v)
		}
	}

	result, err := m.cli.CallTool(ctx, mcp.CallToolRequest{
		Request: mcp.Request{
			Method: "tools/call",
		},
		Header: headers,
		Params: mcp.CallToolParams{
			Name:      m.info.Name,
			Arguments: json.RawMessage(argumentsInJSON),
			Meta:      specOptions.meta,
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to call mcp tool: %w", err)
	}

	marshaledResult, err := sonic.MarshalString(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal mcp tool result: %w", err)
	}
	if result.IsError {
		return "", fmt.Errorf("failed to call mcp tool, mcp server return error: %s", marshaledResult)
	}

	return marshaledResult, nil
}

type mcpOptions struct {
	customHeaders map[string]string
	meta          *mcp.Meta
}

func getMCPSpecificOptions(defaults *mcpOptions, opts ...tool.Option) *mcpOptions {
	return tool.GetImplSpecificOptions(defaults, opts...)
}

// WithCustomHeaders sets custom headers for MCP requests
func WithCustomHeaders(m map[string]string) tool.Option {
	return tool.WrapImplSpecificOptFn(func(o *mcpOptions) {
		o.customHeaders = m
	})
}

// WithMeta sets metadata for MCP requests
func WithMeta(meta *mcp.Meta) tool.Option {
	return tool.WrapImplSpecificOptFn(func(o *mcpOptions) {
		o.meta = meta
	})
}
