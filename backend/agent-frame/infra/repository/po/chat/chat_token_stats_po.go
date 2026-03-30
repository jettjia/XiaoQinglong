package chat

import (
	"time"

	"github.com/jettjia/igo-pkg/pkg/util"
	"gorm.io/gorm"
)

type ChatTokenStats struct {
	Ulid         string `gorm:"column:ulid;primaryKey;type:varchar(128);comment:ulid;" json:"ulid"`
	SessionId    string `gorm:"column:session_id;type:varchar(128);comment:会话ID;" json:"session_id"`
	AgentId      string `gorm:"column:agent_id;type:varchar(128);comment:智能体ID;" json:"agent_id"`
	UserId       string `gorm:"column:user_id;type:varchar(128);comment:用户ID;" json:"user_id"`
	Date         string `gorm:"column:date;type:varchar(16);comment:统计日期:YYYY-MM-DD;index:idx_stats;" json:"date"`
	Model        string `gorm:"column:model;type:varchar(128);comment:模型标识;" json:"model"`
	InputTokens  int    `gorm:"column:input_tokens;default:0;type:int;comment:当日输入Token累计;" json:"input_tokens"`
	OutputTokens int    `gorm:"column:output_tokens;default:0;type:int;comment:当日输出Token累计;" json:"output_tokens"`
	TotalTokens  int    `gorm:"column:total_tokens;default:0;type:int;comment:当日总Token累计;" json:"total_tokens"`
	RequestCount int    `gorm:"column:request_count;default:0;type:int;comment:当日请求次数;" json:"request_count"`
	CostAmount   float64 `gorm:"column:cost_amount;type:decimal(10,4);default:0;comment:预估费用;" json:"cost_amount"`
	CreatedAt   int64   `gorm:"column:created_at;autoCreateTime:milli;type:bigint;comment:创建时间;" json:"created_at"`
	UpdatedAt   int64   `gorm:"column:updated_at;autoUpdateTime:milli;type:bigint;comment:修改时间;" json:"updated_at"`
}

func (po *ChatTokenStats) BeforeCreate(tx *gorm.DB) error {
	po.Ulid = util.Ulid()
	return nil
}

func (po *ChatTokenStats) TableName() string {
	return "chat_token_stats"
}

// AddTokens 添加token统计
func (po *ChatTokenStats) AddTokens(input, output int) {
	po.InputTokens += input
	po.OutputTokens += output
	po.TotalTokens += input + output
	po.RequestCount++
	po.UpdatedAt = time.Now().UnixMilli()
}

// GetDateKey 获取日期索引键
func GetDateKey() string {
	return time.Now().Format("2006-01-02")
}