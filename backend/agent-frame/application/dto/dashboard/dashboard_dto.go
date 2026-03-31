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
