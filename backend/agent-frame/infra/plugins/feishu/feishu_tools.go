package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ToolGenerator 飞书工具生成器
type ToolGenerator struct {
	authHandler *AuthHandler
}

// NewToolGenerator 创建飞书工具生成器
func NewToolGenerator() *ToolGenerator {
	return &ToolGenerator{
		authHandler: NewAuthHandler(),
	}
}

// Tool 工具定义
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// GenerateTools 生成工具列表
func (g *ToolGenerator) GenerateTools(ctx context.Context) ([]*Tool, error) {
	return []*Tool{
		{
			Name:        "feishu_doc_search",
			Description: "搜索飞书云文档",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "搜索关键词",
					},
					"count": map[string]interface{}{
						"type":        "integer",
						"description": "返回数量，默认10",
						"default":     10,
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "feishu_wiki_search",
			Description: "搜索飞书知识库",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "搜索关键词",
					},
					"count": map[string]interface{}{
						"type":        "integer",
						"description": "返回数量，默认10",
						"default":     10,
					},
				},
				"required": []string{"query"},
			},
		},
	}, nil
}

// SearchDocs 搜索文档
func (g *ToolGenerator) SearchDocs(ctx context.Context, accessToken string, query string, count int) ([]*DocResult, error) {
	apiURL := "https://open.feishu.cn/open-apis/suite/docs-api/search"

	data := map[string]interface{}{
		"search_key": query,
		"count":      count,
		"docs_types": []string{"doc", "docx"},
	}
	jsonData, _ := json.Marshal(data)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var searchResp DocSearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, err
	}

	if searchResp.Code != 0 {
		return nil, fmt.Errorf("feishu doc search failed: %s", searchResp.Msg)
	}

	return searchResp.Data.Docs, nil
}

// SearchWiki 搜索知识库
func (g *ToolGenerator) SearchWiki(ctx context.Context, accessToken string, query string, count int) ([]*DocResult, error) {
	apiURL := "https://open.feishu.cn/open-apis/wiki/v2/spaces/search"

	data := map[string]interface{}{
		"query":       query,
		"count":       count,
		"search_type": 1, // 0: 未分类 1: 知识库
	}
	jsonData, _ := json.Marshal(data)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var searchResp WikiSearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, err
	}

	if searchResp.Code != 0 {
		return nil, fmt.Errorf("feishu wiki search failed: %s", searchResp.Msg)
	}

	return searchResp.Data.Docs, nil
}

// DocSearchResponse 文档搜索响应
type DocSearchResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Docs []*DocResult `json:"docs"`
	} `json:"data"`
}

// WikiSearchResponse 知识库搜索响应
type WikiSearchResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Docs []*DocResult `json:"docs"`
	} `json:"data"`
}

// DocResult 文档结果
type DocResult struct {
	DocID       string `json:"doc_id"`
	Title       string `json:"title"`
	OwnerID     string `json:"owner_id"`
	WikiSpaceID string `json:"wiki_space_id,omitempty"`
	Type        string `json:"type"`
	URL         string `json:"url"`
	UpdateTime  int64  `json:"update_time"`
}

// ValidateToken 验证token是否有效
func (g *ToolGenerator) ValidateToken(ctx context.Context, accessToken string) (bool, error) {
	if accessToken == "" {
		return false, fmt.Errorf("access token is empty")
	}

	apiURL := "https://open.feishu.cn/open-apis/authen/v1/user_info"

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return false, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200, nil
}
