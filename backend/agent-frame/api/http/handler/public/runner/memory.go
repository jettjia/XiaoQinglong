package runner

import (
	"context"

	"github.com/gin-gonic/gin"
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
			logger.GetRunnerLogger().WithError(err).Errorf("Failed to create memory with index: name=%s, err=%v", memory.Name, err)
		} else {
			logger.GetRunnerLogger().Infof("Memory with index created: name=%s, memoryType=%s", memory.Name, memory.MemoryType)
		}
	}
	return nil
}

// SaveMemoriesHandler 处理runner回调保存记忆的请求
func (h *Handler) SaveMemoriesHandler(c *gin.Context) {
	log := logger.GetRunnerLogger()
	log.Infof("[Memory Callback] Received request, RemoteAddr=%s", c.Request.RemoteAddr)

	var req struct {
		AgentID   string `json:"agent_id"`
		UserID    string `json:"user_id"`
		SessionID string `json:"session_id"`
		Memories  []any  `json:"memories"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf("[Memory Callback] Failed to parse request: %v", err)
		c.JSON(400, gin.H{"error": "failed to parse request: " + err.Error()})
		return
	}

	log.Infof("[Memory Callback] Received %d memories, agentID=%s, sessionID=%s", len(req.Memories), req.AgentID, req.SessionID)
	for i, mem := range req.Memories {
		if memMap, ok := mem.(map[string]any); ok {
			log.Infof("[Memory Callback]   [%d] name=%v, type=%v, description=%v, content=%v",
				i, memMap["name"], memMap["type"], memMap["description"], memMap["content"])
		}
	}

	if len(req.Memories) == 0 {
		c.JSON(200, gin.H{"saved": 0})
		return
	}

	if err := h.saveMemoriesFromRunner(c.Request.Context(), req.AgentID, req.UserID, req.SessionID, req.Memories); err != nil {
		log.WithError(err).Error("Failed to save memories from runner")
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	log.Infof("[Memory Callback] Saved %d memories successfully", len(req.Memories))
	c.JSON(200, gin.H{"saved": len(req.Memories)})
}

