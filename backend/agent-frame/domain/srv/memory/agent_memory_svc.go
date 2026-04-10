package memory

import (
	"context"
	"strings"

	entity "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/memory"
	po "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/memory"
	repo "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/repo/memory"
)

// AgentMemorySvc 智能体记忆服务
type AgentMemorySvc struct {
	memoryRepo *repo.AgentMemoryRepo
}

// NewAgentMemorySvc NewAgentMemorySvc
func NewAgentMemorySvc() *AgentMemorySvc {
	return &AgentMemorySvc{
		memoryRepo: repo.NewAgentMemoryRepo(),
	}
}

// CreateMemory 创建记忆
func (s *AgentMemorySvc) CreateMemory(ctx context.Context, memory *entity.AgentMemory) (ulid string, err error) {
	return s.memoryRepo.Create(ctx, memory)
}

// DeleteMemory 删除记忆
func (s *AgentMemorySvc) DeleteMemory(ctx context.Context, ulid string) error {
	return s.memoryRepo.Delete(ctx, ulid)
}

// UpdateMemory 更新记忆
func (s *AgentMemorySvc) UpdateMemory(ctx context.Context, memory *entity.AgentMemory) error {
	return s.memoryRepo.Update(ctx, memory)
}

// FindMemoryById 查看记忆byId
func (s *AgentMemorySvc) FindMemoryById(ctx context.Context, ulid string) (*entity.AgentMemory, error) {
	return s.memoryRepo.FindById(ctx, ulid)
}

// FindMemoriesByAgentAndUser 查看智能体和用户的记忆
func (s *AgentMemorySvc) FindMemoriesByAgentAndUser(ctx context.Context, agentId, userId string) ([]*entity.AgentMemory, error) {
	return s.memoryRepo.FindByAgentAndUser(ctx, agentId, userId)
}

// FindMemoriesBySession 查看某个 session 的记忆（按创建时间排序）
func (s *AgentMemorySvc) FindMemoriesBySession(ctx context.Context, sessionId string) ([]*entity.AgentMemory, error) {
	return s.memoryRepo.FindBySession(ctx, sessionId)
}

// SearchMemories 搜索记忆（基于关键词）
func (s *AgentMemorySvc) SearchMemories(ctx context.Context, agentId, userId string, query string) ([]*entity.AgentMemory, error) {
	// 简单方案：使用关键词匹配
	// 高级方案：可以使用向量数据库做语义检索
	return s.memoryRepo.SearchByKeywords(ctx, agentId, userId, query)
}

// GetRelevantMemories 获取相关记忆（用于注入到context）
func (s *AgentMemorySvc) GetRelevantMemories(ctx context.Context, agentId, userId, query string, maxCount int) ([]*entity.AgentMemory, error) {
	memories, err := s.memoryRepo.FindByAgentAndUser(ctx, agentId, userId)
	if err != nil {
		return nil, err
	}

	if len(memories) == 0 {
		return memories, nil
	}

	// 如果没有query关键词，返回最近的记忆
	if query == "" {
		if len(memories) > maxCount {
			return memories[:maxCount], nil
		}
		return memories, nil
	}

	// 简单相关性过滤：包含关键词的记忆优先
	queryKeywords := strings.ToLower(query)
	relevant := make([]*entity.AgentMemory, 0)
	others := make([]*entity.AgentMemory, 0)

	for _, m := range memories {
		keywords := strings.ToLower(m.Keywords)
		content := strings.ToLower(m.Content)

		// 优先：关键词匹配
		if strings.Contains(keywords, queryKeywords) || strings.Contains(content, queryKeywords) {
			relevant = append(relevant, m)
		} else {
			others = append(others, m)
		}
	}

	// 合并结果：先放相关的，再放其他的，按重要性排序
	result := make([]*entity.AgentMemory, 0, len(memories))
	result = append(result, relevant...)
	result = append(result, others...)

	if len(result) > maxCount {
		return result[:maxCount], nil
	}
	return result, nil
}

// ExtractAndSaveMemories 从对话中提取并保存记忆
func (s *AgentMemorySvc) ExtractAndSaveMemories(ctx context.Context, agentId, userId, sessionId, userInput, assistantOutput string) error {
	// 简单方案：从用户输入和助手输出中提取关键词作为记忆
	// 高级方案：LLM生成结构化记忆

	// 1. 提取用户提到的实体（简单分词）
	words := extractSimpleEntities(userInput)

	for _, word := range words {
		if len(word) < 2 {
			continue
		}
		memory := &entity.AgentMemory{
			AgentId:    agentId,
			UserId:     userId,
			SessionId:  sessionId,
			MemoryType: "entity",
			Content:    word,
			Keywords:   word,
			Importance: 1,
		}
		s.memoryRepo.Create(ctx, memory)
	}

	return nil
}

// extractSimpleEntities 简单提取实体（分词）
func extractSimpleEntities(text string) []string {
	// 简化方案：按空格和标点分割
	// 实际应该用中文分词库如 go-jieb
	var words []string
	var current []rune

	for _, r := range text {
		if r == ' ' || r == ',' || r == '.' || r == '，' || r == '。' || r == '\n' || r == '\t' {
			if len(current) > 0 {
				words = append(words, string(current))
				current = nil
			}
		} else {
			current = append(current, r)
		}
	}

	if len(current) > 0 {
		words = append(words, string(current))
	}

	return words
}

// DeleteUserMemories 删除用户的所有记忆
func (s *AgentMemorySvc) DeleteUserMemories(ctx context.Context, userId string) error {
	return s.memoryRepo.DeleteByUser(ctx, userId)
}

// FindMemoriesByType 按类型查询记忆
func (s *AgentMemorySvc) FindMemoriesByType(ctx context.Context, agentId, userId, memoryType string) ([]*entity.AgentMemory, error) {
	return s.memoryRepo.FindByType(ctx, agentId, userId, memoryType)
}

// CreateMemoryWithIndex 创建记忆并更新索引
func (s *AgentMemorySvc) CreateMemoryWithIndex(ctx context.Context, memory *entity.AgentMemory) error {
	return s.memoryRepo.CreateWithIndex(ctx, memory)
}

// DeleteMemoryWithIndex 删除记忆并删除索引
func (s *AgentMemorySvc) DeleteMemoryWithIndex(ctx context.Context, ulid string) error {
	return s.memoryRepo.DeleteWithIndex(ctx, ulid)
}

// GetMemoryIndex 获取记忆索引（用于构建 prompt）
func (s *AgentMemorySvc) GetMemoryIndex(ctx context.Context, agentId, userId string) ([]*po.MemoryIndex, error) {
	return s.memoryRepo.GetMemoryIndex(ctx, agentId, userId)
}