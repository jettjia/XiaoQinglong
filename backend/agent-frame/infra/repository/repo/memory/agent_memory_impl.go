package memory

import (
	"context"
	"strings"
	"time"

	"github.com/jettjia/igo-pkg/pkg/data"
	"github.com/jettjia/igo-pkg/pkg/util"
	"gorm.io/gorm"

	entity "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/memory"
	irepository "github.com/jettjia/xiaoqinglong/agent-frame/domain/irepository/memory"
	"github.com/jettjia/xiaoqinglong/agent-frame/infra/pkg/idata"
	po "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/memory"
)

var _ irepository.IAgentMemoryRepo = (*AgentMemoryRepo)(nil)

type AgentMemoryRepo struct {
	data *data.Data
}

func NewAgentMemoryRepo() *AgentMemoryRepo {
	return &AgentMemoryRepo{data: idata.NewDataOptionCli()}
}

func (r *AgentMemoryRepo) Create(ctx context.Context, memory *entity.AgentMemory) (ulid string, err error) {
	ulid = util.Ulid()
	now := time.Now().UnixMilli()

	values := map[string]interface{}{
		"ulid":        ulid,
		"agent_id":    memory.AgentId,
		"user_id":     memory.UserId,
		"session_id":  memory.SessionId,
		"memory_type": memory.MemoryType,
		"name":        memory.Name,
		"description": memory.Description,
		"content":     memory.Content,
		"keywords":    memory.Keywords,
		"importance":  memory.Importance,
		"source":      memory.Source,
		"source_msg":  memory.SourceMsgId,
		"expires_at":  memory.ExpiresAt,
		"created_at":  now,
		"updated_at":  now,
		"deleted_at":  0,
	}

	if err = r.data.DB(ctx).Table("agent_memory").Create(values).Error; err != nil {
		return
	}
	return ulid, nil
}

func (r *AgentMemoryRepo) Delete(ctx context.Context, ulid string) error {
	return r.data.DB(ctx).Model(&po.AgentMemory{}).Where("ulid = ?", ulid).Updates(map[string]interface{}{
		"deleted_at": time.Now().UnixMilli(),
	}).Error
}

func (r *AgentMemoryRepo) Update(ctx context.Context, memory *entity.AgentMemory) error {
	now := time.Now().UnixMilli()
	values := map[string]interface{}{
		"agent_id":    memory.AgentId,
		"user_id":     memory.UserId,
		"session_id":  memory.SessionId,
		"memory_type": memory.MemoryType,
		"name":        memory.Name,
		"description": memory.Description,
		"content":     memory.Content,
		"keywords":    memory.Keywords,
		"importance":  memory.Importance,
		"source":      memory.Source,
		"updated_at":  now,
	}
	if memory.ExpiresAt > 0 {
		values["expires_at"] = memory.ExpiresAt
	}
	return r.data.DB(ctx).Model(&po.AgentMemory{}).Where("ulid = ?", memory.Ulid).Updates(values).Error
}

func (r *AgentMemoryRepo) FindById(ctx context.Context, ulid string) (*entity.AgentMemory, error) {
	var memoryPo po.AgentMemory
	if err := r.data.DB(ctx).Where("ulid = ? AND deleted_at = 0", ulid).First(&memoryPo).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return poToEntity(&memoryPo), nil
}

func (r *AgentMemoryRepo) FindByAgentAndUser(ctx context.Context, agentId, userId string) ([]*entity.AgentMemory, error) {
	var memoryPos []*po.AgentMemory
	query := r.data.DB(ctx).Where("agent_id = ? AND user_id = ? AND deleted_at = 0 AND (expires_at = 0 OR expires_at > ?)", agentId, userId, time.Now().UnixMilli())
	if err := query.Order("importance DESC, updated_at DESC").Find(&memoryPos).Error; err != nil {
		return nil, err
	}
	return poToEntities(memoryPos), nil
}

func (r *AgentMemoryRepo) SearchByKeywords(ctx context.Context, agentId, userId, keywords string) ([]*entity.AgentMemory, error) {
	var memoryPos []*po.AgentMemory
	// 将关键词分割并构建LIKE条件
	keywordList := strings.Split(keywords, ",")
	likeConditions := make([]string, 0)
	args := make([]interface{}, 0)
	for _, kw := range keywordList {
		kw = strings.TrimSpace(kw)
		if kw != "" {
			likeConditions = append(likeConditions, "keywords LIKE ?")
			args = append(args, "%"+kw+"%")
		}
	}

	if len(likeConditions) == 0 {
		return nil, nil
	}

	conditionStr := strings.Join(likeConditions, " OR ")
	args = append([]interface{}{agentId, userId, time.Now().UnixMilli()}, args...)

	query := r.data.DB(ctx).Where("agent_id = ? AND user_id = ? AND deleted_at = 0 AND (expires_at = 0 OR expires_at > ?) AND ("+conditionStr+")", args...)
	if err := query.Order("importance DESC, updated_at DESC").Find(&memoryPos).Error; err != nil {
		return nil, err
	}
	return poToEntities(memoryPos), nil
}

func (r *AgentMemoryRepo) FindRecent(ctx context.Context, agentId, userId string, limit int) ([]*entity.AgentMemory, error) {
	var memoryPos []*po.AgentMemory
	query := r.data.DB(ctx).Where("agent_id = ? AND user_id = ? AND deleted_at = 0 AND (expires_at = 0 OR expires_at > ?)", agentId, userId, time.Now().UnixMilli())
	if err := query.Order("updated_at DESC").Limit(limit).Find(&memoryPos).Error; err != nil {
		return nil, err
	}
	return poToEntities(memoryPos), nil
}

func (r *AgentMemoryRepo) DeleteByUser(ctx context.Context, userId string) error {
	return r.data.DB(ctx).Model(&po.AgentMemory{}).Where("user_id = ?", userId).Updates(map[string]interface{}{
		"deleted_at": time.Now().UnixMilli(),
	}).Error
}

func (r *AgentMemoryRepo) FindByType(ctx context.Context, agentId, userId, memoryType string) ([]*entity.AgentMemory, error) {
	var memoryPos []*po.AgentMemory
	query := r.data.DB(ctx).Where("agent_id = ? AND user_id = ? AND memory_type = ? AND deleted_at = 0 AND (expires_at = 0 OR expires_at > ?)", agentId, userId, memoryType, time.Now().UnixMilli())
	if err := query.Order("importance DESC, updated_at DESC").Find(&memoryPos).Error; err != nil {
		return nil, err
	}
	return poToEntities(memoryPos), nil
}

func (r *AgentMemoryRepo) CreateWithIndex(ctx context.Context, memory *entity.AgentMemory) error {
	// 开启事务
	return r.data.DB(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 创建记忆
		ulid := util.Ulid()
		now := time.Now().UnixMilli()
		values := map[string]interface{}{
			"ulid":        ulid,
			"agent_id":    memory.AgentId,
			"user_id":     memory.UserId,
			"session_id":  memory.SessionId,
			"memory_type": memory.MemoryType,
			"name":        memory.Name,
			"description": memory.Description,
			"content":     memory.Content,
			"keywords":    memory.Keywords,
			"importance":  memory.Importance,
			"source":      memory.Source,
			"source_msg":  memory.SourceMsgId,
			"expires_at":  memory.ExpiresAt,
			"created_at":  now,
			"updated_at":  now,
			"deleted_at":  0,
		}
		if err := tx.Table("agent_memory").Create(values).Error; err != nil {
			return err
		}

		// 2. 创建索引
		hookLine := po.BuildHookLine(ulid, memory.Name, memory.Description)
		indexValues := map[string]interface{}{
			"ulid":        util.Ulid(),
			"memory_id":   ulid,
			"hook_line":   hookLine,
			"memory_type": memory.MemoryType,
			"agent_id":    memory.AgentId,
			"user_id":     memory.UserId,
			"created_at":  now,
			"updated_at":  now,
		}
		return tx.Table("memory_index").Create(indexValues).Error
	})
}

func (r *AgentMemoryRepo) DeleteWithIndex(ctx context.Context, ulid string) error {
	return r.data.DB(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 软删除记忆
		if err := tx.Model(&po.AgentMemory{}).Where("ulid = ?", ulid).Updates(map[string]interface{}{
			"deleted_at": time.Now().UnixMilli(),
		}).Error; err != nil {
			return err
		}
		// 2. 删除索引
		return tx.Where("memory_id = ?", ulid).Delete(&po.MemoryIndex{}).Error
	})
}

func (r *AgentMemoryRepo) GetMemoryIndex(ctx context.Context, agentId, userId string) ([]*po.MemoryIndex, error) {
	var indices []*po.MemoryIndex
	err := r.data.DB(ctx).Where("agent_id = ? AND user_id = ?", agentId, userId).
		Order("updated_at DESC").Limit(200). // 限制最多 200 条，和 Claude Code 的 MEMORY.md 一致
		Find(&indices).Error
	return indices, err
}

// poToEntity converts PO to Entity
func poToEntity(po *po.AgentMemory) *entity.AgentMemory {
	return &entity.AgentMemory{
		Ulid:        po.Ulid,
		CreatedAt:   po.CreatedAt,
		UpdatedAt:   po.UpdatedAt,
		DeletedAt:   po.DeletedAt,
		AgentId:     po.AgentId,
		UserId:      po.UserId,
		SessionId:   po.SessionId,
		MemoryType:  po.MemoryType,
		Name:        po.Name,
		Description: po.Description,
		Content:     po.Content,
		Keywords:    po.Keywords,
		Importance:  po.Importance,
		Source:      po.Source,
		SourceMsgId: po.SourceMsgId,
		ExpiresAt:   po.ExpiresAt,
	}
}

// poToEntities converts PO list to Entity list
func poToEntities(pos []*po.AgentMemory) []*entity.AgentMemory {
	result := make([]*entity.AgentMemory, 0, len(pos))
	for _, p := range pos {
		result = append(result, poToEntity(p))
	}
	return result
}