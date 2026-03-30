package knowledge_base

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	entity "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/knowledge_base"
	repo "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/repo/knowledge_base"
)

// KnowledgeRetrievalSvc 知识检索服务
type KnowledgeRetrievalSvc struct {
	kbRepo *repo.SysKnowledgeBase
}

// NewKnowledgeRetrievalSvc NewKnowledgeRetrievalSvc
func NewKnowledgeRetrievalSvc() *KnowledgeRetrievalSvc {
	return &KnowledgeRetrievalSvc{
		kbRepo: repo.NewSysKnowledgeBaseImpl(),
	}
}

// KnowledgeRetrievalResult 检索结果
type KnowledgeRetrievalResult struct {
	Title   string  `json:"title"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
	KbName  string  `json:"kb_name"`
}

// RetrievalConfig 检索配置（从agent config解析）
type RetrievalConfig struct {
	KbId  string `json:"kb_id"`
	TopK  int    `json:"top_k"`
}

// RecallFromKnowledgeBases 从多个知识库召回
func (s *KnowledgeRetrievalSvc) RecallFromKnowledgeBases(ctx context.Context, configs []RetrievalConfig, query string) ([]KnowledgeRetrievalResult, error) {
	if len(configs) == 0 {
		return nil, nil
	}

	var allResults []KnowledgeRetrievalResult

	for _, cfg := range configs {
		results, err := s.RecallFromSingleKnowledgeBase(ctx, cfg.KbId, query, cfg.TopK)
		if err != nil {
			// 单个知识库失败不影响其他知识库
			continue
		}
		allResults = append(allResults, results...)
	}

	return allResults, nil
}

// RecallFromSingleKnowledgeBase 从单个知识库召回
func (s *KnowledgeRetrievalSvc) RecallFromSingleKnowledgeBase(ctx context.Context, kbId string, query string, topK int) ([]KnowledgeRetrievalResult, error) {
	if topK <= 0 {
		topK = 5 // 默认召回5条
	}

	// 获取知识库配置
	kb, err := s.kbRepo.FindById(ctx, kbId)
	if err != nil {
		return nil, fmt.Errorf("failed to find knowledge base: %w", err)
	}
	if kb == nil || kb.DeletedAt != 0 {
		return nil, fmt.Errorf("knowledge base not found or deleted: %s", kbId)
	}
	if !kb.Enabled {
		return nil, fmt.Errorf("knowledge base disabled: %s", kbId)
	}
	if kb.RetrievalUrl == "" {
		return nil, fmt.Errorf("retrieval url is empty for kb: %s", kbId)
	}

	// 调用检索服务
	return s.callRetrievalService(ctx, kb, query, topK)
}

// callRetrievalService 调用外部检索服务
func (s *KnowledgeRetrievalSvc) callRetrievalService(ctx context.Context, kb *entity.SysKnowledgeBase, query string, topK int) ([]KnowledgeRetrievalResult, error) {
	// 构建请求
	recallReq := map[string]interface{}{
		"query": query,
		"top_k": topK,
	}
	reqBody, err := json.Marshal(recallReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal recall request: %w", err)
	}

	// 创建HTTP请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", kb.RetrievalUrl, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// 设置Token认证
	if kb.Token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+kb.Token)
	}

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call retrieval service: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("retrieval service returned status %d: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var results []KnowledgeRetrievalResult
	if err := json.Unmarshal(body, &results); err != nil {
		// 尝试解析为单条结果
		var singleResult KnowledgeRetrievalResult
		if err2 := json.Unmarshal(body, &singleResult); err2 == nil {
			results = []KnowledgeRetrievalResult{singleResult}
		} else {
			return nil, fmt.Errorf("failed to parse retrieval response: %w", err)
		}
	}

	// 设置知识库名称
	for i := range results {
		if results[i].KbName == "" {
			results[i].KbName = kb.Name
		}
	}

	return results, nil
}

// FormatKnowledgeContext 格式化知识库上下文（用于注入到prompt）
func (s *KnowledgeRetrievalSvc) FormatKnowledgeContext(results []KnowledgeRetrievalResult) string {
	if len(results) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n【相关知识】\n")

	for i, r := range results {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, r.KbName, r.Title))
		sb.WriteString(fmt.Sprintf("   %s\n\n", r.Content))
	}

	return sb.String()
}