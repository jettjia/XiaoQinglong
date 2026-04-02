package runner

import (
	"context"

	memoryEntity "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/memory"
	"github.com/jettjia/xiaoqinglong/agent-frame/pkg/logger"
)

// loadMemoryContext 加载长期记忆上下文
func (h *Handler) loadMemoryContext(ctx context.Context, agentId, userId, query string, agentConfig map[string]any) ([]map[string]any, error) {
	// 检查是否启用长期记忆
	enableMemory := false
	maxMemoryCount := 5

	if memConfig, ok := agentConfig["long_term_memory"].(map[string]any); ok {
		if enabled, ok := memConfig["enabled"].(bool); ok {
			enableMemory = enabled
		}
		if count, ok := memConfig["max_count"].(float64); ok {
			maxMemoryCount = int(count)
		}
	}

	if !enableMemory {
		return nil, nil
	}

	// 获取相关记忆
	memories, err := h.memorySvc.GetRelevantMemories(ctx, agentId, userId, query, maxMemoryCount)
	if err != nil {
		return nil, err
	}

	if len(memories) == 0 {
		return nil, nil
	}

	// 构建记忆上下文消息
	result := make([]map[string]any, 0, len(memories))
	for _, m := range memories {
		result = append(result, map[string]any{
			"role":    "system",
			"content": "[记忆] " + m.Content,
		})
	}

	return result, nil
}

// saveMemoriesFromRunner 保存 runner 返回的记忆
func (h *Handler) saveMemoriesFromRunner(ctx context.Context, agentId, userId, sessionId string, memoriesRaw []any) error {
	for _, m := range memoriesRaw {
		memMap, ok := m.(map[string]any)
		if !ok {
			continue
		}
		memory := &memoryEntity.AgentMemory{
			AgentId:     agentId,
			UserId:      userId,
			SessionId:   sessionId,
			Name:        getStringFromMap(memMap, "name"),
			Description: getStringFromMap(memMap, "description"),
			MemoryType:  getStringFromMap(memMap, "type", "user"),
			Content:     getStringFromMap(memMap, "content"),
			Importance:  2,
		}
		if err := h.memorySvc.CreateMemoryWithIndex(ctx, memory); err != nil {
			logger.GetRunnerLogger().WithError(err).Errorf("Failed to create memory: %s", memory.Name)
		}
	}
	return nil
}
