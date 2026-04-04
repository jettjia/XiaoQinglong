package dashboard

import (
	"context"

	dto "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/dashboard"
	agentSrv "github.com/jettjia/xiaoqinglong/agent-frame/domain/srv/agent"
	channelSrv "github.com/jettjia/xiaoqinglong/agent-frame/domain/srv/channel"
	chatSrv "github.com/jettjia/xiaoqinglong/agent-frame/domain/srv/chat"
	jobSrv "github.com/jettjia/xiaoqinglong/agent-frame/domain/srv/job"
	knowledgeBaseSrv "github.com/jettjia/xiaoqinglong/agent-frame/domain/srv/knowledge_base"
	"github.com/jettjia/igo-pkg/pkg/xsql/builder"
)

type DashboardSvc struct {
	agentSvc          *agentSrv.SysAgentSvc
	kbSvc             *knowledgeBaseSrv.SysKnowledgeBaseSvc
	channelSvc        *channelSrv.SysChannelSvc
	chatTokenStatsSvc *chatSrv.ChatTokenStatsSvc
	chatSessionSvc    *chatSrv.ChatSessionSvc
	jobSvc            *jobSrv.JobExecution
}

func NewDashboardSvc() *DashboardSvc {
	return &DashboardSvc{
		agentSvc:          agentSrv.NewSysAgentSvc(),
		kbSvc:             knowledgeBaseSrv.NewSysKnowledgeBaseSvc(),
		channelSvc:        channelSrv.NewSysChannelSvc(),
		chatTokenStatsSvc: chatSrv.NewChatTokenStatsSvc(),
		chatSessionSvc:    chatSrv.NewChatSessionSvc(),
		jobSvc:            jobSrv.NewJobExecutionSvc(),
	}
}

func (s *DashboardSvc) GetOverview(ctx context.Context, req *dto.DashboardOverviewReq) (*dto.DashboardOverviewRsp, error) {
	// Get active agents count (only non-deleted)
	agentQueries := []*builder.Query{
		{Key: "deleted_at", Operator: builder.Operator_opEq, Value: 0},
	}
	agents, err := s.agentSvc.FindAll(ctx, agentQueries)
	activeAgents := 0
	periodicAgents := 0
	if err == nil && agents != nil {
		for _, a := range agents {
			if a.Enabled {
				activeAgents++
			}
			if a.IsPeriodic || a.CronRule != "" {
				periodicAgents++
			}
		}
	}

	// Get active knowledge sources count (only non-deleted)
	kbQueries := []*builder.Query{
		{Key: "deleted_at", Operator: builder.Operator_opEq, Value: 0},
	}
	kbList, err := s.kbSvc.FindAll(ctx, kbQueries)
	activeKnowledgeSources := 0
	if err == nil && kbList != nil {
		for _, kb := range kbList {
			if kb.Enabled {
				activeKnowledgeSources++
			}
		}
	}

	// Get total tokens
	totalTokens, err := s.chatTokenStatsSvc.GetTotalTokens(ctx)
	if err != nil {
		totalTokens = 0
	}

	// Tasks completed - count job executions with status = 'success'
	tasksCompleted, err := s.jobSvc.CountByStatus(ctx, "success")
	if err != nil {
		tasksCompleted = 0
	}

	return &dto.DashboardOverviewRsp{
		ActiveAgents:           activeAgents,
		PeriodicAgents:         periodicAgents,
		TasksCompleted:         tasksCompleted,
		TotalTokens:            totalTokens,
		ActiveKnowledgeSources: activeKnowledgeSources,
	}, nil
}

func (s *DashboardSvc) GetTokenUsageRanking(ctx context.Context, req *dto.TokenUsageRankingReq) (*dto.TokenUsageRankingRsp, error) {
	items, err := s.chatTokenStatsSvc.GetTokenRanking(ctx, req.Limit)
	if err != nil {
		return nil, err
	}

	rankings := make([]dto.TokenUsageItem, 0, len(items))
	for _, item := range items {
		rankings = append(rankings, dto.TokenUsageItem{
			AgentId:     item.AgentId,
			AgentName:   item.AgentName,
			TotalTokens: item.TotalTokens,
		})
	}

	return &dto.TokenUsageRankingRsp{Rankings: rankings}, nil
}

func (s *DashboardSvc) GetChannelActivity(ctx context.Context, req *dto.ChannelActivityReq) (*dto.ChannelActivityRsp, error) {
	channels, err := s.channelSvc.FindAll(ctx, nil)
	if err != nil {
		return nil, err
	}

	items := make([]dto.ChannelActivityItem, 0, len(channels))
	for _, ch := range channels {
		status := "inactive"
		if ch.Enabled {
			status = "active"
		}
		items = append(items, dto.ChannelActivityItem{
			ChannelId:    ch.Ulid,
			ChannelName:  ch.Name,
			Status:       status,
			MessageCount: 0, // TODO: Need to join with chat_session to count messages
		})
	}

	return &dto.ChannelActivityRsp{Channels: items}, nil
}

func (s *DashboardSvc) GetRecentSessions(ctx context.Context, req *dto.GetRecentSessionsReq) (*dto.GetRecentSessionsRsp, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	sessions, err := s.chatSessionSvc.FindRecentSessions(ctx, limit)
	if err != nil {
		return nil, err
	}

	items := make([]dto.RecentSessionItem, 0, len(sessions))
	for _, sess := range sessions {
		items = append(items, dto.RecentSessionItem{
			Ulid:      sess.Ulid,
			CreatedAt: sess.CreatedAt,
			UpdatedAt: sess.UpdatedAt,
			UserId:    sess.UserId,
			AgentId:   sess.AgentId,
			Title:     sess.Title,
			Status:    sess.Status,
			Channel:   sess.Channel,
			Model:     "",
			CreatedBy: "",
			UpdatedBy: "",
		})
	}

	return &dto.GetRecentSessionsRsp{Sessions: items}, nil
}
