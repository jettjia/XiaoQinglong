package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/jettjia/XiaoQinglong/runner/types"
)

// HTTPRunner HTTP 模式的 Runner 客户端
type HTTPRunner struct {
	endpoint string
	client   *http.Client
}

// NewHTTPRunner 创建 HTTP Runner 客户端
func NewHTTPRunner(endpoint string) *HTTPRunner {
	return &HTTPRunner{
		endpoint: endpoint,
		client:   &http.Client{},
	}
}

// StreamEvent SSE 事件
type StreamEvent struct {
	Type string
	Data map[string]any
}

// RunStream 流式运行
func (h *HTTPRunner) RunStream(ctx context.Context, req *types.RunRequest) (<-chan StreamEvent, error) {
	// 设置流式输出
	if req.Options == nil {
		req.Options = &types.RunOptions{}
	}
	req.Options.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, h.endpoint+"/run", strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	eventsChan := make(chan StreamEvent, 100)

	go func() {
		defer close(eventsChan)
		defer resp.Body.Close()

		reader := bufio.NewReader(resp.Body)
		for {
			// 读取事件类型行 (event: xxx)
			eventType := ""
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					if err != io.EOF {
						fmt.Printf("[SSE Error] read error: %v\n", err)
					}
					return
				}
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				if strings.HasPrefix(line, "event:") {
					eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
					break
				}
			}

			// 读取数据行 - Runner 格式是直接 JSON，没有 "data:" 前缀
			dataLine, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					fmt.Printf("[SSE Error] read error: %v\n", err)
				}
				return
			}
			dataLine = strings.TrimSpace(dataLine)
			if dataLine == "" {
				continue
			}

			// 解析 JSON
			var data map[string]any
			if err := json.Unmarshal([]byte(dataLine), &data); err != nil {
				fmt.Printf("[SSE Error] failed to unmarshal: %v, data: %s\n", err, dataLine)
				continue
			}

			eventsChan <- StreamEvent{
				Type: eventType,
				Data: data,
			}
		}
	}()

	return eventsChan, nil
}

// Stop 停止运行
func (h *HTTPRunner) Stop(ctx context.Context, checkpointID string) error {
	body, _ := json.Marshal(map[string]string{"checkpoint_id": checkpointID})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.endpoint+"/stop", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}
