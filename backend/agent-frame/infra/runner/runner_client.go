package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RunnerClient 调用 Runner 服务
type RunnerClient struct {
	baseURL    string
	httpClient *http.Client
}

// IntentResult 意图识别结果
type IntentResult struct {
	Intent      string         `json:"intent"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Data        map[string]any `json:"data"`
}

// ModelConfig 模型配置
type ModelConfig struct {
	Provider string `json:"provider"`
	Name    string `json:"name"`
	ApiKey  string `json:"api_key"`
	ApiBase string `json:"api_base"`
}

// NewRunnerClient 创建 RunnerClient
func NewRunnerClient(runnerURL string) *RunnerClient {
	return &RunnerClient{
		baseURL: runnerURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// DefaultRunnerClient 创建默认配置的 RunnerClient
func DefaultRunnerClient() *RunnerClient {
	return NewRunnerClient("http://localhost:18080")
}

// RecognizeIntent 调用 Runner LLM 进行意图识别
func (c *RunnerClient) RecognizeIntent(ctx context.Context, command string, modelConfig *ModelConfig) (*IntentResult, error) {
	// 构建意图识别的 prompt
	prompt := fmt.Sprintf(`你是一个 AI Agent 平台的管理助手。根据用户的指令，识别管理员意图。

支持的意图：
1. create_agent: 创建新智能体。提取 name, description。
2. add_model: 添加模型配置。提取 name, provider, api_base(可选)。
3. show_inbox: 查看收件箱/任务。
4. config_kb: 配置知识库。提取 name, retrieval_url。
5. test_kb_recall: 测试知识库召回。提取 kb_name(可选), query。
6. install_skill: 安装技能（无法自动完成，返回引导）。
7. unknown: 无法识别。

用户指令: "%s"

返回 JSON 格式（只返回 JSON，不要其他内容）：
{
  "intent": "意图类型",
  "title": "简短标题",
  "description": "操作描述",
  "data": { 提取的参数 }
}`, command)

	// 构建 Runner 请求 - 必须包含 models 配置
	runnerReq := map[string]any{
		"prompt": prompt,
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"models": map[string]any{
			"default": map[string]any{
				"provider": modelConfig.Provider,
				"name":     modelConfig.Name,
				"api_key":  modelConfig.ApiKey,
				"api_base": modelConfig.ApiBase,
			},
		},
		"options": map[string]any{
			"temperature": 0.3, // 低温度保证输出稳定
			"max_tokens": 500,
		},
	}

	body, err := json.Marshal(runnerReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.baseURL + "/run"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call runner: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("runner returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// 解析 Runner 响应
	var runnerResp map[string]any
	if err := json.Unmarshal(respBody, &runnerResp); err != nil {
		return nil, fmt.Errorf("failed to parse runner response: %w", err)
	}

	// 尝试从 content 字段提取 LLM 输出
	var llmOutput string
	if content, ok := runnerResp["content"].(string); ok {
		llmOutput = content
	}

	// 如果 content 为空，尝试从 metadata 中获取
	if llmOutput == "" {
		if metadata, ok := runnerResp["metadata"].(map[string]any); ok {
			if content, ok := metadata["content"].(string); ok {
				llmOutput = content
			}
		}
	}

	if llmOutput == "" {
		return nil, fmt.Errorf("no content in runner response")
	}

	// 解析 LLM 输出的 JSON
	var result IntentResult
	if err := json.Unmarshal([]byte(llmOutput), &result); err != nil {
		// 尝试清理 JSON（可能包含 markdown 代码块）
		cleaned := cleanJSON(llmOutput)
		if err2 := json.Unmarshal([]byte(cleaned), &result); err2 != nil {
			return nil, fmt.Errorf("failed to parse intent result: %w, output: %s", err2, llmOutput)
		}
	}

	return &result, nil
}

// cleanJSON 清理 JSON 字符串（去除 markdown 代码块等）
func cleanJSON(s string) string {
	// 去除 ```json 和 ``` 包裹
	const jsonBlockPrefix = "```json"
	const jsonBlockSuffix = "```"
	const jsonBlockSuffixAlt = "```\n"

	// 去除开头可能有的 markdown 代码块标记
	if len(s) > 0 && s[0] == '`' {
		// 检查是否是 ```json 开头
		if len(s) >= len(jsonBlockPrefix) && s[:len(jsonBlockPrefix)] == jsonBlockPrefix {
			s = s[len(jsonBlockPrefix):]
			// 去除剩余的换行
			for len(s) > 0 && (s[0] == '\n' || s[0] == '\r') {
				s = s[1:]
			}
		}
	}

	// 去除结尾的 ``` 标记
	if len(s) >= len(jsonBlockSuffix) {
		endIdx := len(s) - len(jsonBlockSuffix)
		if s[endIdx:] == jsonBlockSuffix || s[endIdx:] == jsonBlockSuffixAlt {
			s = s[:endIdx]
			// 去除可能的换行
			for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == '`') {
				s = s[:len(s)-1]
			}
		}
	}

	return s
}
