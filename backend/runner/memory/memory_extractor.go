package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/jettjia/XiaoQinglong/runner/types"
)

// MemoryExtractor 记忆提取器
type MemoryExtractor struct {
	modelConfig *types.ModelConfig
	apiBase     string
}

// NewMemoryExtractor 创建记忆提取器
func NewMemoryExtractor(modelConfig *types.ModelConfig) *MemoryExtractor {
	apiBase := modelConfig.APIBase
	if apiBase == "" {
		apiBase = "https://api.openai.com/v1"
	}
	return &MemoryExtractor{
		modelConfig: modelConfig,
		apiBase:     apiBase,
	}
}

// ExtractMemories 从对话中提取记忆
func (e *MemoryExtractor) ExtractMemories(ctx context.Context, userInput, assistantOutput string) ([]types.MemoryEntry, error) {
	if e.modelConfig.APIKey == "" || e.modelConfig.Name == "" {
		return nil, nil
	}

	// 构建提取 prompt
	extractionPrompt := `你是一个记忆提取专家。从以下对话中提取关键信息并以 JSON 格式返回。

对话:
用户: ` + userInput + `
助手: ` + assistantOutput + `

记忆类型说明:
- user: 用户角色、偏好、知识 (如 "用户是数据科学家")
- feedback: 用户指导 (如 "用户说不要用 mock 测试")
- project: 项目上下文 (如 "项目截止日期是 3 月 15 日")
- reference: 外部系统指针 (如 "bug 在 Linear 的 INGEST 项目跟踪")

提取规则:
1. 只提取对话中明确提到的信息，不要推测
2. 每条记忆需要有: name(英文简短带下划线), description(描述), type(类型), content(完整内容)
3. 优先提取 feedback 类型（用户明确给过指导的）
4. 最多提取 5 条记忆
5. 如果没有值得记忆的信息，返回空数组 []

返回格式:
{
  "memories": [
    {"name": "user_role", "description": "用户是数据科学家，专注于日志系统", "type": "user", "content": "..."},
    {"name": "feedback_testing", "description": "不要使用 mock 数据库测试", "type": "feedback", "content": "..."}
  ]
}`

	// 调用 LLM
	result, err := e.callLLM(ctx, extractionPrompt)
	if err != nil {
		return nil, err
	}

	// 解析结果
	var extractionResult struct {
		Memories []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Type        string `json:"type"`
			Content     string `json:"content"`
		} `json:"memories"`
	}
	if err := json.Unmarshal([]byte(result), &extractionResult); err != nil {
		return nil, fmt.Errorf("failed to parse extraction result: %w", err)
	}

	if len(extractionResult.Memories) == 0 {
		return nil, nil
	}

	// 转换为 MemoryEntry
	memories := make([]types.MemoryEntry, 0, len(extractionResult.Memories))
	for _, m := range extractionResult.Memories {
		memories = append(memories, types.MemoryEntry{
			Name:        m.Name,
			Description: m.Description,
			Type:        m.Type,
			Content:     m.Content,
			Importance:  2,
		})
	}

	return memories, nil
}

// callLLM 调用 LLM（OpenAI compatible API）
func (e *MemoryExtractor) callLLM(ctx context.Context, prompt string) (string, error) {
	// 构建请求
	reqBody := map[string]any{
		"model": e.modelConfig.Name,
		"messages": []map[string]any{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.3,
	}
	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", e.apiBase+"/chat/completions", bytes.NewBuffer(reqBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.modelConfig.APIKey)

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call LLM: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM returned status %d: %s", resp.StatusCode, string(body))
	}

	// 解析 OpenAI 响应格式
	var openAIResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &openAIResp); err != nil {
		return "", fmt.Errorf("failed to parse LLM response: %w", err)
	}

	log.Printf("[Memory LLM] raw response: %s", string(body))

	if len(openAIResp.Choices) == 0 {
		return "", fmt.Errorf("LLM returned no choices")
	}

	// 去除 markdown 代码块标记
	content := openAIResp.Choices[0].Message.Content

	// 去掉首尾空白字符
	content = strings.TrimSpace(content)

	// 移除 markdown 代码块标记（处理 ```json\n... 的情况）
	content = strings.Replace(content, "```json\n", "", 1)
	content = strings.Replace(content, "```\n", "", 1)
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	log.Printf("[Memory LLM] cleaned response: %s", content)

	return content, nil
}

// ExtractMemoriesAsync 异步提取记忆，不阻塞主流程
func (e *MemoryExtractor) ExtractMemoriesAsync(ctx context.Context, userInput, assistantOutput string, callback func([]types.MemoryEntry)) {
	go func() {
		memories, err := e.ExtractMemories(ctx, userInput, assistantOutput)
		if err != nil {
			// 忽略错误，静默完成
			return
		}
		if len(memories) > 0 {
			callback(memories)
		}
	}()
}

// GetModelConfig 从 models map 中获取默认模型的配置
func GetModelConfigForMemory(models map[string]types.ModelConfig) *types.ModelConfig {
	if models == nil {
		return nil
	}

	// 优先使用 default 模型
	if defaultModel, ok := models["default"]; ok {
		return &defaultModel
	}

	// 否则使用第一个模型
	for _, model := range models {
		return &model
	}

	return nil
}

// FormatMemoriesForLog 格式化记忆用于日志
func FormatMemoriesForLog(memories []types.MemoryEntry) string {
	if len(memories) == 0 {
		return ""
	}
	var lines []string
	for _, m := range memories {
		lines = append(lines, fmt.Sprintf("  - [%s] %s (%s)", m.Name, m.Description, m.Type))
	}
	return strings.Join(lines, "\n")
}