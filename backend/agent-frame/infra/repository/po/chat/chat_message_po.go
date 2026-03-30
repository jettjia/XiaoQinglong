package chat

import (
	"database/sql/driver"

	"github.com/jettjia/igo-pkg/pkg/util"
	"gorm.io/gorm"
)

type ChatMessage struct {
	Ulid         string `gorm:"column:ulid;primaryKey;type:varchar(128);comment:ulid;" json:"ulid"`
	SessionId    string `gorm:"column:session_id;type:varchar(128);comment:会话ID;index:idx_session;" json:"session_id"`
	CreatedAt    int64  `gorm:"column:created_at;autoCreateTime:milli;type:bigint;comment:创建时间;" json:"created_at"`
	Role         string `gorm:"column:role;type:varchar(32);comment:角色:user/assistant/system;" json:"role"`
	Content      string `gorm:"column:content;type:text;comment:消息内容;" json:"content"`
	Model        string `gorm:"column:model;type:varchar(128);comment:使用的模型;" json:"model"`
	InputTokens  int    `gorm:"column:input_tokens;default:0;type:int;comment:输入Token数;" json:"input_tokens"`
	OutputTokens int    `gorm:"column:output_tokens;default:0;type:int;comment:输出Token数;" json:"output_tokens"`
	TotalTokens  int    `gorm:"column:total_tokens;default:0;type:int;comment:总Token数;" json:"total_tokens"`
	LatencyMs    int    `gorm:"column:latency_ms;type:int;comment:响应延迟;" json:"latency_ms"`
	Trace        StringJSON `gorm:"column:trace;type:json;comment:执行轨迹;" json:"trace"`
	Status       string `gorm:"column:status;type:varchar(32);default:sending;comment:状态:sending/success/failed/pending_approval;" json:"status"`
	ErrorMsg     string `gorm:"column:error_msg;type:text;comment:错误信息;" json:"error_msg"`
	Metadata     StringJSON `gorm:"column:metadata;type:json;comment:附加元数据;" json:"metadata"`
}

// StringJSON is a custom type for handling JSON string columns that can be null
type StringJSON struct {
Val string
}

// Scan implements the Scanner interface
func (j *StringJSON) Scan(value interface{}) error {
	if value == nil {
		j.Val = ""
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	j.Val = string(bytes)
	return nil
}

// Value implements the driver.Valuer interface
func (j StringJSON) Value() (driver.Value, error) {
	if j.Val == "" {
		return nil, nil
	}
	return j.Val, nil
}

// UnmarshalJSON implements json.Unmarshaler
func (j *StringJSON) UnmarshalJSON(data []byte) error {
	j.Val = string(data)
	return nil
}

// MarshalJSON implements json.Marshaler
func (j StringJSON) MarshalJSON() ([]byte, error) {
	if j.Val == "" {
		return []byte("null"), nil
	}
	return []byte(j.Val), nil
}

func (po *ChatMessage) BeforeCreate(tx *gorm.DB) error {
	po.Ulid = util.Ulid()
	return nil
}

func (po *ChatMessage) TableName() string {
	return "chat_message"
}

// IsPendingApproval 是否等待审批
func (po *ChatMessage) IsPendingApproval() bool {
	return po.Status == "pending_approval"
}