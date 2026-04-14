package chat

import (
	"context"

	"github.com/jettjia/igo-pkg/pkg/xsql/builder"

	entity "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/chat"
)

// IChatSessionRepo 聊天会话仓库接口
type IChatSessionRepo interface {
	Create(ctx context.Context, session *entity.ChatSession) (ulid string, err error)
	CreateWithId(ctx context.Context, session *entity.ChatSession, ulid string) error
	Delete(ctx context.Context, ulid string) error
	Update(ctx context.Context, session *entity.ChatSession) error
	UpdateStatus(ctx context.Context, ulid string, status string) error
	FindById(ctx context.Context, ulid string) (*entity.ChatSession, error)
	FindByUserId(ctx context.Context, userId string, status string) ([]*entity.ChatSession, error)
	FindPage(ctx context.Context, queries []*builder.Query, reqPage *builder.PageData, reqSort *builder.SortData) ([]*entity.ChatSession, *builder.PageData, error)
	FindRecent(ctx context.Context, limit int) ([]*entity.ChatSession, error) // 获取最近的会话
	CountByChannel(ctx context.Context) (map[string]int, error)              // 按渠道统计消息数
	FindByUserIdAndChannel(ctx context.Context, userId, channel string) (*entity.ChatSession, error)
	CountByAgent(ctx context.Context) ([]*AgentUsageItem, error)
}

// IChatMessageRepo 聊天消息仓库接口
type IChatMessageRepo interface {
	Create(ctx context.Context, message *entity.ChatMessage) (ulid string, err error)
	Update(ctx context.Context, message *entity.ChatMessage) error
	UpdateStatus(ctx context.Context, ulid string, status string) error
	FindById(ctx context.Context, ulid string) (*entity.ChatMessage, error)
	FindBySessionId(ctx context.Context, sessionId string) ([]*entity.ChatMessage, error)
	DeleteBySessionId(ctx context.Context, sessionId string) error
}

// IChatApprovalRepo 聊天审批仓库接口
type IChatApprovalRepo interface {
	Create(ctx context.Context, approval *entity.ChatApproval) (ulid string, err error)
	Update(ctx context.Context, approval *entity.ChatApproval) error
	UpdateStatus(ctx context.Context, ulid string, status, approvedBy string, reason string) error
	FindById(ctx context.Context, ulid string) (*entity.ChatApproval, error)
	FindByMessageId(ctx context.Context, messageId string) (*entity.ChatApproval, error)
	FindPending(ctx context.Context) ([]*entity.ChatApproval, error)
	FindByUserId(ctx context.Context, userId string) ([]*entity.ChatApproval, error)
}

// IChatTokenStatsRepo Token统计仓库接口
type IChatTokenStatsRepo interface {
	Create(ctx context.Context, stats *entity.ChatTokenStats) (ulid string, err error)
	Update(ctx context.Context, stats *entity.ChatTokenStats) error
	FindOrCreate(ctx context.Context, agentId, userId, date, model string) (*entity.ChatTokenStats, error)
	AddTokens(ctx context.Context, agentId, userId, date, model string, input, output int) error
	GetTotalTokens(ctx context.Context) (int, error)
	GetTokenRanking(ctx context.Context, limit int) ([]*TokenRankingItem, error)
}

// TokenRankingItem Token排行项
type TokenRankingItem struct {
	AgentId     string
	AgentName   string
	TotalTokens int
}

// AgentUsageItem 智能体使用项
type AgentUsageItem struct {
	AgentId      string
	SessionCount int
	MessageCount int
}