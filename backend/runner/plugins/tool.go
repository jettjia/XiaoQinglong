package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/jettjia/XiaoQinglong/runner/types"
)

// ========== HTTP Tool ==========

type HTTPTool struct {
	name        string
	description string
	endpoint    string
	method      string
	headers     map[string]string
}

func NewHTTPTool(config types.ToolConfig) *HTTPTool {
	return &HTTPTool{
		name:        config.Name,
		description: config.Description,
		endpoint:    config.Endpoint,
		method:      config.Method,
		headers:     config.Headers,
	}
}

// Info returns tool information
func (t *HTTPTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	params := schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"params": {
			Type:     schema.Object,
			Desc:     "Query or path parameters",
			Required: false,
		},
		"body": {
			Type:     schema.Object,
			Desc:     "Request body",
			Required: false,
		},
	})
	return &schema.ToolInfo{
		Name:        t.name,
		Desc:        t.description,
		ParamsOneOf: params,
	}, nil
}

// InvokableRun executes the HTTP tool
func (t *HTTPTool) InvokableRun(ctx context.Context, argumentsInJSON string, opt ...tool.Option) (string, error) {
	var inputMap map[string]any
	if err := json.Unmarshal([]byte(argumentsInJSON), &inputMap); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	params, _ := inputMap["params"].(map[string]any)
	body := inputMap["body"]

	// Build URL with path parameters
	url := t.endpoint
	for k, v := range params {
		url = replacePathParam(url, "{"+k+"}", fmt.Sprintf("%v", v))
	}

	// Build request
	var req *http.Request
	if body != nil {
		bodyBytes, _ := json.Marshal(body)
		req, _ = http.NewRequestWithContext(ctx, t.method, url, bytes.NewReader(bodyBytes))
	} else {
		req, _ = http.NewRequestWithContext(ctx, t.method, url, nil)
	}

	// Add headers
	for k, v := range t.headers {
		req.Header.Add(k, v)
	}

	// Execute request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(respBody), nil
}

func replacePathParam(url, key, value string) string {
	return string(bytes.ReplaceAll([]byte(url), []byte(key), []byte(value)))
}
