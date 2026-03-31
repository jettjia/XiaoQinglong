package chat

import (
	"context"
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
	return r.data.DB(ctx).Model(&po.ChatSession{}).Where("ulid = ?", ulid).Updates(map[string]interface{}{
		"deleted_at": time.Now().UnixMilli(),
	}).Error
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
	if err := r.data.DB(ctx).Model(&po.ChatTokenStats{}).Select("COALESCE(SUM(total_tokens), 0) as total").Scan(&result).Error; err != nil {
		return 0, err
	}
	return result.Total, nil
}

func (r *ChatTokenStats) GetTokenRanking(ctx context.Context, limit int) ([]*irepository.TokenRankingItem, error) {
	var results []*struct {
		AgentId     string
		AgentName   string
		TotalTokens int
	}

	if limit <= 0 {
		limit = 10
	}

	err := r.data.DB(ctx).Model(&po.ChatTokenStats{}).
		Select("agent_id, SUM(total_tokens) as total_tokens").
		Group("agent_id").
		Order("total_tokens DESC").
		Limit(limit).
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	// Get agent names
	items := make([]*irepository.TokenRankingItem, 0, len(results))
	for _, res := range results {
		agentName := res.AgentId // fallback to ID if not found
		// Try to get agent name from sys_agent table if available
		var agentNameFromDB string
		err := r.data.DB(ctx).Table("sys_agent").Where("ulid = ?", res.AgentId).Pluck("name", &agentNameFromDB).Error
		if err == nil && agentNameFromDB != "" {
			agentName = agentNameFromDB
		}
		items = append(items, &irepository.TokenRankingItem{
			AgentId:     res.AgentId,
			AgentName:   agentName,
			TotalTokens: res.TotalTokens,
		})
	}

	return items, nil
}