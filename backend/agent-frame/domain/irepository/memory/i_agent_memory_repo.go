package memory

import (
	"context"

	entity "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/memory"
)

// IAgentMemoryRepo 智能体记忆仓库接口
type IAgentMemoryRepo interface {
	Create(ctx context.Context, memory *entity.AgentMemory) (ulid string, err error)
	Delete(ctx context.Context, ulid string) error
	Update(ctx context.Context, memory *entity.AgentMemory) error
	FindById(ctx context.Context, ulid string) (*entity.AgentMemory, error)
	// 按agent和用户查找记忆
	FindByAgentAndUser(ctx context.Context, agentId, userId string) ([]*entity.AgentMemory, error)
	// 关键词检索记忆
	SearchByKeywords(ctx context.Context, agentId, userId, keywords string) ([]*entity.AgentMemory, error)
	// 查找最近的记忆
	FindRecent(ctx context.Context, agentId, userId string, limit int) ([]*entity.AgentMemory, error)
	// 删除用户的所有记忆
	DeleteByUser(ctx context.Context, userId string) error
}