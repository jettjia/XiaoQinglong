package chat

import (
	"context"
	"sort"
	"time"

	"github.com/jettjia/igo-pkg/pkg/data"
	"github.com/jettjia/igo-pkg/pkg/util"
	"github.com/jettjia/igo-pkg/pkg/xsql/builder"
	"gorm.io/gorm"

	entity "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/chat"
	irepository "github.com/jettjia/xiaoqinglong/agent-frame/domain/irepository/chat"
	"github.com/jettjia/xiaoqinglong/agent-frame/infra/pkg/idata"
	converter "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/converter/chat"
	po "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/chat"
	poJob "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/job"
)

var _ irepository.IChatSessionRepo = (*ChatSession)(nil)

type ChatSession struct {
	data *data.Data
}

func NewChatSessionImpl() *ChatSession {
	return &ChatSession{data: idata.NewDataOptionCli()}
}

func (r *ChatSession) Create(ctx context.Context, session *entity.ChatSession) (ulid string, err error) {
	chatPo := converter.E2PChatSessionAdd(session)
	if err = r.data.DB(ctx).Create(&chatPo).Error; err != nil {
		return
	}
	return chatPo.Ulid, nil
}

func (r *ChatSession) Delete(ctx context.Context, ulid string) error {
	return r.data.DB(ctx).Unscoped().Where("ulid = ?", ulid).Delete(&po.ChatSession{}).Error
}

func (r *ChatSession) Update(ctx context.Context, session *entity.ChatSession) error {
	chatPo := converter.E2PChatSessionUpdate(session)
	return r.data.DB(ctx).Model(&po.ChatSession{}).Where("ulid = ?", session.Ulid).Updates(chatPo).Error
}

func (r *ChatSession) UpdateStatus(ctx context.Context, ulid string, status string) error {
	return r.data.DB(ctx).Model(&po.ChatSession{}).Where("ulid = ?", ulid).Updates(map[string]interface{}{
		"status":     status,
		"updated_at": time.Now().UnixMilli(),
	}).Error
}

func (r *ChatSession) FindById(ctx context.Context, ulid string) (*entity.ChatSession, error) {
	var chatPo po.ChatSession
	if err := r.data.DB(ctx).Where("ulid = ? AND deleted_at = 0", ulid).First(&chatPo).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return converter.P2EChatSession(&chatPo), nil
}

func (r *ChatSession) CreateWithId(ctx context.Context, session *entity.ChatSession, ulid string) error {
	chatPo := converter.E2PChatSessionAdd(session)
	chatPo.Ulid = ulid
	return r.data.DB(ctx).Create(&chatPo).Error
}

func (r *ChatSession) FindByUserIdAndChannel(ctx context.Context, userId, channel string) (*entity.ChatSession, error) {
	var chatPo po.ChatSession
	if err := r.data.DB(ctx).Where("user_id = ? AND channel = ? AND deleted_at = 0", userId, channel).Order("updated_at DESC").First(&chatPo).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return converter.P2EChatSession(&chatPo), nil
}

func (r *ChatSession) FindByUserId(ctx context.Context, userId string, status string) ([]*entity.ChatSession, error) {
	var chatPos []*po.ChatSession
	query := r.data.DB(ctx).Where("user_id = ? AND deleted_at = 0", userId)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if err := query.Order("updated_at DESC").Find(&chatPos).Error; err != nil {
		return nil, err
	}
	return converter.P2EChatSessions(chatPos), nil
}

func (r *ChatSession) FindPage(ctx context.Context, queries []*builder.Query, reqPage *builder.PageData, reqSort *builder.SortData) ([]*entity.ChatSession, *builder.PageData, error) {
	var total int64
	chatPos := make([]*po.ChatSession, 0)

	whereStr, values, err := builder.GormBuildWhere(queries)
	if err != nil {
		return nil, nil, err
	}

	if reqSort == nil {
		reqSort = &builder.SortData{Sort: "ulid", Direction: "desc"}
	}
	if reqPage == nil {
		reqPage = &builder.PageData{PageNum: 1, PageSize: 10}
	}

	dbQuery := r.data.DB(ctx).Model(&po.ChatSession{}).Where(whereStr, values...)

	if err = dbQuery.Count(&total).Error; err != nil {
		return nil, nil, err
	}

	rspPag := &builder.PageData{
		PageNum:     reqPage.PageNum,
		PageSize:    reqPage.PageSize,
		TotalNumber: total,
		TotalPage:   builder.CeilPageNum(total, reqPage.PageSize),
	}

	if total == 0 {
		return []*entity.ChatSession{}, rspPag, nil
	}

	err = dbQuery.
		Order(reqSort.Sort + " " + reqSort.Direction).
		Scopes(builder.GormPaginate(reqPage.PageNum, reqPage.PageSize)).
		Find(&chatPos).Error

	if err != nil {
		return nil, nil, err
	}

	return converter.P2EChatSessions(chatPos), rspPag, nil
}

func (r *ChatSession) FindRecent(ctx context.Context, limit int) ([]*entity.ChatSession, error) {
	var chatPos []*po.ChatSession
	if limit <= 0 {
		limit = 10
	}
	if err := r.data.DB(ctx).
		Where("deleted_at = 0").
		Order("updated_at DESC").
		Limit(limit).
		Find(&chatPos).Error; err != nil {
		return nil, err
	}
	return converter.P2EChatSessions(chatPos), nil
}

func (r *ChatSession) CountByChannel(ctx context.Context) (map[string]int, error) {
	var results []struct {
		Channel      string
		MessageCount int
	}
	if err := r.data.DB(ctx).Model(&po.ChatSession{}).
		Select("channel, COUNT(*) as message_count").
		Where("deleted_at = 0").
		Group("channel").
		Scan(&results).Error; err != nil {
		return nil, err
	}
	countMap := make(map[string]int)
	for _, res := range results {
		countMap[res.Channel] = res.MessageCount
	}
	return countMap, nil
}

// ====== ChatMessage ======

var _ irepository.IChatMessageRepo = (*ChatMessage)(nil)

type ChatMessage struct {
	data *data.Data
}

func NewChatMessageImpl() *ChatMessage {
	return &ChatMessage{data: idata.NewDataOptionCli()}
}

func (r *ChatMessage) Create(ctx context.Context, message *entity.ChatMessage) (ulid string, err error) {
	ulid = util.Ulid()

	// Build map for insert to handle JSON fields properly
	values := map[string]interface{}{
		"ulid":          ulid,
		"session_id":    message.SessionId,
		"role":          message.Role,
		"content":       message.Content,
		"model":         message.Model,
		"input_tokens":  message.InputTokens,
		"output_tokens": message.OutputTokens,
		"total_tokens":  message.TotalTokens,
		"latency_ms":    message.LatencyMs,
		"status":        message.Status,
		"error_msg":     message.ErrorMsg,
		"created_at":    time.Now().UnixMilli(),
	}

	// Handle JSON fields - empty strings become NULL for PostgreSQL JSON type
	if message.Trace != "" {
		values["trace"] = message.Trace
	} else {
		values["trace"] = nil
	}
	if message.Metadata != "" {
		values["metadata"] = message.Metadata
	} else {
		values["metadata"] = nil
	}

	if err = r.data.DB(ctx).Table("chat_message").Create(values).Error; err != nil {
		return
	}
	return ulid, nil
}

func (r *ChatMessage) Update(ctx context.Context, message *entity.ChatMessage) error {
	msgPo := converter.E2PChatMessageUpdate(message)
	return r.data.DB(ctx).Model(&po.ChatMessage{}).Where("ulid = ?", message.Ulid).Updates(msgPo).Error
}

func (r *ChatMessage) UpdateStatus(ctx context.Context, ulid string, status string) error {
	return r.data.DB(ctx).Model(&po.ChatMessage{}).Where("ulid = ?", ulid).Update("status", status).Error
}

func (r *ChatMessage) FindById(ctx context.Context, ulid string) (*entity.ChatMessage, error) {
	var msgPo po.ChatMessage
	if err := r.data.DB(ctx).Where("ulid = ?", ulid).First(&msgPo).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return converter.P2EChatMessage(&msgPo), nil
}

func (r *ChatMessage) FindBySessionId(ctx context.Context, sessionId string) ([]*entity.ChatMessage, error) {
	var msgPos []*po.ChatMessage
	if err := r.data.DB(ctx).Where("session_id = ?", sessionId).Order("created_at ASC").Find(&msgPos).Error; err != nil {
		return nil, err
	}
	return converter.P2EChatMessages(msgPos), nil
}

// DeleteBySessionId 删除会话的所有消息（硬删除）
func (r *ChatMessage) DeleteBySessionId(ctx context.Context, sessionId string) error {
	return r.data.DB(ctx).Unscoped().Where("session_id = ?", sessionId).Delete(&po.ChatMessage{}).Error
}

// ====== ChatApproval ======

var _ irepository.IChatApprovalRepo = (*ChatApproval)(nil)

type ChatApproval struct {
	data *data.Data
}

func NewChatApprovalImpl() *ChatApproval {
	return &ChatApproval{data: idata.NewDataOptionCli()}
}

func (r *ChatApproval) Create(ctx context.Context, approval *entity.ChatApproval) (ulid string, err error) {
	approvalPo := converter.E2PChatApprovalAdd(approval)
	if err = r.data.DB(ctx).Create(&approvalPo).Error; err != nil {
		return
	}
	return approvalPo.Ulid, nil
}

func (r *ChatApproval) Update(ctx context.Context, approval *entity.ChatApproval) error {
	approvalPo := converter.E2PChatApprovalUpdate(approval)
	return r.data.DB(ctx).Model(&po.ChatApproval{}).Where("ulid = ?", approval.Ulid).Updates(approvalPo).Error
}

func (r *ChatApproval) UpdateStatus(ctx context.Context, ulid string, status, approvedBy string, reason string) error {
	updates := map[string]interface{}{
		"status":      status,
		"approved_by": approvedBy,
		"approved_at": time.Now().UnixMilli(),
	}
	if reason != "" {
		updates["reason"] = reason
	}
	return r.data.DB(ctx).Model(&po.ChatApproval{}).Where("ulid = ?", ulid).Updates(updates).Error
}

func (r *ChatApproval) FindById(ctx context.Context, ulid string) (*entity.ChatApproval, error) {
	var approvalPo po.ChatApproval
	if err := r.data.DB(ctx).Where("ulid = ?", ulid).First(&approvalPo).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return converter.P2EChatApproval(&approvalPo), nil
}

func (r *ChatApproval) FindByMessageId(ctx context.Context, messageId string) (*entity.ChatApproval, error) {
	var approvalPo po.ChatApproval
	if err := r.data.DB(ctx).Where("message_id = ?", messageId).First(&approvalPo).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return converter.P2EChatApproval(&approvalPo), nil
}

func (r *ChatApproval) FindPending(ctx context.Context) ([]*entity.ChatApproval, error) {
	var approvalPos []*po.ChatApproval
	if err := r.data.DB(ctx).Where("status = ?", "pending").Order("created_at DESC").Find(&approvalPos).Error; err != nil {
		return nil, err
	}
	return converter.P2EChatApprovals(approvalPos), nil
}

func (r *ChatApproval) FindByUserId(ctx context.Context, userId string) ([]*entity.ChatApproval, error) {
	var approvalPos []*po.ChatApproval
	if err := r.data.DB(ctx).Where("approved_by = ?", userId).Order("created_at DESC").Find(&approvalPos).Error; err != nil {
		return nil, err
	}
	return converter.P2EChatApprovals(approvalPos), nil
}

// ====== ChatTokenStats ======

var _ irepository.IChatTokenStatsRepo = (*ChatTokenStats)(nil)

type ChatTokenStats struct {
	data *data.Data
}

func NewChatTokenStatsImpl() *ChatTokenStats {
	return &ChatTokenStats{data: idata.NewDataOptionCli()}
}

func (r *ChatTokenStats) Create(ctx context.Context, stats *entity.ChatTokenStats) (ulid string, err error) {
	statsPo := converter.E2PChatTokenStatsAdd(stats)
	if err = r.data.DB(ctx).Create(&statsPo).Error; err != nil {
		return
	}
	return statsPo.Ulid, nil
}

func (r *ChatTokenStats) Update(ctx context.Context, stats *entity.ChatTokenStats) error {
	statsPo := converter.E2PChatTokenStatsUpdate(stats)
	return r.data.DB(ctx).Model(&po.ChatTokenStats{}).Where("ulid = ?", stats.Ulid).Updates(statsPo).Error
}

func (r *ChatTokenStats) FindOrCreate(ctx context.Context, agentId, userId, date, model string) (*entity.ChatTokenStats, error) {
	var statsPo po.ChatTokenStats
	err := r.data.DB(ctx).Where("agent_id = ? AND user_id = ? AND date = ? AND model = ?", agentId, userId, date, model).First(&statsPo).Error
	if err == nil {
		return converter.P2EChatTokenStats(&statsPo), nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	// Create new
	newStats := &entity.ChatTokenStats{
		AgentId: agentId,
		UserId:  userId,
		Date:    date,
		Model:   model,
	}
	newStatsPo := converter.E2PChatTokenStatsAdd(newStats)
	if err = r.data.DB(ctx).Create(newStatsPo).Error; err != nil {
		return nil, err
	}
	return converter.P2EChatTokenStats(newStatsPo), nil
}

func (r *ChatTokenStats) AddTokens(ctx context.Context, agentId, userId, date, model string, input, output int) error {
	stats, err := r.FindOrCreate(ctx, agentId, userId, date, model)
	if err != nil {
		return err
	}

	stats.InputTokens += input
	stats.OutputTokens += output
	stats.TotalTokens += input + output
	stats.RequestCount++

	statsPo := converter.E2PChatTokenStatsUpdate(stats)
	return r.data.DB(ctx).Model(&po.ChatTokenStats{}).Where("ulid = ?", stats.Ulid).Updates(statsPo).Error
}

func (r *ChatTokenStats) GetTotalTokens(ctx context.Context) (int, error) {
	var result struct {
		Total int
	}

	// 查询 chat_token_stats 表的总 token
	if err := r.data.DB(ctx).Model(&po.ChatTokenStats{}).Select("COALESCE(SUM(total_tokens), 0) as total").Scan(&result).Error; err != nil {
		return 0, err
	}
	chatTokens := result.Total

	// 查询 job_execution_log 表成功任务的 token 消耗
	var jobTokens int
	if err := r.data.DB(ctx).Model(&poJob.JobExecutionPO{}).
		Select("COALESCE(SUM(tokens_used), 0) as total").
		Where("status = 'success' AND deleted_at = 0").
		Scan(&jobTokens).Error; err != nil {
		// job 表查不到不报错，只是 chat token
		jobTokens = 0
	}

	return chatTokens + jobTokens, nil
}

func (r *ChatTokenStats) GetTokenRanking(ctx context.Context, limit int) ([]*irepository.TokenRankingItem, error) {
	if limit <= 0 {
		limit = 10
	}

	// Token from chat sessions
	var chatResults []struct {
		AgentId     string
		TotalTokens int
	}
	err := r.data.DB(ctx).Model(&po.ChatTokenStats{}).
		Select("agent_id, SUM(total_tokens) as total_tokens").
		Group("agent_id").
		Scan(&chatResults)
	if err != nil {
		return nil, err.Error
	}

	// Token from job executions
	var jobResults []struct {
		AgentId     string
		TotalTokens int
	}
	err = r.data.DB(ctx).Model(&poJob.JobExecutionPO{}).
		Select("agent_id, COALESCE(SUM(tokens_used), 0) as total_tokens").
		Where("status = 'success' AND deleted_at = 0").
		Group("agent_id").
		Scan(&jobResults)
	if err != nil {
		jobResults = nil // fallback if error
	}

	// Merge by agent_id
	tokenMap := make(map[string]int)
	for _, res := range chatResults {
		tokenMap[res.AgentId] += res.TotalTokens
	}
	for _, res := range jobResults {
		tokenMap[res.AgentId] += res.TotalTokens
	}

	// Sort by total_tokens desc
	type rankedItem struct {
		agentId     string
		totalTokens int
	}
	ranked := make([]rankedItem, 0, len(tokenMap))
	for agentId, tokens := range tokenMap {
		ranked = append(ranked, rankedItem{agentId, tokens})
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].totalTokens > ranked[j].totalTokens
	})

	// Apply limit and get agent names
	items := make([]*irepository.TokenRankingItem, 0, limit)
	for i := 0; i < len(ranked) && i < limit; i++ {
		agentName := ranked[i].agentId
		var agentNameFromDB string
		_ = r.data.DB(ctx).Table("sys_agent").Where("ulid = ?", ranked[i].agentId).Pluck("name", &agentNameFromDB).Error
		if agentNameFromDB != "" {
			agentName = agentNameFromDB
		}
		items = append(items, &irepository.TokenRankingItem{
			AgentId:     ranked[i].agentId,
			AgentName:   agentName,
			TotalTokens: ranked[i].totalTokens,
		})
	}

	return items, nil
}
