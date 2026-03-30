package chat

import (
	"context"
	"time"

	"github.com/jettjia/igo-pkg/pkg/xsql/builder"

	entity "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/chat"
	repo "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/repo/chat"
)

// ChatSessionSvc chat session service
type ChatSessionSvc struct {
	sessionRepo *repo.ChatSession
	messageRepo *repo.ChatMessage
}

// NewChatSessionSvc NewChatSessionSvc
func NewChatSessionSvc() *ChatSessionSvc {
	return &ChatSessionSvc{
		sessionRepo: repo.NewChatSessionImpl(),
		messageRepo: repo.NewChatMessageImpl(),
	}
}

// CreateSession 创建会话
func (s *ChatSessionSvc) CreateSession(ctx context.Context, session *entity.ChatSession) (ulid string, err error) {
	return s.sessionRepo.Create(ctx, session)
}

// DeleteSession 删除会话
func (s *ChatSessionSvc) DeleteSession(ctx context.Context, ulid string) error {
	return s.sessionRepo.Delete(ctx, ulid)
}

// UpdateSession 更新会话
func (s *ChatSessionSvc) UpdateSession(ctx context.Context, session *entity.ChatSession) error {
	return s.sessionRepo.Update(ctx, session)
}

// UpdateSessionStatus 更新会话状态
func (s *ChatSessionSvc) UpdateSessionStatus(ctx context.Context, ulid string, status string) error {
	return s.sessionRepo.UpdateStatus(ctx, ulid, status)
}

// FindSessionById 查看会话byId
func (s *ChatSessionSvc) FindSessionById(ctx context.Context, ulid string) (*entity.ChatSession, error) {
	return s.sessionRepo.FindById(ctx, ulid)
}

// FindSessionsByUserId 查看用户的会话列表
func (s *ChatSessionSvc) FindSessionsByUserId(ctx context.Context, userId string, status string) ([]*entity.ChatSession, error) {
	return s.sessionRepo.FindByUserId(ctx, userId, status)
}

// FindSessionPage 分页查询会话
func (s *ChatSessionSvc) FindSessionPage(ctx context.Context, queries []*builder.Query, reqPage *builder.PageData, reqSort *builder.SortData) ([]*entity.ChatSession, *builder.PageData, error) {
	return s.sessionRepo.FindPage(ctx, queries, reqPage, reqSort)
}

// ====== ChatMessage ======

// CreateMessage 创建消息
func (s *ChatSessionSvc) CreateMessage(ctx context.Context, message *entity.ChatMessage) (ulid string, err error) {
	return s.messageRepo.Create(ctx, message)
}

// UpdateMessage 更新消息
func (s *ChatSessionSvc) UpdateMessage(ctx context.Context, message *entity.ChatMessage) error {
	return s.messageRepo.Update(ctx, message)
}

// UpdateMessageStatus 更新消息状态
func (s *ChatSessionSvc) UpdateMessageStatus(ctx context.Context, ulid string, status string) error {
	return s.messageRepo.UpdateStatus(ctx, ulid, status)
}

// FindMessageById 查看消息byId
func (s *ChatSessionSvc) FindMessageById(ctx context.Context, ulid string) (*entity.ChatMessage, error) {
	return s.messageRepo.FindById(ctx, ulid)
}

// FindMessagesBySessionId 查看会话的消息列表
func (s *ChatSessionSvc) FindMessagesBySessionId(ctx context.Context, sessionId string) ([]*entity.ChatMessage, error) {
	return s.messageRepo.FindBySessionId(ctx, sessionId)
}

// ====== ChatApproval ======

// ChatApprovalSvc chat approval service
type ChatApprovalSvc struct {
	approvalRepo *repo.ChatApproval
}

// NewChatApprovalSvc NewChatApprovalSvc
func NewChatApprovalSvc() *ChatApprovalSvc {
	return &ChatApprovalSvc{
		approvalRepo: repo.NewChatApprovalImpl(),
	}
}

// CreateApproval 创建审批
func (s *ChatApprovalSvc) CreateApproval(ctx context.Context, approval *entity.ChatApproval) (ulid string, err error) {
	return s.approvalRepo.Create(ctx, approval)
}

// UpdateApproval 更新审批
func (s *ChatApprovalSvc) UpdateApproval(ctx context.Context, approval *entity.ChatApproval) error {
	return s.approvalRepo.Update(ctx, approval)
}

// UpdateApprovalStatus 更新审批状态
func (s *ChatApprovalSvc) UpdateApprovalStatus(ctx context.Context, ulid string, status string, approvedBy string, reason string) error {
	return s.approvalRepo.UpdateStatus(ctx, ulid, status, approvedBy, reason)
}

// Approve 批准
func (s *ChatApprovalSvc) Approve(ctx context.Context, ulid string, approvedBy string, reason string) error {
	return s.approvalRepo.UpdateStatus(ctx, ulid, "approved", approvedBy, reason)
}

// Reject 拒绝
func (s *ChatApprovalSvc) Reject(ctx context.Context, ulid string, approvedBy string, reason string) error {
	return s.approvalRepo.UpdateStatus(ctx, ulid, "rejected", approvedBy, reason)
}

// FindApprovalById 查看审批byId
func (s *ChatApprovalSvc) FindApprovalById(ctx context.Context, ulid string) (*entity.ChatApproval, error) {
	return s.approvalRepo.FindById(ctx, ulid)
}

// FindApprovalByMessageId 查看消息的审批
func (s *ChatApprovalSvc) FindApprovalByMessageId(ctx context.Context, messageId string) (*entity.ChatApproval, error) {
	return s.approvalRepo.FindByMessageId(ctx, messageId)
}

// FindPendingApprovals 查看待审批列表
func (s *ChatApprovalSvc) FindPendingApprovals(ctx context.Context) ([]*entity.ChatApproval, error) {
	return s.approvalRepo.FindPending(ctx)
}

// FindApprovalsByUserId 查看用户的审批列表
func (s *ChatApprovalSvc) FindApprovalsByUserId(ctx context.Context, userId string) ([]*entity.ChatApproval, error) {
	return s.approvalRepo.FindByUserId(ctx, userId)
}

// ====== ChatTokenStats ======

// ChatTokenStatsSvc token统计服务
type ChatTokenStatsSvc struct {
	statsRepo *repo.ChatTokenStats
}

// NewChatTokenStatsSvc NewChatTokenStatsSvc
func NewChatTokenStatsSvc() *ChatTokenStatsSvc {
	return &ChatTokenStatsSvc{
		statsRepo: repo.NewChatTokenStatsImpl(),
	}
}

// CreateStats 创建统计
func (s *ChatTokenStatsSvc) CreateStats(ctx context.Context, stats *entity.ChatTokenStats) (ulid string, err error) {
	return s.statsRepo.Create(ctx, stats)
}

// UpdateStats 更新统计
func (s *ChatTokenStatsSvc) UpdateStats(ctx context.Context, stats *entity.ChatTokenStats) error {
	return s.statsRepo.Update(ctx, stats)
}

// FindOrCreateStats 查询或创建统计
func (s *ChatTokenStatsSvc) FindOrCreateStats(ctx context.Context, agentId, userId, model string) (*entity.ChatTokenStats, error) {
	date := time.Now().Format("2006-01-02")
	return s.statsRepo.FindOrCreate(ctx, agentId, userId, date, model)
}

// AddTokens 添加token
func (s *ChatTokenStatsSvc) AddTokens(ctx context.Context, agentId, userId, model string, input, output int) error {
	date := time.Now().Format("2006-01-02")
	return s.statsRepo.AddTokens(ctx, agentId, userId, date, model, input, output)
}