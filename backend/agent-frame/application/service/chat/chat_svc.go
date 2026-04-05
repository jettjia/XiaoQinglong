package chat

import (
	"context"
	"os"
	"path/filepath"

	"github.com/jettjia/igo-pkg/pkg/xerror"

	ass "github.com/jettjia/xiaoqinglong/agent-frame/application/assembler/chat"
	dto "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/chat"
	srv "github.com/jettjia/xiaoqinglong/agent-frame/domain/srv/chat"
	"github.com/jettjia/xiaoqinglong/agent-frame/pkg/logger"
	"github.com/jettjia/xiaoqinglong/agent-frame/types/apierror"
)

// ChatSessionService chat session application service
type ChatSessionService struct {
	chatDto    *ass.ChatAssembler
	sessionSrv *srv.ChatSessionSvc
	messageSrv *srv.ChatSessionSvc
}

// NewChatSessionService NewChatSessionService
func NewChatSessionService() *ChatSessionService {
	return &ChatSessionService{
		chatDto:    ass.NewChatAssembler(),
		sessionSrv: srv.NewChatSessionSvc(),
		messageSrv: srv.NewChatSessionSvc(),
	}
}

// CreateChatSession 创建会话
func (s *ChatSessionService) CreateChatSession(ctx context.Context, req *dto.CreateChatSessionReq) (*dto.CreateChatSessionRsp, error) {
	en := s.chatDto.D2ECreateChatSession(req)

	ulid, err := s.sessionSrv.CreateSession(ctx, en)
	if err != nil {
		return nil, err
	}

	return &dto.CreateChatSessionRsp{Ulid: ulid}, nil
}

// DeleteChatSession 删除会话
func (s *ChatSessionService) DeleteChatSession(ctx context.Context, req *dto.DelChatSessionReq) error {
	en := s.chatDto.D2EDeleteChatSession(req)

	// 删除会话关联的上传文件
	sessionID := en.Ulid
	uploadsDir := os.Getenv("APP_DATA")
	if uploadsDir == "" {
		uploadsDir = "/tmp/xiaoqinglong/data"
	}
	sessionUploadsDir := filepath.Join(uploadsDir, "uploads", sessionID)
	logger.GetRunnerLogger().Infof("[ChatSessionService] Delete session files: sessionID=%s, path=%s", sessionID, sessionUploadsDir)
	os.RemoveAll(sessionUploadsDir)

	return s.sessionSrv.DeleteSession(ctx, en.Ulid)
}

// UpdateChatSession 更新会话
func (s *ChatSessionService) UpdateChatSession(ctx context.Context, req *dto.UpdateChatSessionReq) error {
	en := s.chatDto.D2EUpdateChatSession(req)
	return s.sessionSrv.UpdateSession(ctx, en)
}

// UpdateChatSessionStatus 更新会话状态
func (s *ChatSessionService) UpdateChatSessionStatus(ctx context.Context, req *dto.UpdateChatSessionStatusReq) error {
	return s.sessionSrv.UpdateSessionStatus(ctx, req.Ulid, req.Status)
}

// FindChatSessionById 查看会话byId
func (s *ChatSessionService) FindChatSessionById(ctx context.Context, req *dto.FindChatSessionByIdReq) (*dto.ChatSessionRsp, error) {
	en, err := s.sessionSrv.FindSessionById(ctx, req.Ulid)
	if err != nil {
		return nil, err
	}

	if en == nil || en.DeletedAt != 0 {
		return nil, xerror.NewErrorOpt(apierror.NotFoundErr, xerror.WithCause("session not found or deleted"))
	}

	return s.chatDto.E2DChatSession(en), nil
}

// FindChatSessionsByUserId 查看用户的会话列表
func (s *ChatSessionService) FindChatSessionsByUserId(ctx context.Context, req *dto.FindChatSessionsByUserIdReq) ([]*dto.ChatSessionRsp, error) {
	ens, err := s.sessionSrv.FindSessionsByUserId(ctx, req.UserId, req.Status)
	if err != nil {
		return nil, err
	}

	return s.chatDto.E2DChatSessions(ens), nil
}

// FindChatSessionPage 分页查询会话
func (s *ChatSessionService) FindChatSessionPage(ctx context.Context, req *dto.FindChatSessionPageReq) (*dto.FindChatSessionPageRsp, error) {
	ens, pageData, err := s.sessionSrv.FindSessionPage(ctx, req.Query, req.PageData, req.SortData)
	if err != nil {
		return nil, err
	}

	entries := s.chatDto.E2DChatSessions(ens)
	return &dto.FindChatSessionPageRsp{
		Entries:  entries,
		PageData: pageData,
	}, nil
}

//////////////////////////////////////////////////////////////////

// FindOrCreateChatSessionByChannel 查找或创建渠道会话
func (s *ChatSessionService) FindOrCreateChatSessionByChannel(ctx context.Context, userId, channel, agentId string) (*dto.ChatSessionRsp, error) {
	en, err := s.sessionSrv.FindOrCreateSessionByChannel(ctx, userId, channel, agentId)
	if err != nil {
		return nil, err
	}

	return s.chatDto.E2DChatSession(en), nil
}

//////////////////////////////////////////////////////////////////

// ChatMessageService chat message application service
type ChatMessageService struct {
	chatDto    *ass.ChatAssembler
	sessionSrv *srv.ChatSessionSvc
	statsSrv   *srv.ChatTokenStatsSvc
}

// NewChatMessageService NewChatMessageService
func NewChatMessageService() *ChatMessageService {
	return &ChatMessageService{
		chatDto:    ass.NewChatAssembler(),
		sessionSrv: srv.NewChatSessionSvc(),
		statsSrv:   srv.NewChatTokenStatsSvc(),
	}
}

// CreateChatMessage 创建消息
func (s *ChatMessageService) CreateChatMessage(ctx context.Context, req *dto.CreateChatMessageReq) (*dto.CreateChatMessageRsp, error) {
	en := s.chatDto.D2ECreateChatMessage(req)

	ulid, err := s.sessionSrv.CreateMessage(ctx, en)
	if err != nil {
		return nil, err
	}

	// Record token stats for assistant messages
	if req.Role == "assistant" && (req.InputTokens > 0 || req.OutputTokens > 0 || req.TotalTokens > 0) {
		// Get agentId and userId from session
		session, err := s.sessionSrv.FindSessionById(ctx, req.SessionId)
		if err == nil && session != nil {
			input := req.InputTokens
			output := req.OutputTokens
			if req.TotalTokens > 0 && input == 0 && output == 0 {
				output = req.TotalTokens
			}
			_ = s.statsSrv.AddTokens(ctx, session.AgentId, session.UserId, req.Model, input, output)
		}
	}

	return &dto.CreateChatMessageRsp{Ulid: ulid}, nil
}

// UpdateChatMessage 更新消息
func (s *ChatMessageService) UpdateChatMessage(ctx context.Context, req *dto.UpdateChatMessageReq) error {
	en := s.chatDto.D2EUpdateChatMessage(req)
	return s.sessionSrv.UpdateMessage(ctx, en)
}

// UpdateChatMessageStatus 更新消息状态
func (s *ChatMessageService) UpdateChatMessageStatus(ctx context.Context, req *dto.UpdateChatMessageStatusReq) error {
	return s.sessionSrv.UpdateMessageStatus(ctx, req.Ulid, req.Status)
}

// FindChatMessageById 查看消息byId
func (s *ChatMessageService) FindChatMessageById(ctx context.Context, req *dto.FindChatMessageByIdReq) (*dto.ChatMessageRsp, error) {
	en, err := s.sessionSrv.FindMessageById(ctx, req.Ulid)
	if err != nil {
		return nil, err
	}

	if en == nil {
		return nil, xerror.NewErrorOpt(apierror.NotFoundErr, xerror.WithCause("message not found"))
	}

	return s.chatDto.E2DChatMessage(en), nil
}

// FindChatMessagesBySessionId 查看会话的消息列表
func (s *ChatMessageService) FindChatMessagesBySessionId(ctx context.Context, req *dto.FindChatMessagesBySessionIdReq) ([]*dto.ChatMessageRsp, error) {
	ens, err := s.sessionSrv.FindMessagesBySessionId(ctx, req.SessionId)
	if err != nil {
		return nil, err
	}

	return s.chatDto.E2DChatMessages(ens), nil
}

// UpdateChatSession 更新会话
func (s *ChatMessageService) UpdateChatSession(ctx context.Context, req *dto.UpdateChatSessionReq) error {
	en := s.chatDto.D2EUpdateChatSession(req)
	return s.sessionSrv.UpdateSession(ctx, en)
}

// FindOrCreateChatSessionByChannel 查找或创建渠道会话
func (s *ChatMessageService) FindOrCreateChatSessionByChannel(ctx context.Context, userId, channel, agentId string) (*dto.ChatSessionRsp, error) {
	en, err := s.sessionSrv.FindOrCreateSessionByChannel(ctx, userId, channel, agentId)
	if err != nil {
		return nil, err
	}

	return s.chatDto.E2DChatSession(en), nil
}

//////////////////////////////////////////////////////////////////

// ChatApprovalService chat approval application service
type ChatApprovalService struct {
	chatDto     *ass.ChatAssembler
	approvalSrv *srv.ChatApprovalSvc
}

// NewChatApprovalService NewChatApprovalService
func NewChatApprovalService() *ChatApprovalService {
	return &ChatApprovalService{
		chatDto:     ass.NewChatAssembler(),
		approvalSrv: srv.NewChatApprovalSvc(),
	}
}

// CreateChatApproval 创建审批
func (s *ChatApprovalService) CreateChatApproval(ctx context.Context, req *dto.CreateChatApprovalReq) (*dto.CreateChatApprovalRsp, error) {
	en := s.chatDto.D2ECreateChatApproval(req)

	ulid, err := s.approvalSrv.CreateApproval(ctx, en)
	if err != nil {
		return nil, err
	}

	return &dto.CreateChatApprovalRsp{Ulid: ulid}, nil
}

// ApproveChatApproval 批准审批
func (s *ChatApprovalService) ApproveChatApproval(ctx context.Context, req *dto.ApproveChatApprovalReq) error {
	return s.approvalSrv.Approve(ctx, req.Ulid, req.ApprovedBy, req.Reason)
}

// RejectChatApproval 拒绝审批
func (s *ChatApprovalService) RejectChatApproval(ctx context.Context, req *dto.RejectChatApprovalReq) error {
	return s.approvalSrv.Reject(ctx, req.Ulid, req.ApprovedBy, req.Reason)
}

// FindChatApprovalById 查看审批byId
func (s *ChatApprovalService) FindChatApprovalById(ctx context.Context, req *dto.FindChatApprovalByIdReq) (*dto.ChatApprovalRsp, error) {
	en, err := s.approvalSrv.FindApprovalById(ctx, req.Ulid)
	if err != nil {
		return nil, err
	}

	if en == nil {
		return nil, xerror.NewErrorOpt(apierror.NotFoundErr, xerror.WithCause("approval not found"))
	}

	return s.chatDto.E2DChatApproval(en), nil
}

// FindChatApprovalByMessageId 查看消息的审批
func (s *ChatApprovalService) FindChatApprovalByMessageId(ctx context.Context, req *dto.FindChatApprovalByMessageIdReq) (*dto.ChatApprovalRsp, error) {
	en, err := s.approvalSrv.FindApprovalByMessageId(ctx, req.MessageId)
	if err != nil {
		return nil, err
	}

	if en == nil {
		return nil, xerror.NewErrorOpt(apierror.NotFoundErr, xerror.WithCause("approval not found"))
	}

	return s.chatDto.E2DChatApproval(en), nil
}

// FindPendingChatApprovals 查看待审批列表
func (s *ChatApprovalService) FindPendingChatApprovals(ctx context.Context, req *dto.FindPendingChatApprovalsReq) ([]*dto.ChatApprovalRsp, error) {
	ens, err := s.approvalSrv.FindPendingApprovals(ctx)
	if err != nil {
		return nil, err
	}

	return s.chatDto.E2DChatApprovals(ens), nil
}

// FindChatApprovalsByUserId 查看用户的审批列表
func (s *ChatApprovalService) FindChatApprovalsByUserId(ctx context.Context, req *dto.FindChatApprovalsByUserIdReq) ([]*dto.ChatApprovalRsp, error) {
	ens, err := s.approvalSrv.FindApprovalsByUserId(ctx, req.UserId)
	if err != nil {
		return nil, err
	}

	return s.chatDto.E2DChatApprovals(ens), nil
}

//////////////////////////////////////////////////////////////////

// ChatTokenStatsService token统计 application service
type ChatTokenStatsService struct {
	chatDto  *ass.ChatAssembler
	statsSrv *srv.ChatTokenStatsSvc
}

// NewChatTokenStatsService NewChatTokenStatsService
func NewChatTokenStatsService() *ChatTokenStatsService {
	return &ChatTokenStatsService{
		chatDto:  ass.NewChatAssembler(),
		statsSrv: srv.NewChatTokenStatsSvc(),
	}
}

// FindOrCreateChatTokenStats 查询或创建Token统计
func (s *ChatTokenStatsService) FindOrCreateChatTokenStats(ctx context.Context, agentId, userId, model string) (*dto.ChatTokenStatsRsp, error) {
	en, err := s.statsSrv.FindOrCreateStats(ctx, agentId, userId, model)
	if err != nil {
		return nil, err
	}

	return s.chatDto.E2DChatTokenStats(en), nil
}

// AddChatTokens 添加Token
func (s *ChatTokenStatsService) AddChatTokens(ctx context.Context, agentId, userId, model string, input, output int) error {
	return s.statsSrv.AddTokens(ctx, agentId, userId, model, input, output)
}
