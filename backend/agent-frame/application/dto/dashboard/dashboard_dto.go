package dashboard

// DashboardOverviewReq 获取概览请求
type DashboardOverviewReq struct {
}

// DashboardOverviewRsp 获取概览响应
type DashboardOverviewRsp struct {
	ActiveAgents           int `json:"active_agents"`
	PeriodicAgents         int `json:"periodic_agents"`
	TasksCompleted         int `json:"tasks_completed"`
	TotalTokens            int `json:"total_tokens"`
	ActiveKnowledgeSources int `json:"active_knowledge_sources"`
}

// TokenUsageRankingReq Token排行请求
type TokenUsageRankingReq struct {
	Limit int `form:"limit" json:"limit"`
}

// TokenUsageRankingRsp Token排行响应
type TokenUsageRankingRsp struct {
	Rankings []TokenUsageItem `json:"rankings"`
}

// TokenUsageItem Token使用项
type TokenUsageItem struct {
	AgentId     string `json:"agent_id"`
	AgentName   string `json:"agent_name"`
	TotalTokens int    `json:"total_tokens"`
}

// AgentUsageRankingReq 智能体使用排行请求
type AgentUsageRankingReq struct {
	Limit int `form:"limit" json:"limit"`
}

// AgentUsageRankingRsp 智能体使用排行响应
type AgentUsageRankingRsp struct {
	Rankings []AgentUsageItem `json:"rankings"`
}

// AgentUsageItem 智能体使用项
type AgentUsageItem struct {
	AgentId      string `json:"agent_id"`
	AgentName    string `json:"agent_name"`
	SessionCount int    `json:"session_count"`
	MessageCount int    `json:"message_count"`
}

// ChannelActivityReq 渠道活动请求
type ChannelActivityReq struct {
}

// ChannelActivityRsp 渠道活动响应
type ChannelActivityRsp struct {
	Channels []ChannelActivityItem `json:"channels"`
}

// ChannelActivityItem 渠道活动项
type ChannelActivityItem struct {
	ChannelId    string `json:"channel_id"`
	ChannelName  string `json:"channel_name"`
	Status       string `json:"status"`
	MessageCount int    `json:"message_count"`
}

// RecentSessionItem 最近会话项
type RecentSessionItem struct {
	Ulid      string `json:"ulid"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
	UserId    string `json:"user_id"`
	AgentId   string `json:"agent_id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Channel   string `json:"channel"`
	Model     string `json:"model"`
	CreatedBy string `json:"created_by"`
	UpdatedBy string `json:"updated_by"`
}

// GetRecentSessionsReq 获取最近会话请求
type GetRecentSessionsReq struct {
	Limit int `form:"limit" json:"limit"`
}

// GetRecentSessionsRsp 获取最近会话响应
type GetRecentSessionsRsp struct {
	Sessions []RecentSessionItem `json:"sessions"`
}
