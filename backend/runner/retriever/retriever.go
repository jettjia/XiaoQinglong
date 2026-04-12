package retriever

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/jettjia/XiaoQinglong/runner/pkg/logger"
)

// ========== Knowledge Base Config ==========

// KnowledgeBaseConfig 知识库配置
type KnowledgeBaseConfig struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	RetrievalURL string `json:"retrieval_url"`
	Token        string `json:"token"`
	TopK         int    `json:"top_k"`
}

// ========== Knowledge Retriever ==========

// KnowledgeRetriever 知识检索器
type KnowledgeRetriever struct {
	configs    []KnowledgeBaseConfig
	httpClient *http.Client
}

// NewKnowledgeRetriever 创建知识检索器
func NewKnowledgeRetriever(configs []KnowledgeBaseConfig) *KnowledgeRetriever {
	return &KnowledgeRetriever{
		configs:    configs,
		httpClient: &http.Client{},
	}
}

// Retrieve 根据 query 从知识库检索相关内容
func (kr *KnowledgeRetriever) Retrieve(ctx context.Context, query string) ([]*schema.Document, error) {
	var docs []*schema.Document

	logger.Infof("[KnowledgeRetriever] Retrieve: query=%s, configs count=%d", query, len(kr.configs))
	for _, cfg := range kr.configs {
		logger.Infof("[KnowledgeRetriever] Retrieve: querying KB %s, url=%s", cfg.Name, cfg.RetrievalURL)
		kbDocs, err := kr.retrieveFromKB(ctx, cfg, query)
		if err != nil {
			logger.Warnf("[KnowledgeRetriever] retrieve from KB %s failed: %v", cfg.Name, err)
			continue
		}
		logger.Infof("[KnowledgeRetriever] Retrieve: got %d docs from KB %s", len(kbDocs), cfg.Name)
		docs = append(docs, kbDocs...)
	}

	return docs, nil
}

// retrieveFromKB 从单个知识库检索
func (kr *KnowledgeRetriever) retrieveFromKB(ctx context.Context, cfg KnowledgeBaseConfig, query string) ([]*schema.Document, error) {
	if cfg.RetrievalURL == "" {
		return nil, fmt.Errorf("retrieval url is empty for KB: %s", cfg.Name)
	}

	topK := cfg.TopK
	if topK <= 0 {
		topK = 5
	}

	reqBody := map[string]any{
		"query": query,
		"top_k": topK,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", cfg.RetrievalURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	}

	resp, err := kr.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("KB returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var results []kbResult
	if err := json.Unmarshal(respBody, &results); err != nil {
		var single kbResult
		if err2 := json.Unmarshal(respBody, &single); err2 == nil {
			results = []kbResult{single}
		} else {
			return nil, fmt.Errorf("parse response failed: %w", err)
		}
	}

	docs := make([]*schema.Document, 0, len(results))
	for i, r := range results {
		doc := &schema.Document{
			ID:      fmt.Sprintf("%s-%d", cfg.ID, i),
			Content: r.Content,
			MetaData: map[string]any{
				"kb_id":   cfg.ID,
				"kb_name": cfg.Name,
				"title":   r.Title,
				"score":   r.Score,
			},
		}
		docs = append(docs, doc)
	}

	return docs, nil
}

type kbResult struct {
	Title   string  `json:"title"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

// ========== Tool 包装 ==========

// FormatKnowledgeResults 将检索结果格式化为字符串
func FormatKnowledgeResults(docs []*schema.Document) string {
	if len(docs) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "# Knowledge Base")
	lines = append(lines, "Use the following information from knowledge base to answer questions:")
	lines = append(lines, "")
	for i, doc := range docs {
		lines = append(lines, fmt.Sprintf("## [%d] %s", i+1, doc.MetaData["title"]))
		lines = append(lines, fmt.Sprintf("Source: %s (score: %.2f)", doc.MetaData["kb_name"], doc.MetaData["score"]))
		lines = append(lines, "")
		lines = append(lines, doc.Content)
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// CreateRetrievalTool 创建检索工具（供 Agent 调用）
func CreateRetrievalTool(kbConfigs []KnowledgeBaseConfig) tool.BaseTool {
	kr := NewKnowledgeRetriever(kbConfigs)
	return newRetrievalTool(kr)
}

func newRetrievalTool(kr *KnowledgeRetriever) tool.BaseTool {
	return &retrievalTool{kr: kr}
}

type retrievalTool struct {
	kr *KnowledgeRetriever
}

func (t *retrievalTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "retrieve_knowledge",
		Desc: "根据用户问题从知识库检索相关内容。返回相关的文档片段，包括标题、内容和相关性评分。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"query": {Type: schema.String, Desc: "检索查询词", Required: true},
		}),
	}, nil
}

func (t *retrievalTool) InvokableRun(ctx context.Context, argumentsInJSON string, opt ...tool.Option) (string, error) {
	var args struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("parse arguments failed: %w", err)
	}

	docs, err := t.kr.Retrieve(ctx, args.Query)
	if err != nil {
		return "", fmt.Errorf("retrieve failed: %w", err)
	}

	if len(docs) == 0 {
		return "未找到相关内容", nil
	}

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("找到 %d 条相关内容：\n\n", len(docs)))
	for i, doc := range docs {
		buf.WriteString(fmt.Sprintf("【%d】%s\n", i+1, doc.MetaData["title"]))
		buf.WriteString(fmt.Sprintf("来源：%s (评分: %.2f)\n", doc.MetaData["kb_name"], doc.MetaData["score"]))
		buf.WriteString(fmt.Sprintf("内容：%s\n\n", doc.Content))
	}

	return buf.String(), nil
}
